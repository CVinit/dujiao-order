package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ---------------------------------------------------------------------------
// IPRateLimiter — per-IP token bucket rate limiter with periodic cleanup.
// ---------------------------------------------------------------------------

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter provides per-IP rate limiting using token buckets.
type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
	stopCh   chan struct{}
}

// NewIPRateLimiter creates a new rate limiter with the given requests-per-second
// and burst size. It starts a background goroutine to clean up stale entries.
func NewIPRateLimiter(rps float64, burst int) *IPRateLimiter {
	rl := &IPRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Limit(rps),
		burst:    burst,
		stopCh:   make(chan struct{}),
	}
	go rl.cleanup(3 * time.Minute)
	return rl
}

func (rl *IPRateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{limiter: rate.NewLimiter(rl.rate, rl.burst)}
		rl.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (rl *IPRateLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastSeen) > interval {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Middleware returns an HTTP middleware that returns 429 if the client IP
// exceeds the configured rate limit.
func (rl *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := safeIP(r)

		limiter := rl.getVisitor(ip)
		if !limiter.Allow() {
			http.Error(w, "请求过于频繁，请稍后再试", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// TurnstileVerifier — verifies Cloudflare Turnstile tokens server-side.
// ---------------------------------------------------------------------------

const turnstileSiteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// turnstileResponse represents the JSON response from the Turnstile siteverify API.
type turnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// TurnstileVerifier verifies Cloudflare Turnstile tokens via the siteverify API.
type TurnstileVerifier struct {
	secret     string
	httpClient *http.Client
}

// NewTurnstileVerifier creates a new verifier. If secretKey is empty, Verify
// always returns true (for local development without Turnstile).
func NewTurnstileVerifier(secretKey string) *TurnstileVerifier {
	return &TurnstileVerifier{
		secret: secretKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Verify sends a server-side verification request for the given token.
// Returns true if verification succeeds. If the secret key is empty, it
// returns true without making a request (for local development).
func (v *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if v.secret == "" {
		return true, nil
	}

	data := url.Values{}
	data.Set("secret", v.secret)
	data.Set("response", token)
	if remoteIP != "" {
		data.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileSiteverifyURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, fmt.Errorf("create turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("turnstile request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("read turnstile response: %w", err)
	}

	var result turnstileResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("parse turnstile response: %w", err)
	}

	return result.Success, nil
}

// ---------------------------------------------------------------------------
// BanTracker — tracks failed query attempts per IP and auto-bans after threshold.
// ---------------------------------------------------------------------------

type banEntry struct {
	failCount int
	failTime  time.Time
	bannedAt  time.Time
}

// BanTracker tracks failed attempts per IP and auto-bans IPs that exceed the
// threshold. Bans automatically expire after the configured duration. A
// background goroutine cleans up stale entries periodically.
type BanTracker struct {
	mu        sync.Mutex
	entries   map[string]*banEntry
	threshold int
	duration  time.Duration
	stopCh    chan struct{}
}

// NewBanTracker creates a new tracker with the given failure threshold and ban
// duration. After an IP records `threshold` consecutive failures it is banned
// for `duration`. A successful query resets the failure counter.
// A background goroutine is started to clean up stale entries.
func NewBanTracker(threshold int, duration time.Duration) *BanTracker {
	bt := &BanTracker{
		entries:   make(map[string]*banEntry),
		threshold: threshold,
		duration:  duration,
		stopCh:    make(chan struct{}),
	}
	go bt.cleanup(3 * time.Minute)
	return bt
}

func (bt *BanTracker) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			bt.mu.Lock()
			for ip, e := range bt.entries {
				if !e.bannedAt.IsZero() && time.Since(e.bannedAt) > bt.duration {
					// Ban expired; remove entry.
					delete(bt.entries, ip)
				} else if e.bannedAt.IsZero() && time.Since(e.failTime) > interval {
					// Stale entry with failures below threshold; remove.
					delete(bt.entries, ip)
				}
			}
			bt.mu.Unlock()
		case <-bt.stopCh:
			return
		}
	}
}

// RecordFailure increments the failure counter for the given IP. It returns
// true if the IP is now banned (i.e. this failure pushed the count over the
// threshold).
func (bt *BanTracker) RecordFailure(ip string) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	e, exists := bt.entries[ip]
	if !exists {
		e = &banEntry{}
		bt.entries[ip] = e
	}

	// If already banned, just extend the ban.
	if !e.bannedAt.IsZero() {
		return false
	}

	e.failCount++
	e.failTime = time.Now()
	if e.failCount >= bt.threshold {
		e.bannedAt = time.Now()
		return true
	}
	return false
}

// IsBanned returns true if the given IP is currently banned (and the ban has
// not yet expired).
func (bt *BanTracker) IsBanned(ip string) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	e, exists := bt.entries[ip]
	if !exists {
		return false
	}

	if e.bannedAt.IsZero() {
		return false
	}

	if time.Since(e.bannedAt) > bt.duration {
		// Ban has expired; clean up the entry.
		delete(bt.entries, ip)
		return false
	}
	return true
}

// RecordSuccess resets the failure counter for the given IP on a successful query.
func (bt *BanTracker) RecordSuccess(ip string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	delete(bt.entries, ip)
}

// Middleware returns an HTTP middleware that returns 403 if the client IP is
// currently banned.
func (bt *BanTracker) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := safeIP(r)

		if bt.IsBanned(ip) {
			http.Error(w, "您的IP已被临时封禁，请稍后再试", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

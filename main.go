package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := loadConfig()

	sql.Register("pgx", stdlib.GetDefaultDriver())

	db, err := Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	turnstileVerifier := NewTurnstileVerifier(cfg.TurnstileSecretKey)
	banTracker := NewBanTracker(cfg.BanThreshold, cfg.BanDuration)
	rateLimiter := NewIPRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", QueryFormHandler(cfg))
	mux.HandleFunc("POST /query", QueryHandler(cfg, db, turnstileVerifier, banTracker))

	var handler http.Handler = mux
	handler = rateLimiter.Middleware(handler)
	handler = banTracker.Middleware(handler)
	handler = loggingMiddleware(handler)

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down server...")
		server.Close()
	}()

	log.Printf("Starting server on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func loadConfig() Config {
	rateLimitRPS := 1.0
	if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			rateLimitRPS = f
		}
	}

	rateLimitBurst := 3
	if v := os.Getenv("RATE_LIMIT_BURST"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			rateLimitBurst = i
		}
	}

	banThreshold := 10
	if v := os.Getenv("BAN_THRESHOLD"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			banThreshold = i
		}
	}

	banDuration := 15 * time.Minute
	if v := os.Getenv("BAN_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			banDuration = d
		}
	}

	listenAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		listenAddr = v
	}

	return Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		ListenAddr:        listenAddr,
		TurnstileSiteKey:  os.Getenv("TURNSTILE_SITE_KEY"),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		RateLimitRPS:      rateLimitRPS,
		RateLimitBurst:    rateLimitBurst,
		BanThreshold:      banThreshold,
		BanDuration:       banDuration,
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

// safeIP extracts the client IP, preferring X-Real-IP set by a trusted
// reverse proxy, falling back to RemoteAddr.
func safeIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	ip, _, err := splitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// fmtTime formats a time.Time for display in the template.
func fmtTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// fmtPrice formats a float64 price value.
func fmtPrice(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

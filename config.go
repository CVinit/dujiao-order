package main

import (
	"net"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL        string
	ListenAddr         string
	TurnstileSiteKey   string
	TurnstileSecretKey string
	RateLimitRPS       float64
	RateLimitBurst     int
	BanThreshold       int
	BanDuration        time.Duration
}

// splitHostPort wraps net.SplitHostPort, returning the host portion of an
// address in the form "host:port". If the address is not in that form the
// original string is returned.
func splitHostPort(addr string) (string, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, "", err
	}
	return host, port, nil
}

package main

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu      sync.Mutex
	limiter map[string]*rate.Limiter
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiter: make(map[string]*rate.Limiter),
	}
}

func (r *RateLimiter) getRateLimiter(rate_key string, limit rate.Limit, burst int) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	limiter, ok := r.limiter[rate_key]
	if !ok {
		limiter = rate.NewLimiter(limit, burst)
		r.limiter[rate_key] = limiter
	}
	return limiter
}

func extractClientID(r *http.Request) string {
	auth := r.Header.Get("X-Api-Key")
	if auth != "" {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	ip := getClientIP(r)
	if ip != "" {
		return ip
	}

	return "unknown-client"
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	} else {
		parts := strings.Split(ip, ",")
		ip = strings.TrimSpace(parts[0])
	}
	return ip
}

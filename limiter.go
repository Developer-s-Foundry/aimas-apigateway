package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*clientLimiter
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{limiters: make(map[string]*clientLimiter)}
}

func (r *RateLimiter) getRateLimiter(clientID string, limit rate.Limit, burst int) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	cl, exists := r.limiters[clientID]
	if !exists {
		limiter := rate.NewLimiter(limit, burst)
		r.limiters[clientID] = &clientLimiter{limiter, time.Now()}
		return limiter
	}

	cl.lastSeen = time.Now()
	return cl.limiter
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

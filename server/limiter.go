package main

import (
	"log/slog"
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.RWMutex
	visitors map[string]*client
}

// cleanupVisitors removes visitors that haven't been seen in the last 2 minutes
func (rl *RateLimiter) cleanupVisitors() {
	for {
		slog.Info("Cleaning up visitors")
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 2*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if the visitor is allowed to make a request
func (rl *RateLimiter) Allow(address string) bool {
	rl.mu.Lock()
	ip, _, err := net.SplitHostPort(address)
	if err != nil {
		slog.Error("Error splitting address", "error", err.Error())
		return false
	}

	if _, found := rl.visitors[ip]; !found {
		rl.visitors[ip] = &client{
			limiter: rate.NewLimiter(rate.Every(time.Minute), AppConfig.RateLimiter.MaxRequestsPerMinute),
		}
	}
	rl.visitors[ip].lastSeen = time.Now()

	if !rl.visitors[ip].limiter.Allow() {
		rl.mu.Unlock()
		return false
	}
	rl.mu.Unlock()
	return true
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*client),
	}
}

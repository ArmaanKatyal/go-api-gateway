package main

import (
	"github.com/ArmaanKatyal/go_api_gateway/server/config"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu sync.RWMutex
	// just here if needed to record any metrics from the rate limiter
	Metrics  *PromMetrics
	visitors map[string]*client
}

// cleanupVisitors removes visitors that haven't been seen in the last 2 minutes
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		slog.Info("Cleaning up visitors")
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > time.Duration(config.AppConfig.RateLimiter.CleanupInterval)*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// isValidV4 checks if the address is a valid IPV4 address
func isValidV4(address string) bool {
	parts := strings.Split(address, ".")

	if len(parts) < 4 {
		return false
	}

	for _, x := range parts {
		if i, err := strconv.Atoi(x); err == nil {
			if i < 0 || i > 255 {
				return false
			}
		} else {
			return false
		}

	}
	return true
}

// Allow checks if the visitor is allowed to make a request
func (rl *RateLimiter) Allow(address string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	address = strings.Split(address, ":")[0]
	if !isValidV4(address) {
		slog.Error("invalid ip address")
		return false
	}

	v, found := rl.visitors[address]
	if !found {
		v = &client{
			limiter: rate.NewLimiter(rate.Every(time.Duration(config.AppConfig.RateLimiter.EventInterval)*time.Second),
				config.AppConfig.RateLimiter.MaxRequests),
		}
		rl.visitors[address] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

func NewRateLimiter(metrics *PromMetrics) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*client),
		Metrics:  metrics,
	}
}

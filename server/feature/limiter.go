package feature

import (
	"github.com/ArmaanKatyal/go_api_gateway/server/config"
	"golang.org/x/time/rate"
	"log/slog"
	"sync"
	"time"
)

type Visitor struct {
	Limiter  *rate.Limiter
	LastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*Visitor
	rate     rate.Limit
	burst    int
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		mu:       sync.Mutex{},
		visitors: make(map[string]*Visitor),
		rate:     rate.Every(time.Duration(config.AppConfig.RateLimiter.EventInterval) * time.Second),
		burst:    config.AppConfig.RateLimiter.MaxRequests,
	}
	return rl
}

func (rl *RateLimiter) CleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		slog.Info("Cleaning up visitors")
		for ip, v := range rl.visitors {
			if time.Since(v.LastSeen) > time.Duration(config.AppConfig.RateLimiter.CleanupInterval)*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) AddIP(ip string) *Visitor {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	c := &Visitor{
		Limiter:  rate.NewLimiter(rl.rate, rl.burst),
		LastSeen: time.Now(),
	}

	rl.visitors[ip] = c

	return c
}

func (rl *RateLimiter) GetVisitor(ip string) *Visitor {
	rl.mu.Lock()
	l, exists := rl.visitors[ip]
	if !exists {
		rl.mu.Unlock()
		return rl.AddIP(ip)
	}
	rl.mu.Unlock()
	return l
}

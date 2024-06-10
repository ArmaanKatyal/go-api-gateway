package main

import (
	"log/slog"
	"time"

	"github.com/sony/gobreaker/v2"
)

type CircuitBreaker struct {
	Breaker *gobreaker.CircuitBreaker[[]byte]
}

func NewCircuitBreaker(settings gobreaker.Settings) *CircuitBreaker {
	return &CircuitBreaker{
		Breaker: gobreaker.NewCircuitBreaker[[]byte](settings),
	}
}

func DefaultSettings(service string) gobreaker.Settings {
	return gobreaker.Settings{
		Name:     "cb-" + service,
		Timeout:  time.Duration(AppConfig.Server.CircuitBreaker.Timeout) * time.Second,
		Interval: time.Duration(AppConfig.Server.CircuitBreaker.Interval) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
	}
}

func (cb *CircuitBreaker) Execute(service string, f func() ([]byte, error)) ([]byte, error) {
	slog.Info("Forwarding request using circuit breaker", "service", service, "breaker", cb.Breaker.Name)
	return cb.Breaker.Execute(f)
}

func (cb *CircuitBreaker) IsOpen() bool {
	return cb.Breaker.State() == gobreaker.StateOpen
}

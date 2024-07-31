package main

import (
	"log/slog"
	"time"

	"github.com/sony/gobreaker/v2"
)

type CircuitBreaker struct {
	Settings CircuitSettings `json:"settings"`
	breaker  *gobreaker.CircuitBreaker[[]byte]
}

type CircuitSettings struct {
	Enabled      bool    `yaml:"enabled"`
	Timeout      uint    `yaml:"timeout"`
	Interval     uint    `yaml:"interval"`
	FailureRatio float64 `yaml:"failureRatio"`
}

func (cs *CircuitSettings) into(name string) gobreaker.Settings {
	return gobreaker.Settings{
		Name:     "cb-" + name,
		Timeout:  time.Duration(cs.Timeout) * time.Second,
		Interval: time.Duration(cs.Interval) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= cs.FailureRatio
		},
	}
}

func NewCircuitBreaker(svc_name string, settings CircuitSettings) *CircuitBreaker {
	return &CircuitBreaker{
		Settings: settings,
		breaker:  gobreaker.NewCircuitBreaker[[]byte](settings.into(svc_name)),
	}
}

func (cb *CircuitBreaker) Execute(service string, f func() ([]byte, error)) ([]byte, error) {
	slog.Info("Forwarding request using circuit breaker", "service", service, "breaker", cb.breaker.Name)
	return cb.breaker.Execute(f)
}

func (cb *CircuitBreaker) IsOpen() bool {
	return cb.breaker.State() == gobreaker.StateOpen
}

func (cb *CircuitBreaker) IsEnabled() bool {
	return cb.Settings.Enabled
}

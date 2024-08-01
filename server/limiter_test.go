package main

import (
	"github.com/ArmaanKatyal/go_api_gateway/server/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiterAllow(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid address",
			input:    "1.1.1.1:8080",
			expected: true,
		},
		{
			name:     "invalid address",
			input:    "1.1.1:8080",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.AppConfig.RateLimiter.MaxRequests = 60
			l := NewRateLimiter(nil)
			assert.Equal(t, tt.expected, l.Allow(tt.input))
		})
	}
	t.Run("Rate limiting", func(t *testing.T) {
		config.AppConfig.RateLimiter.MaxRequests = 1
		config.AppConfig.RateLimiter.EventInterval = 1
		l := NewRateLimiter(nil)
		assert.True(t, l.Allow("1.1.1.1:8080"))
		assert.False(t, l.Allow("1.1.1.1:8080"))
		time.Sleep(time.Duration(1) * time.Second)
		assert.True(t, l.Allow("1.1.1.1:8080"))
	})
}

func TestLimiterIsValidV4(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid address",
			input:    "1.1.1.1",
			expected: true,
		},
		{
			name:     "invalid address",
			input:    "1.1.1",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isValidV4(tt.input))
		})
	}
}

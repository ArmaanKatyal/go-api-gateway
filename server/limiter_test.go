package main

import (
	"testing"

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
			l := RateLimiter{
				visitors: make(map[string]*client),
			}
			assert.Equal(t, tt.expected, l.Allow(tt.input))
		})
	}
}

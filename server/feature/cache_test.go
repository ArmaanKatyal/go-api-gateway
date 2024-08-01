package feature

import (
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
)

type params struct {
	enabled            bool
	expirationInterval uint
	cleanupInterval    uint
}

func TestCacheHandler(t *testing.T) {
	tests := []struct {
		name     string
		given    params
		expected params
	}{
		{
			name:     "default values",
			given:    params{enabled: false, expirationInterval: 0, cleanupInterval: 0},
			expected: params{enabled: false, expirationInterval: 5, cleanupInterval: 10},
		},
		{
			name:     "custom values",
			given:    params{enabled: true, expirationInterval: 10, cleanupInterval: 20},
			expected: params{enabled: true, expirationInterval: 10, cleanupInterval: 20},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheHandler := NewCacheHandler(tt.given.enabled, tt.given.expirationInterval, tt.given.cleanupInterval)
			assert.Equal(t, tt.expected.enabled, cacheHandler.Enabled)
			assert.Equal(t, tt.expected.expirationInterval, cacheHandler.ExpirationInterval)
			assert.Equal(t, tt.expected.cleanupInterval, cacheHandler.CleanupInterval)
		})
	}
}

func TestCacheGet(t *testing.T) {
	t.Run("success get value", func(t *testing.T) {
		cacheHandler := NewCacheHandler(true, 5, 10)
		cacheHandler.Set("test", "value")
		value, found := cacheHandler.Get("test")
		assert.True(t, found)
		assert.Equal(t, "value", value)
	})
	t.Run("fail get value", func(t *testing.T) {
		cacheHandler := NewCacheHandler(true, 5, 10)
		value, found := cacheHandler.Get("test")
		assert.False(t, found)
		assert.Nil(t, value)
	})
	t.Run("override value", func(t *testing.T) {
		cacheHandler := NewCacheHandler(true, 5, 10)
		cacheHandler.Set("test", "value")
		cacheHandler.Set("test", "new value")
		value, found := cacheHandler.Get("test")
		assert.True(t, found)
		assert.Equal(t, "new value", value)
	})
	t.Run("expired value", func(t *testing.T) {
		cacheHandler := CacheHandler{
			Enabled:            true,
			ExpirationInterval: 1,
			CleanupInterval:    1,
			cache:              cache.New(time.Duration(1)*time.Millisecond, time.Duration(1)*time.Millisecond),
		}
		cacheHandler.cache.Set("test", "value", time.Duration(1)*time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		value, found := cacheHandler.Get("test")
		assert.False(t, found)
		assert.Nil(t, value)
	})
}

func TestCacheSet(t *testing.T) {
	t.Run("success set value", func(t *testing.T) {
		cacheHandler := NewCacheHandler(true, 5, 10)
		cacheHandler.Set("test", "value")
		value, found := cacheHandler.Get("test")
		assert.True(t, found)
		assert.Equal(t, "value", value)
	})
	t.Run("override value", func(t *testing.T) {
		cacheHandler := NewCacheHandler(true, 5, 10)
		cacheHandler.Set("test", "value")
		cacheHandler.Set("test", "new value")
		value, found := cacheHandler.Get("test")
		assert.True(t, found)
		assert.Equal(t, "new value", value)
	})
}

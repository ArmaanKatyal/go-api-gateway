package main

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type CacheHandler struct {
	Enabled            bool `json:"enabled"`
	ExpirationInterval int  `json:"expirationInterval"`
	CleanupInterval    int  `json:"cleanupInterval"`
	cache              *cache.Cache
}

func NewCacheHandler(enabled bool, expirationInterval int, cleanupInterval int) *CacheHandler {
	// If 0, set to default values
	if expirationInterval == 0 {
		expirationInterval = 5
	}
	if cleanupInterval == 0 {
		cleanupInterval = 10
	}
	return &CacheHandler{
		Enabled:            enabled,
		ExpirationInterval: expirationInterval,
		CleanupInterval:    cleanupInterval,
		cache: cache.New(time.Duration(expirationInterval)*time.Minute,
			time.Duration(cleanupInterval)*time.Minute),
	}
}

func (c *CacheHandler) Get(key string) (interface{}, bool) {
	return c.cache.Get(key)
}

func (c *CacheHandler) Set(key string, value interface{}) {
	c.cache.Set(key, value, time.Duration(c.ExpirationInterval)*time.Minute)
}

func (c *CacheHandler) IsEnabled() bool {
	return c.Enabled
}

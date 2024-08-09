package feature

import (
	"github.com/ArmaanKatyal/go_api_gateway/server/config"
	"time"

	"github.com/patrickmn/go-cache"
)

type CacheHandler struct {
	Enabled            bool `json:"enabled"`
	ExpirationInterval uint `json:"expirationInterval"`
	CleanupInterval    uint `json:"cleanupInterval"`
	cache              *cache.Cache
}

func NewCacheHandler(conf *config.CacheSettings) *CacheHandler {
	// If 0, set to default values
	var expInt uint
	var cleanupInt uint
	if conf.ExpirationInterval == 0 {
		expInt = 5
	}
	if conf.CleanupInterval == 0 {
		cleanupInt = 10
	}
	return &CacheHandler{
		Enabled:            conf.Enabled,
		ExpirationInterval: expInt,
		CleanupInterval:    cleanupInt,
		cache: cache.New(time.Duration(expInt)*time.Minute,
			time.Duration(cleanupInt)*time.Minute),
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

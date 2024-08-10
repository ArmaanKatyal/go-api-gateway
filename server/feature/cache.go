package feature

import (
	"time"

	"github.com/ArmaanKatyal/go_api_gateway/server/config"

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
	if conf.ExpirationInterval == 0 {
		conf.ExpirationInterval = 5
	}
	if conf.CleanupInterval == 0 {
		conf.CleanupInterval = 10
	}
	return &CacheHandler{
		Enabled:            conf.Enabled,
		ExpirationInterval: conf.ExpirationInterval,
		CleanupInterval:    conf.CleanupInterval,
		cache: cache.New(time.Duration(conf.ExpirationInterval)*time.Minute,
			time.Duration(conf.CleanupInterval)*time.Minute),
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

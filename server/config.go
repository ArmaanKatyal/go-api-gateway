package main

import (
	"encoding/json"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

var AppConfig Conf

type Conf struct {
	Server struct {
		Host           string `yaml:"host"`
		Port           string `yaml:"port"`
		ReadTimeout    int    `yaml:"readTimeout"`
		WriteTimeout   int    `yaml:"writeTimeout"`
		CircuitBreaker struct {
			Enabled  bool `yaml:"enabled"`
			Timeout  int  `yaml:"timeout"`
			Interval int  `yaml:"interval"`
		}
	}
	Registry struct {
		HeartbeatInterval int `yaml:"heartbeatInterval"`
		Services          []struct {
			Name        string   `yaml:"name"`
			Addr        string   `yaml:"addr"`
			WhiteList   []string `yaml:"whitelist"`
			FallbackUri string   `yaml:"fallbackUri"`
		}
	}
	RateLimiter struct {
		Enabled              bool `yaml:"enabled"`
		MaxRequestsPerMinute int  `yaml:"maxRequestsPerMinute"`
	}
}

// GetConfMarshal returns the configuration as a json byte array
func (c *Conf) GetConfMarshal() []byte {
	out, err := json.Marshal(c)
	if err != nil {
		return []byte{}
	}
	return out
}

// Verify checks if the configuration is valid
func (c *Conf) Verify() bool {
	if c.Server.Host == "" || c.Server.Port == "" {
		return false
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 5
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 10
	}
	if c.Registry.HeartbeatInterval == 0 {
		c.Registry.HeartbeatInterval = 30
	}
	if c.RateLimiter.MaxRequestsPerMinute == 0 {
		c.RateLimiter.MaxRequestsPerMinute = 100
	}
	if c.Server.CircuitBreaker.Timeout == 0 {
		c.Server.CircuitBreaker.Timeout = 10
	}
	if c.Server.CircuitBreaker.Interval == 0 {
		c.Server.CircuitBreaker.Interval = 10
	}
	return true
}

// LoadConf loads the configuration from the config.yaml file
func LoadConf() {
	c := Conf{}
	yamlFile, err := os.ReadFile("config.yaml")
	if err != nil {
		slog.Error("yamlFile.Get err", "error", err.Error())
	}
	err = yaml.Unmarshal(yamlFile, &c)
	if err != nil {
		slog.Error("yaml unmarshal error ocurred", "error", err.Error())
		os.Exit(1)
	}
	if !c.Verify() {
		slog.Error("Config verification failed")
		os.Exit(1)
	}
	AppConfig = c
	slog.Info("Config loaded successfully")
}

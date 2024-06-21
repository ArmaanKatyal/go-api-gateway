package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppConfig is the global configuration object
var AppConfig Conf

type Conf struct {
	Server struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
		// the maximum duration for reading the entire request, including the body
		ReadTimeout int `yaml:"readTimeout"`
		// the maximum duration before timing out writes of the response
		WriteTimeout int `yaml:"writeTimeout"`
		// the maximum duration before timing out the graceful shutdown
		GracefulTimeout int `yaml:"gracefulTimeout"`

		TLSConfig struct {
			Enabled bool `yaml:"enabled"`
			// path to the certificate and key files
			CertFile string `yaml:"certFile"`
			KeyFile  string `yaml:"keyFile"`
		}

		CircuitBreaker struct {
			Enabled  bool `yaml:"enabled"`
			Timeout  int  `yaml:"timeout"`
			Interval int  `yaml:"interval"`
		}

		Metrics struct {
			Prefix  string    `yaml:"prefix"`
			Buckets []float64 `yaml:"buckets"`
		} `yaml:"metrics"`
	}

	Registry struct {
		// Interval (secs) at which the service will send a heartbeat to all registered services
		HeartbeatInterval int `yaml:"heartbeatInterval"`
		Services          []struct {
			Name      string   `yaml:"name"`
			Addr      string   `yaml:"addr"`
			WhiteList []string `yaml:"whitelist"`
			// uri to redirect to if the service is down
			FallbackUri string `yaml:"fallbackUri"`
			Health      struct {
				Enabled bool `yaml:"enabled"`
				// path to the health check endpoint
				Uri string `yaml:"uri"`
			}
			Auth struct {
				Enabled bool `yaml:"enabled"`
				// Give the option to make requets with no/expired token to pas through
				Anonymous bool `yaml:"anonymous"`
				// path to the secret file
				Secret string `yaml:"secret"`
				// list of routes that require authentication
				Routes []string `yaml:"routes"`
			}
			Cache struct {
				Enabled            bool `yaml:"enabled"`
				ExpirationInterval int  `yaml:"expirationInterval"`
				CleanupInterval    int  `yaml:"cleanupInterval"`
			}
		}
	}

	RateLimiter struct {
		Enabled bool `yaml:"enabled"`
		// Maximum number of requests per minute
		MaxRequestsPerMinute int `yaml:"maxRequestsPerMinute"`
		// Interval (mins) at which the rate limiter will clean up old visitors
		CleanupInterval int `yaml:"cleanupInterval"`
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
	if c.RateLimiter.CleanupInterval == 0 {
		c.RateLimiter.CleanupInterval = 2
	}
	return true
}

// LoadConf loads the configuration from the config.yaml file
func LoadConf() {
	c := Conf{}
	yamlFile, err := os.ReadFile("./config/config.yaml")
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

func GetCertFile() string {
	// Append path to root folder
	certPath := filepath.Join(GetWd(), AppConfig.Server.TLSConfig.CertFile)
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		slog.Error("Certificate file not found", "path", certPath)
		os.Exit(1)
	}
	return certPath
}

func GetKeyFile() string {
	certPath := filepath.Join(GetWd(), AppConfig.Server.TLSConfig.KeyFile)
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		slog.Error("Key file not found", "path", certPath)
		os.Exit(1)
	}
	return certPath
}

func TLSEnabled() bool {
	return AppConfig.Server.TLSConfig.Enabled
}

func GetWd() string {
	wd, err := os.Getwd()
	if err != nil {
		slog.Error("Unable to get current working directory", "error", err.Error())
		os.Exit(1)
	}
	return wd
}

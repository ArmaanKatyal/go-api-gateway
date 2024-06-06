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
		Host         string `yaml:"host"`
		Port         string `yaml:"port"`
		ReadTimeout  int    `yaml:"readTimeout"`
		WriteTimeout int    `yaml:"writeTimeout"`
	}
	Registry struct {
		HeartbeatInterval int `yaml:"heartbeatInterval"`
		Services          []struct {
			Name string `yaml:"name"`
			Addr string `yaml:"addr"`
		}
	}
	RateLimiter struct {
		MaxRequestsPerMinute int `yaml:"maxRequestsPerMinute"`
	}
}

func (c *Conf) GetConf() *Conf {
	return c
}

func (c *Conf) GetConfMarshal() []byte {
	out, err := json.Marshal(c)
	if err != nil {
		return []byte{}
	}
	return out
}

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
	AppConfig = c
}

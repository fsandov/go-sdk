package config

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type AppConfig struct {
	AppName      string
	Environment  string
	Port         string
	Timezone     *time.Location
	Architecture string
	OS           string

	Extras map[string]interface{}
}

var (
	instance *AppConfig
	once     sync.Once
)

func Init(cfg *AppConfig) {
	once.Do(func() {
		if cfg.AppName == "" {
			cfg.AppName = "default-app"
		}
		if cfg.Environment == "" {
			cfg.Environment = "local"
		}
		if cfg.Port == "" {
			cfg.Port = "8080"
		}
		if cfg.Timezone == nil {
			cfg.Timezone = DetectTimezone()
		}
		if cfg.OS == "" {
			cfg.OS = runtime.GOOS
		}
		if cfg.Architecture == "" {
			cfg.Architecture = runtime.GOARCH
		}

		if cfg.Extras == nil {
			cfg.Extras = make(map[string]interface{})
		}
		instance = cfg
	})
}

func Get() *AppConfig {
	if instance == nil {
		Init(&AppConfig{})
	}
	return instance
}

func MustGet() *AppConfig {
	if instance == nil {
		panic("AppConfig not initialized: call config.Init first")
	}
	return instance
}

func (c *AppConfig) ExtraString(key string, fallback string) string {
	val, ok := c.Extras[key]
	if !ok {
		return fallback
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fallback
}

func (c *AppConfig) ExtraInt(key string, fallback int) int {
	val, ok := c.Extras[key]
	if !ok {
		return fallback
	}
	if i, ok := val.(int); ok {
		return i
	}
	if s, ok := val.(string); ok {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return fallback
}

func DetectTimezone() *time.Location {
	tzName := os.Getenv("TZ")
	if tzName != "" {
		if tz, err := time.LoadLocation(tzName); err == nil {
			return tz
		}
	}

	if tz, err := time.LoadLocation("America/Santiago"); err == nil {
		return tz
	}
	return time.UTC
}

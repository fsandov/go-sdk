package config

import (
	"sync"
	"testing"
)

func resetSingleton() {
	instance = nil
	once = sync.Once{}
}

func TestGetAutoInitializes(t *testing.T) {
	resetSingleton()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Get() panicked: %v", r)
		}
	}()

	cfg := Get()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.AppName != "default-app" {
		t.Errorf("expected default-app, got %s", cfg.AppName)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected 8080, got %s", cfg.Port)
	}
}

func TestMustGetPanicsWhenNotInit(t *testing.T) {
	resetSingleton()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected MustGet to panic")
		}
	}()

	MustGet()
}

func TestInitSetsValues(t *testing.T) {
	resetSingleton()

	Init(&AppConfig{
		AppName:     "my-app",
		Environment: "production",
		Port:        "3000",
	})

	cfg := Get()
	if cfg.AppName != "my-app" {
		t.Errorf("expected my-app, got %s", cfg.AppName)
	}
	if cfg.Environment != "production" {
		t.Errorf("expected production, got %s", cfg.Environment)
	}
	if cfg.Port != "3000" {
		t.Errorf("expected 3000, got %s", cfg.Port)
	}
}

func TestExtraHelpers(t *testing.T) {
	resetSingleton()

	Init(&AppConfig{
		Extras: map[string]interface{}{
			"strKey": "hello",
			"intKey": 42,
		},
	})

	cfg := Get()
	if v := cfg.ExtraString("strKey", "default"); v != "hello" {
		t.Errorf("expected hello, got %s", v)
	}
	if v := cfg.ExtraString("missing", "default"); v != "default" {
		t.Errorf("expected default, got %s", v)
	}
	if v := cfg.ExtraInt("intKey", 0); v != 42 {
		t.Errorf("expected 42, got %d", v)
	}
	if v := cfg.ExtraInt("missing", 99); v != 99 {
		t.Errorf("expected 99, got %d", v)
	}
}

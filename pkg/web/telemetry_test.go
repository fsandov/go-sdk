package web

import (
	"context"
	"testing"
	"time"

	"github.com/fsandov/go-sdk/pkg/config"
)

func init() {
	config.Init(&config.AppConfig{AppName: "test-app", Environment: "local"})
}

func TestSetupTelemetryDoesNotAutoShutdown(t *testing.T) {
	app := &GinApp{
		ginConfig: GinConfig{
			EnableTracing: false,
			EnableMetrics: true,
		},
	}

	err := app.setupTelemetry()
	if err != nil {
		t.Fatalf("setupTelemetry failed: %v", err)
	}

	if app.meter == nil {
		t.Fatal("expected meter provider to be set")
	}

	time.Sleep(6 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = app.ShutdownTelemetry(ctx)
	if err != nil {
		t.Fatalf("ShutdownTelemetry failed (provider was likely already shut down): %v", err)
	}
}

package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/fsandov/go-sdk/pkg/config"
	"github.com/fsandov/go-sdk/pkg/env"
	"github.com/fsandov/go-sdk/pkg/logs"
	"github.com/gin-gonic/gin"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

type GinApp struct {
	engine     *gin.Engine
	httpServer *http.Server
	logger     *logs.Logger
	tracer     *sdktrace.TracerProvider
	meter      *sdkmetric.MeterProvider
	ginConfig  GinConfig
}

type GinConfig struct {
	Port                string
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	ShutdownTimeout     time.Duration
	MaxHeaderBytes      int
	EnablePprof         bool
	EnableMetrics       bool
	EnableRequestID     bool
	EnableRecovery      bool
	EnableCompression   bool
	EnableCORS          bool
	EnableTracing       bool
	EnableGinPagination bool
	EnableXAuthAppToken bool
	OTELEndpoint        string
}

func DefaultGinConfig() *GinConfig {
	if env.IsRemote() {
		return &GinConfig{
			Port:                "8080",
			ReadTimeout:         15 * time.Second,
			WriteTimeout:        15 * time.Second,
			IdleTimeout:         60 * time.Second,
			ShutdownTimeout:     10 * time.Second,
			MaxHeaderBytes:      1 << 20,
			EnablePprof:         false,
			EnableMetrics:       false,
			EnableRequestID:     true,
			EnableRecovery:      true,
			EnableCompression:   true,
			EnableCORS:          true,
			EnableTracing:       true,
			EnableGinPagination: true,
			EnableXAuthAppToken: true,
			OTELEndpoint:        "otel-collector:4318",
		}
	}

	return &GinConfig{
		Port:                "8080",
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        15 * time.Second,
		IdleTimeout:         60 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		MaxHeaderBytes:      1 << 20,
		EnablePprof:         true,
		EnableMetrics:       true,
		EnableRequestID:     true,
		EnableRecovery:      true,
		EnableCompression:   true,
		EnableCORS:          true,
		EnableTracing:       false,
		EnableGinPagination: true,
		EnableXAuthAppToken: true,
		OTELEndpoint:        "otel-collector:4318",
	}
}

func New(config *GinConfig) *GinApp {
	engine := gin.New()
	engine.ContextWithFallback = true

	if config == nil {
		config = DefaultGinConfig()
	}

	app := &GinApp{
		engine:    engine,
		logger:    logs.GetLogger(),
		ginConfig: *config,
	}

	app.setupRoutes()
	app.setupMiddleware()
	app.startupLog()

	if app.ginConfig.EnableTracing {
		if err := app.setupTelemetry(); err != nil {
			app.logger.Error(context.Background(), "Failed to setup telemetry", zap.Error(err))
		}
	}

	return app
}

func (app *GinApp) Run() error {

	addr := fmt.Sprintf(":%s", app.ginConfig.Port)
	app.httpServer = &http.Server{
		Addr:           addr,
		Handler:        app.engine,
		ReadTimeout:    app.ginConfig.ReadTimeout,
		WriteTimeout:   app.ginConfig.WriteTimeout,
		IdleTimeout:    app.ginConfig.IdleTimeout,
		MaxHeaderBytes: app.ginConfig.MaxHeaderBytes,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		app.logger.Info(context.Background(), "Starting server", zap.String("address", addr))
		if err := app.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	cfg := config.Get()
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		app.logger.Warn(
			context.Background(),
			"Shutting down server...",
			zap.String("app", cfg.AppName),
			zap.String("env", cfg.Environment),
			logs.WithNotifier(),
		)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.ginConfig.ShutdownTimeout)
		defer cancel()

		if err := app.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server forced to shutdown: %w", err)
		}

		app.logger.Info(context.Background(), "Server exited properly")
		return nil
	}
}

func (app *GinApp) Shutdown(ctx context.Context) error {
	app.ShutdownTelemetry(ctx)
	if app.httpServer != nil {
		return app.httpServer.Shutdown(ctx)
	}
	return nil
}

func (app *GinApp) GetEngine() *gin.Engine {
	return app.engine
}

func (app *GinApp) Use(middleware gin.HandlerFunc) {
	app.engine.Use(middleware)
}

func (app *GinApp) startupLog() {
	cfg := config.Get()
	logs.Warn(context.Background(), "API started",
		zap.String("app", cfg.AppName),
		zap.String("env", cfg.Environment),
		zap.String("port", cfg.Port),
		zap.String("timezone", cfg.Timezone.String()),
		zap.String("os", cfg.OS),
		zap.String("arch", cfg.Architecture),
		zap.String("go_version", runtime.Version()),
		logs.WithNotifier(),
	)
}

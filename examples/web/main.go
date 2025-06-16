package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/fsandov/go-sdk/pkg/config"
	"github.com/fsandov/go-sdk/pkg/env"
	"github.com/fsandov/go-sdk/pkg/logs"
	"github.com/fsandov/go-sdk/pkg/web"
	"go.uber.org/zap"
)

func main() {
	logs.NewLogger()
	logs.AutoInitNotifiers()

	cfg := &config.AppConfig{
		AppName:     os.Getenv("APP_NAME"),
		Environment: env.GetEnvironment(),
	}

	config.Init(cfg)
	app := web.New(web.DefaultGinConfig())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := app.Run(); err != nil {
			logs.Error(context.Background(), "Failed to start server", zap.Error(err))
			stop()
		}
	}()

	<-ctx.Done()
	logs.Info(context.Background(), "Shutdown signal received. Shutting down app.")

}

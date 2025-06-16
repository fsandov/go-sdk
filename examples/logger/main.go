package main

import (
	"context"

	"github.com/fsandov/go-sdk/pkg/logs"
	"go.uber.org/zap"
)

func main() {
	logs.NewLogger()
	logs.AutoInitNotifiers() // <-- Auto init notifiers, like Discord

	logs.Info(context.Background(), "Startup ok", zap.String("component", "core"))
	logs.Error(context.Background(), "Something went wrong", zap.String("user", "fsandov"), zap.Int("code", 42), logs.WithNotifyTarget())

	logs.Flush()
}

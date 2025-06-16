package logs

import (
	"os"

	"github.com/fsandov/go-sdk/pkg/notifiers"
	"github.com/fsandov/go-sdk/pkg/notifiers/discord"
	"go.uber.org/zap"
)

func AutoInitNotifiers() {
	logger := GetLogger()
	levels := []string{"error", "warn", "info"}

	for _, lvl := range levels {
		envKey := "DISCORD_WEBHOOK_" + upper(lvl)
		if url := os.Getenv(envKey); url != "" {
			client, err := discord.NewClient(discord.WithURL(url))
			if err != nil {
				logger.zap.Error("Failed to init Discord notifier", zap.String("level", lvl), zap.Error(err))
				continue
			}
			username := "Logger" + capitalize(lvl) + "Manager"
			notifier := notifiers.NewDiscordNotifier(client, username)
			logger.AddNotifier(lvl, notifier)
			logger.zap.Info("Discord notifier configured", zap.String("level", lvl), zap.String("url", url))
		}
	}
}

func upper(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

package notifiers

import (
	"context"
	"fmt"

	"github.com/fsandov/go-sdk/pkg/notifiers/discord"
)

type DiscordNotifier struct {
	Client   *discord.Client
	Username string
}

func NewDiscordNotifier(client *discord.Client, username string) *DiscordNotifier {
	return &DiscordNotifier{
		Client:   client,
		Username: username,
	}
}

func (n *DiscordNotifier) Notify(ctx context.Context, level string, message string, fields map[string]any) error {
	content := fmt.Sprintf("**[%s]** %s", level, message)
	if len(fields) > 0 {
		content += "\n```json\n"
		for k, v := range fields {
			content += fmt.Sprintf("%s: %v\n", k, v)
		}
		content += "```"
	}

	return n.Client.SendWebhook(ctx, discord.WebhookPayload{
		Username: n.Username,
		Content:  content,
	})
}

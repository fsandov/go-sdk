package notifiers

import "context"

type Notifier interface {
	Notify(ctx context.Context, level string, message string, fields map[string]any) error
}

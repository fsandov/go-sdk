package logs

type LogOption interface {
	apply(*logOptions)
}

type logOptions struct {
	withNotifier bool
}

type withNotifierOption struct{}

func (o withNotifierOption) apply(opts *logOptions) {
	opts.withNotifier = true
}

func WithNotifier() LogOption {
	return withNotifierOption{}
}


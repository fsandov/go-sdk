package logs

type LogOption interface {
	apply(*logOptions)
}

type logOptions struct {
	withNotifier bool
	targets      []string
}

type withNotifierOption struct{}
type withNotifyTargetOption struct {
	targets []string
}

func (o withNotifierOption) apply(opts *logOptions) {
	opts.withNotifier = true
}
func (o withNotifyTargetOption) apply(opts *logOptions) {
	opts.withNotifier = true
	opts.targets = append(opts.targets, o.targets...)
}

func WithNotifier() LogOption {
	return withNotifierOption{}
}

func WithNotifyTarget(targets ...string) LogOption {
	return withNotifyTargetOption{targets: targets}
}

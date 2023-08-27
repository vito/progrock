package progrock

import "context"

type recorderKey struct{}

// ToContext returns a new context with the given Recorder attached.
func ToContext(ctx context.Context, recorder *Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, recorder)
}

// FromContext returns the Recorder attached to the given context, or a no-op
// Recorder if none is attached.
func FromContext(ctx context.Context) *Recorder {
	rec := ctx.Value(recorderKey{})
	if rec == nil {
		return NewRecorder(Discard{})
	}

	return rec.(*Recorder)
}

// WithGroup is shorthand for FromContext(ctx).WithGroup(name, opts...).
//
// It returns a new context with the resulting Recorder attached.
func WithGroup(ctx context.Context, name string, opts ...GroupOpt) (context.Context, *Recorder) {
	rec := FromContext(ctx).WithGroup(name, opts...)
	return ToContext(ctx, rec), rec
}

// RecorderToContext is deprecated; use ToContext instead.
func RecorderToContext(ctx context.Context, recorder *Recorder) context.Context {
	return ToContext(ctx, recorder)
}

// RecorderFromContext is deprecated; use FromContext instead.
func RecorderFromContext(ctx context.Context) *Recorder {
	return FromContext(ctx)
}

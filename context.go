package progrock

import "context"

func RecorderToContext(ctx context.Context, recorder *Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, recorder)
}

func RecorderFromContext(ctx context.Context) *Recorder {
	rec := ctx.Value(recorderKey{})
	if rec == nil {
		return NewRecorder(Discard{})
	}

	return rec.(*Recorder)
}

type recorderKey struct{}

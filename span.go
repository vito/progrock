package progrock

import (
	"context"

	"github.com/opencontainers/go-digest"
)

// Span ties together lower-level Progrock APIs to provide a higher-level
// OpenTelemetry-style API, using a Vertex like a span.
func Span(ctx context.Context, id, name string, opts ...VertexOpt) (context.Context, *VertexRecorder) {
	rec := FromContext(ctx)
	vtx := rec.Vertex(digest.Digest(id), name, opts...)
	rec = rec.WithParent(vtx.Vertex.Id)
	ctx = ToContext(ctx, rec)
	return ctx, vtx
}

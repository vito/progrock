package progrock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/opencontainers/go-digest"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// VertexRecorder records updates pertaining to a vertex.
type VertexRecorder struct {
	Vertex   *Vertex
	Recorder *Recorder
}

// VertexOpt is an option for creating a Vertex.
type VertexOpt func(*Vertex)

// WithInputs sets the inputs for the vertex.
func WithInputs(inputs ...digest.Digest) VertexOpt {
	return func(vertex *Vertex) {
		for _, input := range inputs {
			vertex.Inputs = append(vertex.Inputs, input.String())
		}
	}
}

// Internal marks the vertex as internal, meaning it will not be included in
// progress output by default.
func Internal() VertexOpt {
	return func(vertex *Vertex) {
		vertex.Internal = true
	}
}

// Vertex creates a new VertexRecorder for the given vertex.
//
// While the digest can technically be an arbitrary string, it is given a
// digest.Digest type hint to suggest that it should be content-addressed.
//
// The vertex will be associated to the group of the Recorder. The root
// Recorder keeps track of all group memberships seen for a given digest, and
// will emit a union of all groups.
func (recorder *Recorder) Vertex(dig digest.Digest, name string, opts ...VertexOpt) *VertexRecorder {
	now := Clock.Now()

	vtx := &Vertex{
		Id:      dig.String(),
		Name:    name,
		Started: timestamppb.New(now),
	}

	for _, o := range opts {
		o(vtx)
	}

	rec := &VertexRecorder{
		Recorder: recorder,
		Vertex:   vtx,
	}

	recorder.Record(&StatusUpdate{
		Vertexes: []*Vertex{
			vtx,
		},
		Memberships: []*Membership{
			{
				Group:    recorder.Group.Id,
				Vertexes: []string{vtx.Id},
			},
		},
	})

	return rec
}

// Stdout returns an io.Writer that sends log updates for the STDOUT stream.
func (recorder *VertexRecorder) Stdout() io.Writer {
	return &recordWriter{
		Stream:         LogStream_STDOUT,
		VertexRecorder: recorder,
	}
}

// Stderr returns an io.Writer that sends log updates for the STDERR stream.
func (recorder *VertexRecorder) Stderr() io.Writer {
	return &recordWriter{
		Stream:         LogStream_STDERR,
		VertexRecorder: recorder,
	}
}

// Complete marks the vertex as completed and sends an update.
func (recorder *VertexRecorder) Complete() {
	now := Clock.Now()

	if recorder.Vertex.Completed == nil {
		// avoid marking tasks as completed twice; could have been idempotently
		// created through a dependency
		recorder.Vertex.Completed = timestamppb.New(now)
	}

	recorder.sync()
}

// Error marks the vertex as errored and sends an update.
//
// If the error is context.Canceled or has it as a string suffix, the vertex
// will be marked as canceled instead.
func (recorder *VertexRecorder) Error(err error) {
	msg := err.Error()
	if errors.Is(err, context.Canceled) || strings.HasSuffix(err.Error(), context.Canceled.Error()) {
		recorder.Vertex.Canceled = true
	} else {
		recorder.Vertex.Error = &msg
	}
	recorder.sync()
}

// Output records an output digest for the vertex and sends an update.
func (recorder *VertexRecorder) Output(out digest.Digest) {
	recorder.Vertex.Outputs = append(recorder.Vertex.Outputs, out.String())
	recorder.sync()
}

func (recorder *VertexRecorder) Done(err error) {
	if err != nil {
		recorder.Error(err)
	}

	recorder.Complete()
}

// Cached marks the vertex as cached and sends an update.
func (recorder *VertexRecorder) Cached() {
	recorder.Vertex.Cached = true
	recorder.sync()
}

// sync sends an update for the vertex.
func (recorder *VertexRecorder) sync() {
	recorder.Recorder.Record(&StatusUpdate{
		Vertexes: []*Vertex{
			recorder.Vertex,
		},
	})
}

// Task starts a task for the vertex and sends an update.
func (recorder *VertexRecorder) Task(msg string, args ...interface{}) *TaskRecorder {
	name := fmt.Sprintf(msg, args...)

	now := Clock.Now()

	task := &VertexTask{
		Vertex:  recorder.Vertex.Id,
		Name:    name,
		Started: timestamppb.New(now),
	}

	rec := &TaskRecorder{
		Task:           task,
		VertexRecorder: recorder,
	}

	rec.sync()

	return rec
}

// ProgressTask starts a task for the vertex and sends an update.
//
// The task update includes the total value, which can be incremented with the
// returned recorder.
func (recorder *VertexRecorder) ProgressTask(total int64, msg string, args ...interface{}) *TaskRecorder {
	name := fmt.Sprintf(msg, args...)

	now := Clock.Now()

	task := &VertexTask{
		Vertex:  recorder.Vertex.Id,
		Name:    name,
		Started: timestamppb.New(now),
		Total:   total,
	}

	rec := &TaskRecorder{
		Task:           task,
		VertexRecorder: recorder,
	}

	rec.sync()

	return rec
}

type recordWriter struct {
	*VertexRecorder

	Stream LogStream
}

// Write sends a log update for the vertex.
func (w *recordWriter) Write(b []byte) (int, error) {
	d := make([]byte, len(b))
	copy(d, b)

	now := Clock.Now()

	w.Recorder.Record(&StatusUpdate{
		Logs: []*VertexLog{
			{
				Vertex:    w.Vertex.Id,
				Stream:    w.Stream,
				Data:      d,
				Timestamp: timestamppb.New(now),
			},
		},
	})

	return len(b), nil
}

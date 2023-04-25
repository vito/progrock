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

type VertexRecorder struct {
	Vertex   *Vertex
	Recorder *Recorder
}

func (recorder *Recorder) Vertex(dig digest.Digest, name string, inputs ...digest.Digest) *VertexRecorder {
	now := Clock.Now()

	vtx := &Vertex{
		Id: dig.String(),

		Name: name,

		Started: timestamppb.New(now),
	}

	for _, input := range inputs {
		vtx.Inputs = append(vtx.Inputs, input.String())
	}

	rec := &VertexRecorder{
		Recorder: recorder,
		Vertex:   vtx,
	}

	rec.sync()

	return rec
}

func (recorder *VertexRecorder) Stdout() io.Writer {
	return &recordWriter{
		Stream:         1,
		VertexRecorder: recorder,
	}
}

func (recorder *VertexRecorder) Stderr() io.Writer {
	return &recordWriter{
		Stream:         2,
		VertexRecorder: recorder,
	}
}

func (recorder *VertexRecorder) Complete() {
	now := Clock.Now()

	if recorder.Vertex.Completed == nil {
		// avoid marking tasks as completed twice; could have been idempotently
		// created through a dependency
		recorder.Vertex.Completed = timestamppb.New(now)
	}

	recorder.sync()
}

func (recorder *VertexRecorder) Error(err error) {
	msg := err.Error()
	recorder.Vertex.Error = &msg
	if errors.Is(err, context.Canceled) || strings.HasSuffix(err.Error(), context.Canceled.Error()) {
		recorder.Vertex.Canceled = true
	}
	recorder.sync()
}

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

func (recorder *VertexRecorder) Cached() {
	if recorder.Vertex.Completed != nil {
		// referenced again by another workload
		return
	}

	recorder.Vertex.Cached = true
	recorder.sync()
}

func (recorder *VertexRecorder) sync() {
	recorder.Recorder.Record(&StatusUpdate{
		Vertexes: []*Vertex{
			recorder.Vertex,
		},
	})
}

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

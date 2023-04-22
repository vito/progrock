package progrock

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/muesli/termenv"
	"github.com/opencontainers/go-digest"
	"github.com/vito/progrock/graph"
	"github.com/vito/progrock/ui"
)

// Clock is used to determine the current time.
var Clock = clockwork.NewRealClock()

type Writer interface {
	WriteStatus(*graph.SolveStatus)
	Close()
}

type Recorder struct {
	w Writer

	vertexes  map[digest.Digest]*VertexRecorder
	vertexesL sync.Mutex

	displaying *sync.WaitGroup
}

func NewRecorder(w Writer) *Recorder {
	return &Recorder{
		w: w,

		vertexes: map[digest.Digest]*VertexRecorder{},

		displaying: &sync.WaitGroup{},
	}
}

func (recorder *Recorder) Record(status *graph.SolveStatus) {
	for i, v := range status.Vertexes {
		cp := *v
		status.Vertexes[i] = &cp
	}

	for i, v := range status.Statuses {
		cp := *v
		status.Statuses[i] = &cp
	}

	recorder.w.WriteStatus(status)
}

func (recorder *Recorder) Display(interrupt context.CancelFunc, ui ui.Components, w io.Writer, r ui.Reader, tui bool) {
	recorder.displaying.Add(1)
	go func() {
		defer recorder.displaying.Done()
		err := ui.DisplaySolveStatus(interrupt, w, r, tui)
		if err != nil {
			fmt.Fprintf(w, "%s\n", termenv.String(fmt.Sprintf("display error: %s", err)).Foreground(termenv.ANSIRed))
		}
	}()
}

func (recorder *Recorder) Stop() {
	recorder.w.Close()
	recorder.displaying.Wait()
}

type recorderKey struct{}

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

type VertexRecorder struct {
	Vertex   *graph.Vertex
	Recorder *Recorder

	tasks     map[string]*TaskRecorder
	statusesL sync.Mutex
}

func (recorder *Recorder) Vertex(dig digest.Digest, name string) *VertexRecorder {
	recorder.vertexesL.Lock()
	defer recorder.vertexesL.Unlock()

	rec, found := recorder.vertexes[dig]
	if !found {
		now := Clock.Now()

		rec = &VertexRecorder{
			Recorder: recorder,

			Vertex: &graph.Vertex{
				Digest: dig,
				Name:   name,

				Started: &now,
			},

			tasks: map[string]*TaskRecorder{},
		}

		recorder.vertexes[dig] = rec
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
		recorder.Vertex.Completed = &now
	}

	recorder.sync()
}

func (recorder *VertexRecorder) Error(err error) {
	recorder.Vertex.Error = err.Error()
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
	recorder.Recorder.Record(&graph.SolveStatus{
		Vertexes: []*graph.Vertex{
			recorder.Vertex,
		},
	})
}

func (recorder *VertexRecorder) Task(msg string, args ...interface{}) *TaskRecorder {
	recorder.statusesL.Lock()
	defer recorder.statusesL.Unlock()

	id := fmt.Sprintf(msg, args...)

	task, found := recorder.tasks[id]
	if !found {
		now := Clock.Now()
		task = &TaskRecorder{
			Status: &graph.VertexStatus{
				ID:        id,
				Vertex:    recorder.Vertex.Digest,
				Name:      "?name?: " + id, // unused/deprecated?
				Timestamp: now,
			},
			VertexRecorder: recorder,
		}

		recorder.tasks[id] = task
	}

	task.sync()

	return task
}

type TaskRecorder struct {
	*VertexRecorder

	Status *graph.VertexStatus
}

func (recorder *TaskRecorder) Wrap(f func() error) error {
	recorder.Start()

	err := f()
	recorder.Done(err)

	return err
}

func (recorder *TaskRecorder) Done(err error) {
	if err != nil {
		recorder.Error(err)
	}

	recorder.Complete()
}

func (recorder *TaskRecorder) Start() {
	now := Clock.Now()
	recorder.Status.Started = &now
	recorder.sync()
}

func (recorder *TaskRecorder) Complete() {
	now := Clock.Now()

	if recorder.Status.Started == nil {
		recorder.Status.Started = &now
	}

	recorder.Status.Completed = &now
	recorder.sync()
}

func (recorder *TaskRecorder) Progress(cur, total int64) {
	recorder.Status.Current = cur
	recorder.Status.Total = total
	recorder.sync()
}

func (recorder *TaskRecorder) sync() {
	recorder.Recorder.Record(&graph.SolveStatus{
		Statuses: []*graph.VertexStatus{
			recorder.Status,
		},
	})
}

type recordWriter struct {
	*VertexRecorder

	Stream int
}

func (w *recordWriter) Write(b []byte) (int, error) {
	d := make([]byte, len(b))
	copy(d, b)

	w.Recorder.Record(&graph.SolveStatus{
		Logs: []*graph.VertexLog{
			{
				Vertex:    w.Vertex.Digest,
				Stream:    w.Stream,
				Data:      d,
				Timestamp: time.Now(), // XXX: omit?
			},
		},
	})

	return len(b), nil
}

package progrock

import (
	"io"
	"sync"

	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
)

// Clock is used to determine the current time.
var Clock = clockwork.NewRealClock()

type Recorder struct {
	w Writer

	displaying *sync.WaitGroup
}

func NewRecorder(w Writer) *Recorder {
	return &Recorder{
		w: w,

		displaying: &sync.WaitGroup{},
	}
}

type LogSink struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (recorder *Recorder) Record(status *StatusUpdate) {
	for i, v := range status.Vertexes {
		cp := proto.Clone(v)
		status.Vertexes[i] = cp.(*Vertex)
	}

	for i, t := range status.Tasks {
		cp := proto.Clone(t)
		status.Tasks[i] = cp.(*VertexTask)
	}

	recorder.w.WriteStatus(status)
}

func (recorder *Recorder) Close() error {
	return recorder.w.Close()
}

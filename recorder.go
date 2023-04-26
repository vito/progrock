package progrock

import (
	"fmt"

	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// Clock is used to determine the current time.
var Clock = clockwork.NewRealClock()

type Recorder struct {
	w Writer
	g *Group
}

// RootGroup is the name of the toplevel group, which is blank.
const RootGroup = ""

// NewRecorder creates a new Recorder, which writes to the given Writer.
//
// It also initializes the "root" group with the associated labels, which
// involves sending a progress update for the group.
func NewRecorder(w Writer, labels ...*Label) *Recorder {
	return (&Recorder{
		w: w,
	}).Group(RootGroup, labels...)
}

// Record sends a deep-copy of the status update to the Writer.
func (recorder *Recorder) Record(status *StatusUpdate) {
	// perform a deep-copy so buffered writes don't get mutated, similar to
	// copying in Write([]byte) when []byte comes from sync.Pool
	recorder.w.WriteStatus(proto.Clone(status).(*StatusUpdate))
}

func (recorder *Recorder) Group(name string, labels ...*Label) *Recorder {
	now := Clock.Now()

	id := fmt.Sprintf("%s@%d", name, now.UnixNano())

	g := &Group{
		Id:      id,
		Name:    name,
		Labels:  labels,
		Started: timestamppb.New(now),
	}

	if recorder.g != nil {
		g.Parent = &recorder.g.Id
	}

	recorder.sync()

	return &Recorder{
		w: recorder.w,
		g: g,
	}
}

func (recorder *Recorder) Complete() {
	recorder.g.Completed = timestamppb.New(Clock.Now())
	recorder.sync()
}

func (recorder *Recorder) Close() error {
	return recorder.w.Close()
}

func (recorder *Recorder) sync() {
	recorder.Record(&StatusUpdate{
		Groups: []*Group{recorder.g},
	})
}

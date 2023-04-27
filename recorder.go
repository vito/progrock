package progrock

import (
	"fmt"

	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// Clock is used to determine the current time.
var Clock = clockwork.NewRealClock()

// Recorder is a Writer that also tracks a current group.
type Recorder struct {
	w Writer

	Group *Group
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
	}).WithGroup(RootGroup, labels...)
}

// Record sends a deep-copy of the status update to the Writer.
func (recorder *Recorder) Record(status *StatusUpdate) {
	// perform a deep-copy so buffered writes don't get mutated, similar to
	// copying in Write([]byte) when []byte comes from sync.Pool
	update := proto.Clone(status).(*StatusUpdate)

	for _, vertex := range update.Vertexes {
		if vertex.Group == nil {
			vertex.Group = &recorder.Group.Id
		}
	}

	recorder.w.WriteStatus(update)
}

// WithGroup creates a new group with the given name and labels, and sends a
// progress update.
func (recorder *Recorder) WithGroup(name string, labels ...*Label) *Recorder {
	now := Clock.Now()

	id := fmt.Sprintf("%s@%d", name, now.UnixNano())

	g := &Group{
		Id:      id,
		Name:    name,
		Labels:  labels,
		Started: timestamppb.New(now),
	}

	if recorder.Group != nil {
		g.Parent = &recorder.Group.Id
	}

	subRecorder := &Recorder{
		w:     recorder.w,
		Group: g,
	}

	subRecorder.sync()

	return subRecorder
}

// Complete marks the current group as complete, and sends a progress update.
func (recorder *Recorder) Complete() {
	recorder.Group.Completed = timestamppb.New(Clock.Now())
	recorder.sync()
}

// Close closes the underlying Writer.
func (recorder *Recorder) Close() error {
	return recorder.w.Close()
}

// sync sends a progress update for the current group.
func (recorder *Recorder) sync() {
	recorder.Record(&StatusUpdate{
		Groups: []*Group{recorder.Group},
	})
}

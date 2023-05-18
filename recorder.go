package progrock

import (
	"fmt"
	"sync"

	"github.com/jonboulle/clockwork"
	"github.com/opencontainers/go-digest"
	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// Clock is used to determine the current time.
var Clock = clockwork.NewRealClock()

// Recorder is a Writer that also tracks a current group.
type Recorder struct {
	w Writer

	Group *Group

	groups  map[string]*Recorder
	groupsL sync.Mutex
}

// RootGroup is the name of the toplevel group, which is blank.
//
// This is a slight hack, but it gives reasonable meaning to an empty state
// while sidestepping the issue of figuring out what the "root" name should be.
const RootGroup = ""

// NewRecorder creates a new Recorder, which writes to the given Writer.
//
// It also initializes the "root" group with the associated labels, which
// involves sending a progress update for the group.
func NewRecorder(w Writer, labels ...*Label) *Recorder {
	return newEmptyRecorder(w).WithGroup(RootGroup, labels...)
}

func newEmptyRecorder(w Writer) *Recorder {
	return &Recorder{
		w:      w,
		groups: map[string]*Recorder{},
	}
}

// Recorder is also a Writer so that you can forward events to it directly.
var _ Writer = (*Recorder)(nil)

// WriteStatus sends a deep-copy of the status update to the Writer.
func (recorder *Recorder) WriteStatus(status *StatusUpdate) error {
	// perform a deep-copy so buffered writes don't get mutated, similar to
	// copying in Write([]byte) when []byte comes from sync.Pool
	return recorder.w.WriteStatus(proto.Clone(status).(*StatusUpdate))
}

// WithGroup creates a new group with the given name and labels and sends a
// progress update.
//
// Calling WithGroup with the same name will always return the same Recorder
// instance so that you can record to a single hierarchy of groups. When the
// group already exists, the labels argument is ignored.
func (recorder *Recorder) WithGroup(name string, labels ...*Label) *Recorder {
	recorder.groupsL.Lock()
	defer recorder.groupsL.Unlock()

	existing, found := recorder.groups[name]
	if found {
		return existing
	}

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

	subRecorder := newEmptyRecorder(recorder.w)
	subRecorder.Group = g
	subRecorder.sync()

	recorder.groups[name] = subRecorder

	return subRecorder
}

// Join sends a progress update that the given vertexes are members of the
// current group.
func (recorder *Recorder) Join(vertexes ...digest.Digest) {
	strs := make([]string, len(vertexes))
	for i, v := range vertexes {
		strs[i] = v.String()
	}

	recorder.WriteStatus(&StatusUpdate{
		Memberships: []*Membership{
			{
				Group:    recorder.Group.Id,
				Vertexes: strs,
			},
		},
	})
}

// Complete marks the current group and all sub-groups as complete, and sends a
// progress update for each.
func (recorder *Recorder) Complete() {
	for _, g := range recorder.groups {
		g.Complete()
	}
	recorder.Group.Completed = timestamppb.New(Clock.Now())
	recorder.sync()
}

// Close closes the underlying Writer.
func (recorder *Recorder) Close() error {
	return recorder.w.Close()
}

// sync sends a progress update for the current group.
func (recorder *Recorder) sync() {
	recorder.WriteStatus(&StatusUpdate{
		Groups: []*Group{recorder.Group},
	})
}

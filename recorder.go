package progrock

import (
	"fmt"
	"sync"
	"time"

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
// It also initializes the "root" group and sends a progress update for the
// group.
func NewRecorder(w Writer, opts ...GroupOpt) *Recorder {
	return newEmptyRecorder(w).WithGroup(RootGroup, opts...)
}

func newEmptyRecorder(w Writer) *Recorder {
	return &Recorder{
		w:      w,
		groups: map[string]*Recorder{},
	}
}

// Record sends a deep-copy of the status update to the Writer.
func (recorder *Recorder) Record(status *StatusUpdate) error {
	// perform a deep-copy so buffered writes don't get mutated, similar to
	// copying in Write([]byte) when []byte comes from sync.Pool
	return recorder.w.WriteStatus(proto.Clone(status).(*StatusUpdate))
}

// GroupOpt is an option for creating a Group.
type GroupOpt func(*Group)

// WithLabels sets labels on the group.
func WithLabels(labels ...*Label) GroupOpt {
	return func(g *Group) {
		g.Labels = append(g.Labels, labels...)
	}
}

// Weak indicates that the group should not be considered equal to non-weak
// groups. Weak groups may be used to group together vertexes that correspond
// to a single API (e.g. a Dockerfile build), as opposed to "strong" groups
// explicitly configured by the user (e.g. "test", "build", etc).
func Weak() GroupOpt {
	return func(g *Group) {
		g.Weak = true
	}
}

// WithStarted sets the start time of the group.
func WithStarted(started time.Time) GroupOpt {
	return func(g *Group) {
		g.Started = timestamppb.New(started)
	}
}

// WithGroupID sets the ID for the group. An ID should be globally unique. If
// not specified, the ID defaults to the group's name with the group's start
// time appended.
func WithGroupID(id string) GroupOpt {
	return func(g *Group) {
		g.Id = id
	}
}

// WithGroup creates a new group and sends a progress update.
//
// Calling WithGroup with the same name will always return the same Recorder
// instance so that you can record to a single hierarchy of groups. When the
// group already exists, opts are ignored.
func (recorder *Recorder) WithGroup(name string, opts ...GroupOpt) *Recorder {
	recorder.groupsL.Lock()
	defer recorder.groupsL.Unlock()

	existing, found := recorder.groups[name]
	if found {
		return existing
	}

	g := &Group{
		Name: name,
	}

	for _, o := range opts {
		o(g)
	}

	if g.Started == nil {
		g.Started = timestamppb.New(Clock.Now())
	}

	if g.Id == "" {
		g.Id = fmt.Sprintf("%s@%d", name, g.Started.AsTime().UnixNano())
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

	recorder.Record(&StatusUpdate{
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
	recorder.Record(&StatusUpdate{
		Groups: []*Group{recorder.Group},
	})
}

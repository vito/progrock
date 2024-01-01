package console

import (
	"bytes"
	"container/ring"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/muesli/termenv"
	"github.com/vito/midterm"
	"github.com/vito/progrock"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type trace struct {
	clock        clockwork.Clock
	ui           Components
	groupsById   map[string]*group
	verticesById map[string]*vertex
	nextIndex    int
	updates      map[string]struct{}
	memberships  map[string][]string
}

type vertex struct {
	*progrock.Vertex

	clock clockwork.Clock

	tasks       []string
	tasksByName map[string]*progrock.VertexTask
	indent      string
	index       int

	logs          [][]byte
	logsPartial   bool
	logsOffset    int
	logsBuffer    *ring.Ring // stores last logs to print them on error
	prev          *progrock.Vertex
	lastBlockTime *time.Time
	updates       int
	taskUpdates   map[string]struct{}

	term      *midterm.Terminal
	termBytes int
}

func (v *vertex) name() string {
	return strings.Join(strings.Fields(v.Name), " ")
}

func (v *vertex) update() {
	if v.updates == 0 {
		serverNow := v.clock.Now()
		v.lastBlockTime = &serverNow
	}
	v.updates++
}

type group struct {
	*progrock.Group
}

func (g *group) name(t *trace) string {
	name := g.Name

	seen := map[string]struct{}{}
	p := g.Parent
	for p != nil {
		if _, ok := seen[*p]; ok {
			log.Println("!!! LOOP DETECTED", *p, seen)
			break
		}
		seen[*p] = struct{}{}

		pg := t.groupsById[*p]
		if pg.Name == progrock.RootGroup {
			break
		}
		name = pg.Name + " > " + name
		p = pg.Parent
	}

	return name
}

func newTrace(ui Components, clock clockwork.Clock) *trace {
	return &trace{
		clock:        clock,
		ui:           ui,
		verticesById: make(map[string]*vertex),
		groupsById:   make(map[string]*group),
		updates:      make(map[string]struct{}),
		memberships:  make(map[string][]string),
	}
}

func (t *trace) triggerVertexEvent(v *progrock.Vertex) {
	if v.Started == nil {
		return
	}

	var old *progrock.Vertex
	vtx := t.verticesById[v.Id]
	if prev := vtx.prev; prev != nil {
		old = proto.Clone(prev).(*progrock.Vertex)
	}

	var changed bool
	if old == nil {
		changed = true
	} else {
		if v.Id != old.Id {
			changed = true
		}
		if v.Name != old.Name {
			changed = true
		}
		if v.Started != old.Started {
			if v.Started != nil && old.Started == nil || !v.Started.AsTime().Equal(old.Started.AsTime()) {
				changed = true
			}
		}
		if v.Completed != old.Completed && v.Completed != nil {
			changed = true
		}
		if v.Cached != old.Cached {
			changed = true
		}
		if v.Error != old.Error {
			changed = true
		}
	}

	if changed {
		vtx.update()
		t.updates[v.Id] = struct{}{}
	}

	t.verticesById[v.Id].prev = v
}

func (t *trace) update(update *progrock.StatusUpdate) {
	for _, v := range update.Vertexes {
		prev, ok := t.verticesById[v.Id]
		if !ok {
			t.nextIndex++
			t.verticesById[v.Id] = &vertex{
				clock:       t.clock,
				tasksByName: make(map[string]*progrock.VertexTask),
				taskUpdates: make(map[string]struct{}),
				index:       t.nextIndex,
			}
		}
		t.triggerVertexEvent(v)
		// allow a duplicate initial vertex that shouldn't reset state
		if prev == nil || prev.Started == nil || v.Started != nil {
			t.verticesById[v.Id].Vertex = v
		}
	}

	for _, task := range update.Tasks {
		v, ok := t.verticesById[task.Vertex]
		if !ok {
			continue // shouldn't happen
		}
		prev, ok := v.tasksByName[task.Name]
		if !ok {
			v.tasksByName[task.Name] = task
		}
		if task.Started != nil && (prev == nil || prev.Started == nil) {
			v.tasks = append(v.tasks, task.Name)
		}
		v.tasksByName[task.Name] = task
		v.taskUpdates[task.Name] = struct{}{}
		t.updates[v.Id] = struct{}{}
		v.update()
	}

	for _, l := range update.Logs {
		v, ok := t.verticesById[l.Vertex]
		if !ok {
			continue // shouldn't happen
		}
		i := 0
		complete := split(l.Data, '\n', func(dt []byte) {
			if v.logsPartial && len(v.logs) != 0 && i == 0 {
				v.logs[len(v.logs)-1] = append(v.logs[len(v.logs)-1], dt...)
			} else {
				delta := time.Duration(0)
				if v.Started != nil {
					delta = l.Timestamp.AsTime().Sub(v.Started.AsTime())
				}

				v.logs = append(v.logs, []byte(fmt.Sprintf(t.ui.TextLogFormat, v.index, duration(t.ui, delta, v.Completed != nil), dt)))
			}
			i++
		})
		v.logsPartial = !complete
		t.updates[v.Id] = struct{}{}
		v.update()
	}

	for _, g := range update.Groups {
		t.groupsById[g.Id] = &group{Group: g}
	}

	for _, m := range update.Memberships {
		for _, vid := range m.Vertexes {
			var alreadyIn bool
			for _, g := range t.memberships[vid] {
				if g == m.Group {
					alreadyIn = true
					break
				}
			}

			if !alreadyIn {
				t.memberships[vid] = append(t.memberships[vid], m.Group)
			}
		}
	}
}

func duration(ui Components, dt time.Duration, completed bool) string {
	prec := 1
	sec := dt.Seconds()
	if sec < 10 {
		prec = 2
	} else if sec < 100 {
		prec = 1
	}

	if completed {
		return fmt.Sprintf(ui.DoneDuration, sec, prec)
	} else {
		return fmt.Sprintf(ui.RunningDuration, sec, prec)
	}
}

func split(dt []byte, sep byte, fn func([]byte)) bool {
	if len(dt) == 0 {
		return false
	}
	for {
		if len(dt) == 0 {
			return true
		}
		idx := bytes.IndexByte(dt, sep)
		if idx == -1 {
			fn(dt)
			return false
		}
		fn(dt[:idx])
		dt = dt[idx+1:]
	}
}

func addTime(tm *timestamppb.Timestamp, d time.Duration) *time.Time {
	if tm == nil {
		return nil
	}
	t := tm.AsTime().Add(d)
	return &t
}

type Components struct {
	TextContextSwitched           string
	TextLogFormat                 string
	TextVertexRunning             string
	TextVertexCanceled            string
	TextVertexErrored             string
	TextVertexCached              string
	TextVertexDone                string
	TextVertexDoneDuration        string
	TextVertexGroup               string
	TextVertexTask                string
	TextVertexTaskDuration        string
	TextVertexTaskProgressBound   string
	TextVertexTaskProgressUnbound string

	RunningDuration, DoneDuration string
}

func DefaultUI(out *termenv.Output) Components {
	var vertexID = out.String("%d:").Foreground(termenv.ANSIMagenta).String()

	return Components{
		TextLogFormat:                 vertexID + " %s %s",
		TextContextSwitched:           vertexID + " ...\n",
		TextVertexRunning:             vertexID + " %s",
		TextVertexCanceled:            vertexID + " %s " + out.String("CANCELED").Foreground(termenv.ANSIYellow).String(),
		TextVertexErrored:             vertexID + " %s " + out.String("ERROR: %s").Foreground(termenv.ANSIRed).String(),
		TextVertexCached:              vertexID + " %s " + out.String("CACHED").Foreground(termenv.ANSICyan).String(),
		TextVertexDone:                vertexID + " %s " + out.String("DONE").Foreground(termenv.ANSIGreen).String(),
		TextVertexGroup:               vertexID + " > in " + out.String("%s").Foreground(termenv.ANSIBlue).String(),
		TextVertexTask:                vertexID + " %[3]s %[2]s",
		TextVertexTaskProgressBound:   "%s / %s",
		TextVertexTaskProgressUnbound: "%s",
		TextVertexTaskDuration:        "%.1fs",

		RunningDuration: "[%.[2]*[1]fs]",
		DoneDuration:    out.String("[%.[2]*[1]fs]").Foreground(termenv.ANSIBrightBlack).String(),
	}
}

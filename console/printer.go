package console

import (
	"container/ring"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/vito/progrock"
)

// AntiFlicker is used to prevent bouncing between concurrent vertices too
// aggressively.
const AntiFlicker = 5 * time.Second

// MaxDelay is the maximum amount of time to favor the current vertex over
// other active vertices.
const MaxDelay = 10 * time.Second

// MinTimeDelta is the minimum amount of time to require before printing
// updates to a task.
const MinTimeDelta = 5 * time.Second

// MinProgressDelta is the minimum progress percent to require before printing
// updates to a task.
const MinProgressDelta = 0.05 // %

const logsBufferSize = 10

type lastStatus struct {
	Current   int64
	Timestamp time.Time
}

type textMux struct {
	w            io.Writer
	ui           Components
	current      string
	last         map[string]lastStatus
	notFirst     bool
	showInternal bool
}

func (p *textMux) printVtx(t *trace, dgst string) {
	if p.last == nil {
		p.last = make(map[string]lastStatus)
	}

	v, ok := t.verticesById[dgst]
	if !ok {
		return
	}

	if v.Internal && !p.showInternal {
		return
	}

	if dgst != p.current {
		if p.current != "" {
			old := t.verticesById[p.current]
			if old.logsPartial {
				fmt.Fprintln(p.w, "")
			}
			old.logsOffset = 0
			old.updates = 0
			fmt.Fprintf(p.w, p.ui.TextContextSwitched, old.index)
		}

		if p.notFirst {
			fmt.Fprintln(p.w, "")
		} else {
			p.notFirst = true
		}

		p.printHeader(v)
		p.printGroups(v, t)
	}

	for _, name := range v.tasks {
		if _, ok := v.taskUpdates[name]; !ok {
			continue
		}

		task := v.tasksByName[name]

		doPrint := true

		if last, ok := p.last[task.Name]; ok && task.Completed == nil {
			var progressDelta float64
			if task.Total > 0 {
				progressDelta = float64(task.Current-last.Current) / float64(task.Total)
			}
			timeDelta := task.Started.AsTime().Sub(last.Timestamp)
			if progressDelta < MinProgressDelta && timeDelta < MinTimeDelta {
				doPrint = false
			}
		}

		if !doPrint {
			continue
		}

		p.last[task.Name] = lastStatus{
			Timestamp: task.Started.AsTime(),
			Current:   task.Current,
		}

		segs := []string{task.Name}

		if task.Total != 0 {
			segs = append(segs, fmt.Sprintf(p.ui.TextVertexTaskProgressBound, units.BytesSize(float64(task.Current)), units.BytesSize(float64(task.Total))))
		} else if task.Current != 0 {
			segs = append(segs, fmt.Sprintf(p.ui.TextVertexTaskProgressUnbound, units.BytesSize(float64(task.Current))))
		}

		var dur string
		if task.Completed != nil {
			dur = duration(p.ui, task.Completed.AsTime().Sub(task.Started.AsTime()), task.Completed != nil)
		}

		fmt.Fprintf(p.w, p.ui.TextVertexTask, v.index, dur, strings.Join(segs, " "))
		fmt.Fprintln(p.w)
	}
	v.taskUpdates = map[string]struct{}{}

	for i, l := range v.logs {
		if i == 0 {
			l = l[v.logsOffset:]
		}
		fmt.Fprintf(p.w, "%s", []byte(l))
		if i != len(v.logs)-1 || !v.logsPartial {
			fmt.Fprintln(p.w, "")
		}
		if v.logsBuffer == nil {
			v.logsBuffer = ring.New(logsBufferSize)
		}
		v.logsBuffer.Value = l
		if !v.logsPartial {
			v.logsBuffer = v.logsBuffer.Next()
		}
	}

	if len(v.logs) > 0 {
		if v.logsPartial {
			v.logs = v.logs[len(v.logs)-1:]
			v.logsOffset = len(v.logs[0])
		} else {
			v.logs = nil
			v.logsOffset = 0
		}
	}

	p.current = dgst
	if v.Completed != nil {
		p.current = ""
		v.updates = 0

		if v.logsPartial {
			fmt.Fprintln(p.w, "")
		}

		p.printHeader(v)
	}

	delete(t.updates, dgst)
}

func (p *textMux) printHeader(v *vertex) {
	if v.Canceled {
		fmt.Fprintf(p.w, p.ui.TextVertexCanceled, v.index, v.name())
	} else if v.Error != nil {
		fmt.Fprintf(p.w, p.ui.TextVertexErrored, v.index, v.name(), *v.Error)
	} else if v.Cached {
		fmt.Fprintf(p.w, p.ui.TextVertexCached, v.index, v.name())
	} else if v.Completed != nil {
		fmt.Fprintf(p.w, p.ui.TextVertexDone, v.index, v.name())
	} else if v.Started != nil {
		fmt.Fprintf(p.w, p.ui.TextVertexRunning, v.index, v.name())
	} else {
		fmt.Fprintf(p.w, p.ui.TextVertexDone, v.index, v.name())
	}

	fmt.Fprintln(p.w)
}

func (p *textMux) printGroups(v *vertex, t *trace) {
	for _, gid := range t.memberships[v.Id] {
		g, found := t.groupsById[gid]
		if !found {
			continue
		}

		if g.Name == progrock.RootGroup {
			// ignore root group
			continue
		}

		fmt.Fprintf(p.w, p.ui.TextVertexGroup, v.index, g.name(t))
		fmt.Fprintln(p.w)
	}
}

func sortCompleted(t *trace, m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Slice(out, func(i, j int) bool {
		iv := t.verticesById[out[i]]
		jv := t.verticesById[out[j]]
		return iv.Completed.AsTime().Before(jv.Completed.AsTime()) ||
			iv.index < jv.index
	})

	return out
}

func (p *textMux) print(t *trace) {
	completed := map[string]struct{}{}
	rest := map[string]struct{}{}

	for id := range t.updates {
		v, ok := t.verticesById[id]
		if !ok {
			continue
		}
		if v.Completed != nil {
			completed[id] = struct{}{}
		} else {
			rest[id] = struct{}{}
		}
	}

	current := p.current

	// items that have completed need to be printed first
	if _, ok := completed[current]; ok {
		p.printVtx(t, current)
	}

	for _, dgst := range sortCompleted(t, completed) {
		if dgst != current {
			p.printVtx(t, dgst)
		}
	}

	if len(rest) == 0 {
		if current != "" {
			if v := t.verticesById[current]; v.Started != nil && v.Completed == nil {
				return
			}
		}
		// make any open vertex active
		for dgst, v := range t.verticesById {
			if v.Started != nil && v.Completed == nil {
				p.printVtx(t, dgst)
				return
			}
		}
		return
	}

	// now print the active one
	if _, ok := rest[current]; ok {
		p.printVtx(t, current)
	}

	stats := map[string]*vtxStat{}
	now := t.clock.Now()
	sum := 0.0
	var max string
	if current != "" {
		rest[current] = struct{}{}
	}
	for id := range rest {
		v, ok := t.verticesById[id]
		if !ok {
			continue
		}
		tm := now.Sub(*v.lastBlockTime)
		speed := float64(v.updates) / tm.Seconds()
		reveal := (tm > MaxDelay || v.GetFocused()) && id != current
		stats[id] = &vtxStat{blockTime: tm, speed: speed, reveal: reveal}
		sum += speed
		if reveal || max == "" || stats[max].speed < speed {
			max = id
		}
	}
	for dgst := range stats {
		stats[dgst].share = stats[dgst].speed / sum
	}

	if _, ok := completed[current]; ok || current == "" {
		p.printVtx(t, max)
		return
	}

	// show items that were hidden
	for dgst := range rest {
		if stats[dgst].reveal {
			p.printVtx(t, dgst)
			return
		}
	}

	// fair split between vertexes
	if 1.0/(1.0-stats[current].share)*AntiFlicker.Seconds() < stats[current].blockTime.Seconds() {
		p.printVtx(t, max)
		return
	}
}

type vtxStat struct {
	blockTime time.Duration
	speed     float64
	share     float64
	reveal    bool
}

func limitString(s string, l int) string {
	if len(s) > l {
		return s[:l] + "..."
	}
	return s
}

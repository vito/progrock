package progrock

import (
	"fmt"
	"io"
	"sync"

	"github.com/vito/progrock/ui"
)

type Casette struct {
	order    []string
	vertexes map[string]*Vertex
	tasks    map[string][]*VertexTask
	logs     map[string]*ui.Vterm
	done     bool

	width, height int

	l sync.Mutex
}

func NewCasette() *Casette {
	return &Casette{
		vertexes: make(map[string]*Vertex),
		tasks:    make(map[string][]*VertexTask),
		logs:     make(map[string]*ui.Vterm),

		// sane defaults before size is received
		width:  80,
		height: 24,
	}
}

var _ Writer = &Casette{}

func (casette *Casette) WriteStatus(status *StatusUpdate) error {
	casette.l.Lock()
	defer casette.l.Unlock()

	for _, v := range status.Vertexes {
		existing, found := casette.vertexes[v.Id]
		if !found {
			casette.insert(v)
		} else if existing.Completed != nil && v.Cached {
			// ignore redundant cache hit
			continue
		}

		casette.vertexes[v.Id] = v
	}

	for _, t := range status.Tasks {
		tasks := casette.tasks[t.Vertex]
		var updated bool
		for i, task := range tasks {
			if task.Name == t.Name {
				tasks[i] = t
				updated = true
			}
		}
		if !updated {
			tasks = append(tasks, t)
			casette.tasks[t.Vertex] = tasks
		}
	}

	for _, l := range status.Logs {
		sink := casette.vertexLogs(l.Vertex)
		_, err := sink.Write(l.Data)
		if err != nil {
			return fmt.Errorf("write logs: %w", err)
		}
	}

	return nil
}

func (casette *Casette) CompletedCount() int {
	casette.l.Lock()
	defer casette.l.Unlock()
	var completed int
	for _, v := range casette.vertexes {
		if v.Completed != nil {
			completed++
		}
	}
	return completed
}

func (casette *Casette) TotalCount() int {
	casette.l.Lock()
	defer casette.l.Unlock()
	return len(casette.vertexes)
}

func (casette *Casette) Close() error {
	casette.l.Lock()
	casette.done = true
	casette.l.Unlock()
	return nil
}

func (casette *Casette) SetWindowSize(w, h int) {
	casette.l.Lock()
	casette.width = w
	casette.height = h
	for _, l := range casette.logs {
		l.SetWidth(w)
	}
	casette.l.Unlock()
}

func (casette *Casette) Render(w io.Writer, u *UI) error {
	// NB: using its inner lock is a little gross
	casette.l.Lock()
	defer casette.l.Unlock()

	var runningAndFailed []*Vertex
	for _, dig := range casette.order {
		vtx := casette.vertexes[dig]

		if vtx.Completed == nil || vtx.Error != nil {
			runningAndFailed = append(runningAndFailed, vtx)
			continue
		}

		if err := u.RenderVertex(w, vtx); err != nil {
			return err
		}

		tasks := casette.tasks[vtx.Id]
		for _, t := range tasks {
			if err := u.RenderTask(w, t); err != nil {
				return err
			}
		}
	}

	for _, vtx := range runningAndFailed {
		if err := u.RenderVertex(w, vtx); err != nil {
			return err
		}

		tasks := casette.tasks[vtx.Id]
		for _, t := range tasks {
			if err := u.RenderTask(w, t); err != nil {
				return err
			}
		}

		if vtx.Started == nil {
			continue
		}

		term := casette.vertexLogs(vtx.Id)

		if vtx.Error != nil {
			term.SetHeight(term.UsedHeight())
		} else {
			term.SetHeight(casette.termHeight())
		}

		if err := u.RenderTerm(w, term); err != nil {
			return err
		}
	}

	return nil
}

func (casette *Casette) insert(vtx *Vertex) {
	inputs := map[string]struct{}{}
	for _, i := range vtx.Inputs {
		inputs[i] = struct{}{}
	}

	for i, dig := range casette.order {
		other := casette.vertexes[dig]

		if casette.isAncestor(vtx, other) || len(inputs) == 0 {
			inserted := make([]string, len(casette.order)+1)
			copy(inserted, casette.order[:i])
			inserted[i] = vtx.Id
			copy(inserted[i+1:], casette.order[i:])
			casette.order = inserted
			return
		}

		delete(inputs, dig)
	}

	casette.order = append(casette.order, vtx.Id)
}

func (casette *Casette) isAncestor(a, b *Vertex) bool {
	for _, dig := range b.Inputs {
		if dig == a.Id {
			return true
		}

		ancestor, found := casette.vertexes[dig]
		if !found {
			continue
		}

		if casette.isAncestor(a, ancestor) {
			return true
		}
	}

	return false
}

func (casette *Casette) termHeight() int {
	return casette.height / 4
}

func (casette *Casette) vertexLogs(vertex string) *ui.Vterm {
	term, found := casette.logs[vertex]
	if !found {
		term = ui.NewVterm(casette.width)
		casette.logs[vertex] = term
	}

	return term
}

package progrock

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/muesli/termenv"
	"github.com/vito/progrock/ui"
)

// Casette is a Writer that renders a UI to a terminal.
type Casette struct {
	order    []string
	groups   map[string]*Group
	vertexes map[string]*Vertex
	tasks    map[string][]*VertexTask
	logs     map[string]*ui.Vterm
	done     bool

	width, height int

	l sync.Mutex
}

var debug = ui.NewVterm(80)

func NewCasette() *Casette {
	return &Casette{
		groups:   make(map[string]*Group),
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

	for _, g := range status.Groups {
		casette.groups[g.Id] = g
	}

	for _, v := range status.Vertexes {
		existing, found := casette.vertexes[v.Id]
		if !found {
			casette.insert(v)
		} else if existing.Completed != nil && v.Cached {
			// don't clobber the "real" vertex with a cache
			// TODO: count cache hits?
		} else {
			casette.vertexes[v.Id] = v
		}
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
	debug.SetWidth(w)
	casette.l.Unlock()
}

type Groups []*Group

const (
	dot      = "█"
	emptyDot = "○"
	vBar     = "│"
	hBar     = "─"
	hdBar    = "╴"
	tBar     = "┼"
	dBar     = "┊" // ┃┊┆┇┋╎
	blCorner = "╰"
	tlCorner = "╭"
	trCorner = "╮"
	brCorner = "╯"
	vlBar    = "┤"
	vrBar    = "├"
	htBar    = "┴"
	hbBar    = "┬"
	lCaret   = "<"
	rCaret   = ">"
)

func (groups Groups) Shrink() Groups {
	for i := len(groups) - 1; i >= 0; i-- {
		if groups[i] == nil {
			groups = groups[:i]
		} else {
			break
		}
	}
	return groups
}

func (groups Groups) Add(w io.Writer, u *UI, allGroups map[string]*Group, group *Group) Groups {
	if len(groups) == 0 && group.Name == RootGroup {
		return []*Group{group}
	}

	parentIdx := -1
	for i, g := range groups {
		if g == nil {
			continue
		}
		if g.Id == group.Id {
			return groups
		}
		if g.Id == group.GetParent() {
			parentIdx = i
		}
	}

	if parentIdx == -1 {
		parent := allGroups[group.GetParent()]
		groups = groups.Add(w, u, allGroups, parent)
		return groups.Add(w, u, allGroups, group)
	}

	var added bool
	var addedIdx int
	for i, c := range groups {
		if c == nil {
			groups[i] = group
			added = true
			addedIdx = i
			break
		}
	}

	if !added {
		groups = append(groups, group)
		addedIdx = len(groups) - 1
	}

	groups = groups.Shrink()

	for i := range groups {
		if i == parentIdx && addedIdx > parentIdx {
			// line towards the right of the parent
			fmt.Fprint(w, groupColor(parentIdx, vrBar))
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if i == parentIdx && addedIdx < parentIdx {
			// line towards the left of the parent
			fmt.Fprint(w, groupColor(parentIdx, vlBar))
			fmt.Fprint(w, " ")
		} else if i == addedIdx && addedIdx > parentIdx {
			// line left from parent and down to added line
			fmt.Fprint(w, groupColor(addedIdx, trCorner))
			fmt.Fprint(w, " ")
		} else if i == addedIdx && addedIdx < parentIdx {
			// line right from parent and down to added line
			fmt.Fprint(w, groupColor(addedIdx, tlCorner))
			fmt.Fprint(w, " ")
		} else if parentIdx != -1 && addedIdx > parentIdx && i > parentIdx && i < addedIdx {
			// line between parent and added line
			fmt.Fprint(w, groupColor(i, tBar))
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if groups[i] != nil {
			// line out of the way; dim it
			fmt.Fprint(w, groupColor(i, dBar))
			fmt.Fprint(w, " ")
		} else {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)

	groups.GroupPrefix(w, u, group)
	fmt.Fprintln(w, termenv.String(group.Name).Bold())

	return groups
}

func (groups Groups) VertexPrefix(w io.Writer, u *UI, vtx *Vertex, ch string) {
	groups = groups.Shrink()

	var vtxIdx = -1
	for i, g := range groups {
		var symbol string
		if g == nil {
			symbol = " "
		} else if (vtx.Group != nil && *vtx.Group == g.Id) ||
			(vtx.Group == nil && g.Name == RootGroup) {
			symbol = ch
			vtxIdx = i
		} else if vtxIdx != -1 && i >= vtxIdx {
			// line between parent and added line
			symbol = vlBar
		} else {
			symbol = dBar
		}

		fmt.Fprint(w, groupColor(i, symbol))

		if vtxIdx != -1 && i >= vtxIdx && i < len(groups)-1 {
			fmt.Fprint(w, groupColor(vtxIdx, hdBar))
		} else {
			fmt.Fprint(w, " ")
		}
	}
}

func (groups Groups) GroupPrefix(w io.Writer, u *UI, group *Group) {
	groups = groups.Shrink()

	var vtxIdx = -1
	for i, g := range groups {
		var symbol string
		if g == nil {
			symbol = " "
		} else if group.Id == g.Id {
			symbol = "▼"
			vtxIdx = i
		} else {
			symbol = dBar
		}

		fmt.Fprint(w, groupColor(i, symbol))

		if vtxIdx != -1 && i >= vtxIdx && i < len(groups)-1 {
			fmt.Fprint(w, groupColor(vtxIdx, hBar))
		} else {
			fmt.Fprint(w, " ")
		}
	}
}

func (groups Groups) TermPrefix(w io.Writer, u *UI, vtx *Vertex) {
	groups = groups.Shrink()

	for i, g := range groups {
		var symbol string
		if g == nil {
			symbol = " "
		} else if (vtx.Group != nil && *vtx.Group == g.Id) ||
			(vtx.Group == nil && g.Name == RootGroup) {
			symbol = vBar
		} else {
			symbol = dBar
		}

		fmt.Fprint(w, groupColor(i, symbol))
		fmt.Fprint(w, " ")
	}
}

func groupColor(i int, str string) string {
	return termenv.String(str).Foreground(
		termenv.ANSIColor(i%8) + 4,
	).String()
}

func (groups Groups) Reap(w io.Writer, u *UI, allGroups map[string]*Group, active []*Vertex) Groups {
	reaped := map[int]bool{}
	for i, g := range groups {
		if g == nil {
			continue
		}

		var isActive bool
		for _, a := range active {
			if a.Group == nil {
				// TODO decide if this is required field
				continue
			}

			for vg := *a.Group; vg != ""; vg = allGroups[vg].GetParent() {
				if vg == g.Id {
					isActive = true
					break
				}
			}
		}

		if !isActive {
			reaped[i] = true
			groups[i] = nil
		}
	}

	if len(reaped) > 0 {
		for i, g := range groups {
			if g != nil {
				fmt.Fprint(w, groupColor(i, dBar))
				fmt.Fprint(w, " ")
			} else if reaped[i] {
				fmt.Fprint(w, groupColor(i, htBar))
				fmt.Fprint(w, " ")
			} else {
				fmt.Fprint(w, "  ")
			}
		}
		fmt.Fprintln(w)
	}

	return groups.Shrink()
}

func (casette *Casette) Render(w io.Writer, u *UI) error {
	casette.l.Lock()
	defer casette.l.Unlock()

	groups := Groups{}

	runningByGroup := map[string]int{}

	var runningAndFailed []*Vertex
	for i, dig := range casette.order {
		_ = i
		vtx := casette.vertexes[dig]

		if vtx.Internal {
			// skip internal vertices
			continue
		}

		var active []*Vertex
		for _, rest := range casette.order[i:] {
			active = append(active, casette.vertexes[rest])
		}
		active = append(active, runningAndFailed...)
		groups.Reap(w, u, casette.groups, active)

		group := casette.groups[*vtx.Group]
		if group == nil {
			fmt.Fprintln(debug, "group is nil:", *vtx.Group)
		} else {
			groups = groups.Add(w, u, casette.groups, group)
		}

		if vtx.Completed == nil {
			runningByGroup[*vtx.Group]++
		}

		if vtx.Completed == nil || vtx.Error != nil {
			runningAndFailed = append(runningAndFailed, vtx)
			continue
		}

		groups.VertexPrefix(w, u, vtx, dot)
		if err := u.RenderVertex(w, vtx); err != nil {
			return err
		}

		tasks := casette.tasks[vtx.Id]
		for _, t := range tasks {
			groups.VertexPrefix(w, u, vtx, vrBar)
			if err := u.RenderTask(w, t); err != nil {
				return err
			}
		}
	}

	for i, vtx := range runningAndFailed {
		groups.Reap(w, u, casette.groups, runningAndFailed[i:])

		groups.VertexPrefix(w, u, vtx, dot)
		if err := u.RenderVertex(w, vtx); err != nil {
			return err
		}

		tasks := casette.tasks[vtx.Id]
		for _, t := range tasks {
			groups.VertexPrefix(w, u, vtx, vrBar)
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

		buf := new(bytes.Buffer)
		groups.TermPrefix(buf, u, vtx)
		term.SetPrefix(buf.String())

		if err := u.RenderTerm(w, term); err != nil {
			return err
		}
	}

	debug.SetHeight(10)
	fmt.Fprint(w, debug.View())

	return nil
}

func (casette *Casette) insert(vtx *Vertex) {
	if vtx.Started == nil {
		// skip pending vertices; too complicated to deal with
		return
	}

	casette.vertexes[vtx.Id] = vtx

	for i, dig := range casette.order {
		other := casette.vertexes[dig]
		if other.Started.AsTime().After(vtx.Started.AsTime()) {
			inserted := make([]string, len(casette.order)+1)
			copy(inserted, casette.order[:i])
			inserted[i] = vtx.Id
			copy(inserted[i+1:], casette.order[i:])
			casette.order = inserted
			return
		}
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

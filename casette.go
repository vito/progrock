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

	showInternal bool

	debug *ui.Vterm

	l sync.Mutex
}

const (
	block    = "█"
	dot      = "●"
	emptyDot = "○"
	vBar     = "│"
	vbBar    = "┃"
	hBar     = "─"
	hdlBar   = "╴"
	hdrBar   = "╶"
	tBar     = "┼"
	dBar     = "┊" // ┊┆┇┋╎
	blCorner = "╰"
	tlCorner = "╭"
	trCorner = "╮"
	brCorner = "╯"
	vlBar    = "┤"
	vrBar    = "├"
	vlbBar   = "┫"
	vrbBar   = "┣"
	htBar    = "┴"
	htbBar   = "┻"
	hbBar    = "┬"
	lCaret   = "◀" //"<"
	rCaret   = "▶" //">"
	dCaret   = "▼"

	taskSymbol          = vrbBar
	inactiveGroupSymbol = vBar
)

func NewCasette() *Casette {
	return &Casette{
		groups:   make(map[string]*Group),
		vertexes: make(map[string]*Vertex),
		tasks:    make(map[string][]*VertexTask),
		logs:     make(map[string]*ui.Vterm),

		// sane defaults before size is received
		width:  80,
		height: 24,

		debug: ui.NewVterm(80),
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

func (casette *Casette) ShowInternal(show bool) {
	casette.l.Lock()
	defer casette.l.Unlock()
	casette.showInternal = show
}

func (casette *Casette) SetWindowSize(w, h int) {
	casette.l.Lock()
	casette.width = w
	casette.height = h
	for _, l := range casette.logs {
		l.SetWidth(w)
	}
	casette.debug.SetWidth(w)
	casette.l.Unlock()
}

var pulse ui.Frames

func init() {
	pulse = ui.Frames{}
	copy(pulse[:], ui.FadeFrames[:])
	lastFrame := (ui.FramesPerBeat / 4) * 3
	for i := lastFrame; i < len(pulse); i++ {
		pulse[i] = ui.FadeFrames[lastFrame]
	}
}

func (casette *Casette) Render(w io.Writer, u *UI) error {
	casette.l.Lock()
	defer casette.l.Unlock()

	groups := Groups{}

	runningByGroup := map[string]int{}

	var runningAndFailed []*Vertex
	for i, dig := range casette.order {
		vtx := casette.vertexes[dig]

		for _, g := range groups {
			if g != nil {
				g.Witness(vtx)
			}
		}

		if vtx.Internal && !casette.showInternal {
			// skip internal vertices
			continue
		}

		var active []*Vertex
		for _, rest := range casette.order[i:] {
			active = append(active, casette.vertexes[rest])
		}
		active = append(active, runningAndFailed...)

		groups = groups.Reap(w, u, casette.groups, active)

		for _, id := range vtx.Groups {
			group := casette.groups[id]
			if group == nil {
				fmt.Fprintln(casette.debug, "group is nil:", id)
			} else {
				groups = groups.AddGroup(w, u, casette.groups, group)
			}

			if vtx.Completed == nil {
				runningByGroup[id]++
			}
		}

		if vtx.Completed == nil || vtx.Error != nil {
			runningAndFailed = append(runningAndFailed, vtx)
			continue
		}

		groups.VertexPrefix(w, u, vtx, block)
		if err := u.RenderVertex(w, vtx); err != nil {
			return err
		}

		tasks := casette.tasks[vtx.Id]
		for _, t := range tasks {
			groups.TaskPrefix(w, u, vtx)
			if err := u.RenderTask(w, t); err != nil {
				return err
			}
		}

		activeExceptCurrent := active[1:]

		var haveInput []*Vertex
		var siblings int
		for _, a := range activeExceptCurrent {
			if a.IsSibling(vtx) {
				siblings++
				// NB: avoid dealing with siblings entirely; having lines go out and
				// back in to later vertexes in the same group seems more confusing
				// than actually helpful. maybe show this at a higher debug level?
				continue
			}

			if a.HasInput(vtx) && siblings > 1 {
				// avoid forking if the immediate next vertex has an input, i.e.
				// chained commands
				haveInput = append(haveInput, a)
			}
		}

		for i, g := range groups {
			if g != nil {
				if g.WitnessedAll() {
					groups[i] = nil
				}
			}
		}

		if len(haveInput) > 0 {
			groups = groups.AddVertex(w, u, casette.groups, vtx, haveInput)
		}
	}

	for i, vtx := range runningAndFailed {
		groups = groups.Reap(w, u, casette.groups, runningAndFailed[i:])

		symbol := block
		if vtx.Completed == nil {
			symbol, _, _ = u.Spinner.ViewFrame(pulse)
		}

		groups.VertexPrefix(w, u, vtx, symbol)
		if err := u.RenderVertex(w, vtx); err != nil {
			return err
		}

		tasks := casette.tasks[vtx.Id]
		for _, t := range tasks {
			groups.TaskPrefix(w, u, vtx)
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

	groups.Reap(w, u, casette.groups, nil)

	casette.debug.SetHeight(10)
	fmt.Fprint(w, casette.debug.View())

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

type Groups []progressGroup

type progressGroup interface {
	ID() string
	DirectlyContains(*Vertex) bool
	IsActiveVia(*Vertex, map[string]*Group) bool
	Created(*Vertex) bool
	Witness(*Vertex)
	WitnessedAll() bool
}

type progGroup struct {
	*Group
}

func (gg progGroup) ID() string {
	return gg.Id
}

func (gg progGroup) DirectlyContains(vtx *Vertex) bool {
	return vtx.IsInGroup(gg.Group) ||
		len(vtx.Groups) == 0 && gg.Name == RootGroup
}

func (gg progGroup) IsActiveVia(vtx *Vertex, allGroups map[string]*Group) bool {
	return vtx.IsInGroupOrParent(gg.Group, allGroups) ||
		len(vtx.Groups) == 0 && gg.Name == RootGroup
}

func (gg progGroup) Created(vtx *Vertex) bool {
	return false
}

func (vg progGroup) Witness(other *Vertex) {
}

func (vg progGroup) WitnessedAll() bool {
	return false
}

type vertexGroup struct {
	*Vertex
	outputs   []*Vertex
	witnessed map[string]bool
}

func (vg vertexGroup) ID() string {
	return vg.Id
}

func (vg vertexGroup) DirectlyContains(vtx *Vertex) bool {
	return false //vtx.HasInput(vg.Vertex) // TODO
}

func (vg vertexGroup) IsActiveVia(other *Vertex, allGroups map[string]*Group) bool {
	return other.HasInput(vg.Vertex) // TODO
}

func (vg vertexGroup) Created(other *Vertex) bool {
	return other.HasInput(vg.Vertex)
}

func (vg vertexGroup) Witness(other *Vertex) {
	for _, out := range vg.outputs {
		if out.Id == other.Id {
			vg.witnessed[other.Id] = true
		}
	}
}

func (vg vertexGroup) WitnessedAll() bool {
	return len(vg.witnessed) == len(vg.outputs)
}

func (groups Groups) shrink() Groups {
	for i := len(groups) - 1; i >= 0; i-- {
		if groups[i] == nil {
			groups = groups[:i]
		} else {
			break
		}
	}
	return groups
}

func (groups Groups) AddGroup(w io.Writer, u *UI, allGroups map[string]*Group, group *Group) Groups {
	pg := progGroup{group}

	if len(groups) == 0 && group.Name == RootGroup {
		return Groups{pg}
	}

	parentIdx := -1
	for i, g := range groups {
		if g == nil {
			continue
		}
		if g.ID() == group.Id {
			return groups
		}
		if g.ID() == group.GetParent() {
			parentIdx = i
		}
	}

	if parentIdx == -1 {
		parent := allGroups[group.GetParent()]
		groups = groups.AddGroup(w, u, allGroups, parent)
		return groups.AddGroup(w, u, allGroups, group)
	}

	var added bool
	var addedIdx int
	for i, c := range groups {
		if c == nil {
			groups[i] = pg
			added = true
			addedIdx = i
			break
		}
	}

	if !added {
		groups = append(groups, pg)
		addedIdx = len(groups) - 1
	}

	groups = groups.shrink()

	for i, g := range groups {
		if i == parentIdx && addedIdx > parentIdx {
			// line towards the right of the parent
			fmt.Fprint(w, groupColor(parentIdx, vrbBar))
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if i == parentIdx && addedIdx < parentIdx {
			// line towards the left of the parent
			fmt.Fprint(w, groupColor(parentIdx, vlbBar))
			fmt.Fprint(w, " ")
		} else if i == addedIdx && addedIdx > parentIdx {
			// line left from parent and down to added line
			fmt.Fprint(w, groupColor(addedIdx, trCorner))
			fmt.Fprint(w, " ")
		} else if i == addedIdx && addedIdx < parentIdx {
			// line up from added line and right to parent
			fmt.Fprint(w, groupColor(addedIdx, tlCorner))
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if parentIdx != -1 && addedIdx > parentIdx && i > parentIdx && i < addedIdx {
			// line between parent and added line
			if g != nil {
				fmt.Fprint(w, groupColor(i, tBar))
			} else {
				fmt.Fprint(w, groupColor(addedIdx, hBar))
			}
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if parentIdx != -1 && addedIdx < parentIdx && i < parentIdx && i > addedIdx {
			// line between parent and added line
			if g != nil {
				fmt.Fprint(w, groupColor(i, tBar))
			} else {
				fmt.Fprint(w, groupColor(addedIdx, hBar))
			}
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if groups[i] != nil {
			fmt.Fprint(w, groupColor(i, inactiveGroupSymbol))
			fmt.Fprint(w, " ")
		} else {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)

	groups.GroupPrefix(w, u, pg)
	fmt.Fprintln(w, termenv.String(group.Name).Bold())

	return groups
}

func (groups Groups) AddVertex(w io.Writer, u *UI, allGroups map[string]*Group, vtx *Vertex, outputs []*Vertex) Groups {
	vg := vertexGroup{
		vtx,
		outputs,
		map[string]bool{},
	}

	var added bool
	var addedIdx int
	for i, c := range groups {
		if c == nil {
			groups[i] = vg
			added = true
			addedIdx = i
			break
		}
	}

	if !added {
		groups = append(groups, vg)
		addedIdx = len(groups) - 1
	}

	groups = groups.shrink()

	var parentIdx = -1

	for i, g := range groups {
		hasVtx := g != nil && g.DirectlyContains(vtx)
		if hasVtx {
			if parentIdx == -1 {
				parentIdx = i
				// } else {
				// 	panic("impossible?")
			}
		}
	}

	for i, g := range groups {
		if i == parentIdx && addedIdx > parentIdx {
			// line towards the right of the parent
			fmt.Fprint(w, groupColor(parentIdx, vrbBar))
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if i == parentIdx && addedIdx < parentIdx {
			// line towards the left of the parent
			fmt.Fprint(w, groupColor(parentIdx, vlbBar))
			fmt.Fprint(w, " ")
		} else if parentIdx != -1 && i == addedIdx && addedIdx > parentIdx {
			// line left from parent and down to added line
			fmt.Fprint(w, groupColor(addedIdx, trCorner))
			fmt.Fprint(w, " ")
		} else if parentIdx != -1 && i == addedIdx && addedIdx < parentIdx {
			// line up from added line and right to parent
			fmt.Fprint(w, groupColor(addedIdx, tlCorner))
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if parentIdx != -1 && addedIdx > parentIdx && i > parentIdx && i < addedIdx {
			// line between parent and added line
			if g != nil {
				fmt.Fprint(w, groupColor(i, tBar))
			} else {
				fmt.Fprint(w, groupColor(addedIdx, hBar))
			}
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if parentIdx != -1 && addedIdx < parentIdx && i < parentIdx && i > addedIdx {
			// line between parent and added line
			if g != nil {
				fmt.Fprint(w, groupColor(i, tBar))
			} else {
				fmt.Fprint(w, groupColor(addedIdx, hBar))
			}
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if parentIdx == -1 && i == addedIdx {
			fmt.Fprint(w, groupColor(addedIdx, emptyDot)) // TODO pointless?
			fmt.Fprint(w, groupColor(addedIdx, hBar))
		} else if groups[i] != nil {
			fmt.Fprint(w, groupColor(i, inactiveGroupSymbol))
			fmt.Fprint(w, " ")
		} else {
			fmt.Fprint(w, "  ")
		}
	}

	fmt.Fprintln(w, termenv.String(vtx.Name).Foreground(termenv.ANSIBrightBlack))

	return groups
}

func (groups Groups) VertexPrefix(w io.Writer, u *UI, vtx *Vertex, ch string) {
	var firstParentIdx = -1
	var lastParentIdx = -1
	var vtxIdx = -1
	for i, g := range groups {
		if g == nil {
			continue
		}

		if g.Created(vtx) {
			if firstParentIdx == -1 {
				firstParentIdx = i
			}
			lastParentIdx = i
		}

		if g.DirectlyContains(vtx) {
			vtxIdx = i
		}
	}

	if vtxIdx == -1 {
		panic("impossible? vertex has no group")
	}

	for i, g := range groups {
		var symbol string
		if g == nil {
			if firstParentIdx != -1 && vtxIdx > firstParentIdx && i >= firstParentIdx && i < vtxIdx {
				fmt.Fprint(w, groupColor(firstParentIdx, hBar))
			} else if firstParentIdx != -1 && vtxIdx < firstParentIdx && i >= vtxIdx && i < lastParentIdx {
				fmt.Fprint(w, groupColor(firstParentIdx, hBar))
			} else {
				fmt.Fprint(w, " ")
			}
		} else {
			if g.DirectlyContains(vtx) {
				symbol = block
				vtxIdx = i
			} else if g.Created(vtx) && i > vtxIdx {
				if g.WitnessedAll() {
					if i == lastParentIdx {
						symbol = brCorner
					} else {
						symbol = htBar
					}
				} else {
					symbol = vlBar
				}
			} else if g.Created(vtx) {
				if g.WitnessedAll() {
					symbol = blCorner
				} else {
					symbol = vrBar
				}
			} else if firstParentIdx != -1 && i >= firstParentIdx && i < vtxIdx {
				symbol = tBar
			} else if firstParentIdx != -1 && i >= vtxIdx && i < lastParentIdx {
				symbol = tBar
			} else {
				symbol = inactiveGroupSymbol
			}

			// respect color of the group
			fmt.Fprint(w, groupColor(i, symbol))
		}

		if firstParentIdx != -1 && vtxIdx > firstParentIdx && i >= firstParentIdx && i < vtxIdx {
			if i+1 == vtxIdx {
				fmt.Fprint(w, groupColor(firstParentIdx, rCaret))
			} else {
				fmt.Fprint(w, groupColor(firstParentIdx, hBar))
			}
		} else if firstParentIdx != -1 && vtxIdx < firstParentIdx && i >= vtxIdx && i < lastParentIdx {
			if i == vtxIdx {
				fmt.Fprint(w, groupColor(firstParentIdx, lCaret))
			} else {
				fmt.Fprint(w, groupColor(firstParentIdx, hBar))
			}
		} else {
			fmt.Fprint(w, " ")
		}
	}
}

func (groups Groups) TaskPrefix(w io.Writer, u *UI, vtx *Vertex) {
	for i, g := range groups {
		if g == nil {
			fmt.Fprint(w, " ")
		} else if g.DirectlyContains(vtx) {
			fmt.Fprint(w, groupColor(i, taskSymbol))
		} else {
			fmt.Fprint(w, groupColor(i, inactiveGroupSymbol))
		}
		fmt.Fprint(w, " ")
	}
}

func (groups Groups) GroupPrefix(w io.Writer, u *UI, group progressGroup) {
	for i, g := range groups {
		if g == nil {
			fmt.Fprint(w, " ")
		} else if group.ID() == g.ID() {
			fmt.Fprint(w, groupColor(i, dCaret))
		} else {
			fmt.Fprint(w, groupColor(i, inactiveGroupSymbol))
		}
		fmt.Fprint(w, " ")
	}
}

func (groups Groups) TermPrefix(w io.Writer, u *UI, vtx *Vertex) {
	for i, g := range groups {
		if g == nil {
			fmt.Fprint(w, " ")
		} else if g.DirectlyContains(vtx) {
			fmt.Fprint(w, groupColor(i, vbBar))
		} else {
			fmt.Fprint(w, groupColor(i, inactiveGroupSymbol))
		}
		fmt.Fprint(w, " ")
	}
}

func groupColor(i int, str string) string {
	return termenv.String(str).Foreground(
		termenv.ANSIColor((i+1)%7) + 1,
	).String()
}

func (groups Groups) Reap(w io.Writer, u *UI, allGroups map[string]*Group, active []*Vertex) Groups {
	reaped := map[int]bool{}
	for i, g := range groups {
		if g == nil {
			continue
		}

		// if allGroups[g.Id].Completed == nil {
		// 	// let groups linger as long as they're not completed, otherwise future
		// 	// child groups will be orphaned
		// 	continue
		// }

		var isActive bool
		for _, vtx := range active {
			if g.IsActiveVia(vtx, allGroups) {
				isActive = true
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
				fmt.Fprint(w, groupColor(i, inactiveGroupSymbol))
				fmt.Fprint(w, " ")
			} else if reaped[i] {
				fmt.Fprint(w, groupColor(i, htbBar))
				fmt.Fprint(w, " ")
			} else {
				fmt.Fprint(w, "  ")
			}
		}
		fmt.Fprintln(w)
	}

	return groups.shrink()
}

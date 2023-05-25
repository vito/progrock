package progrock

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/muesli/termenv"
	"github.com/vito/progrock/ui"
)

// Tape is a Writer that collects all progress output for displaying in a
// terminal UI.
type Tape struct {
	order          []string
	vertexes       map[string]*Vertex
	groups         map[string]*Group
	group2vertexes map[string]map[string]struct{}
	vertex2groups  map[string]map[string]struct{}
	tasks          map[string][]*VertexTask
	logs           map[string]*ui.Vterm
	done           bool

	width, height int

	// show edges between vertexes in the same group
	verboseEdges bool

	// show internal vertexes
	showInternal bool

	// show output even for completed vertexes
	showAllOutput bool

	debug *ui.Vterm

	l sync.Mutex
}

const (
	block       = "█"
	dot         = "●"
	emptyDot    = "○"
	vBar        = "│"
	vbBar       = "┃"
	hBar        = "─"
	hdlBar      = "╴"
	hdrBar      = "╶"
	tBar        = "┼"
	dBar        = "┊" // ┊┆┇┋╎
	blCorner    = "╰"
	tlCorner    = "╭"
	trCorner    = "╮"
	brCorner    = "╯"
	vlBar       = "┤"
	vrBar       = "├"
	vlbBar      = "┫"
	vrbBar      = "┣"
	htBar       = "┴"
	htbBar      = "┻"
	hbBar       = "┬"
	lCaret      = "◀" //"<"
	rCaret      = "▶" //">"
	dCaret      = "▼"
	dEmptyCaret = "▽"

	taskSymbol          = vrbBar
	inactiveGroupSymbol = vBar
)

// NewTape returns a new Tape.
func NewTape() *Tape {
	return &Tape{
		vertexes:       make(map[string]*Vertex),
		groups:         make(map[string]*Group),
		group2vertexes: make(map[string]map[string]struct{}),
		vertex2groups:  make(map[string]map[string]struct{}),
		tasks:          make(map[string][]*VertexTask),
		logs:           make(map[string]*ui.Vterm),

		// sane defaults before size is received
		width:  80,
		height: 24,

		debug: ui.NewVterm(80),
	}
}

var _ Writer = &Tape{}

// WriteStatus implements Writer by collecting vertex and task updates and
// writing vertex logs to internal virtual terminals.
func (tape *Tape) WriteStatus(status *StatusUpdate) error {
	tape.l.Lock()
	defer tape.l.Unlock()

	for _, g := range status.Groups {
		tape.groups[g.Id] = g
	}

	for _, v := range status.Vertexes {
		existing, found := tape.vertexes[v.Id]
		if !found {
			tape.insert(v.Id, v)
		} else if existing.Completed != nil && v.Cached {
			// don't clobber the "real" vertex with a cache
			// TODO: count cache hits?
		} else {
			tape.vertexes[v.Id] = v
		}
	}

	for _, t := range status.Tasks {
		tasks := tape.tasks[t.Vertex]
		var updated bool
		for i, task := range tasks {
			if task.Name == t.Name {
				tasks[i] = t
				updated = true
			}
		}
		if !updated {
			tasks = append(tasks, t)
			tape.tasks[t.Vertex] = tasks
		}
	}

	for _, l := range status.Logs {
		sink := tape.vertexLogs(l.Vertex)
		_, err := sink.Write(l.Data)
		if err != nil {
			return fmt.Errorf("write logs: %w", err)
		}
	}

	for _, ms := range status.Memberships {
		members, found := tape.group2vertexes[ms.Group]
		if !found {
			members = make(map[string]struct{})
			tape.group2vertexes[ms.Group] = members
		}

		for _, vtxId := range ms.Vertexes {
			groups, found := tape.vertex2groups[vtxId]
			if !found {
				groups = make(map[string]struct{})
				tape.vertex2groups[vtxId] = groups
			}

			if len(groups) == 0 {
				// only place vertexes in their first group
				members[vtxId] = struct{}{}
				groups[ms.Group] = struct{}{}
			}
		}
	}

	return nil
}

// CompletedCount returns the number of completed vertexes.
func (tape *Tape) CompletedCount() int {
	tape.l.Lock()
	defer tape.l.Unlock()
	var completed int
	for _, v := range tape.vertexes {
		if v.Completed != nil {
			completed++
		}
	}
	return completed
}

// TotalCount returns the total number of vertexes.
func (tape *Tape) TotalCount() int {
	tape.l.Lock()
	defer tape.l.Unlock()
	return len(tape.vertexes)
}

// Close marks the Tape as done, which tells it to display all vertex output
// for the final render.
func (tape *Tape) Close() error {
	tape.l.Lock()
	tape.done = true
	tape.l.Unlock()
	return nil
}

// VerboseEdges sets whether to display edges between vertexes in the same
// group.
func (tape *Tape) VerboseEdges(verbose bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.verboseEdges = verbose
}

// ShowInternal sets whether to show internal vertexes in the output.
func (tape *Tape) ShowInternal(show bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.showInternal = show
}

// ShowAllOutput sets whether to show output even for successful vertexes.
func (tape *Tape) ShowAllOutput(show bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.showAllOutput = show
}

// SetWindowSize sets the size of the terminal UI, which influences the
// dimensions for vertex logs, progress bars, etc.
func (tape *Tape) SetWindowSize(w, h int) {
	tape.l.Lock()
	tape.width = w
	tape.height = h
	for _, l := range tape.logs {
		l.SetWidth(w)
	}
	tape.debug.SetWidth(w)
	tape.l.Unlock()
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

func (tape *Tape) Render(w io.Writer, u *UI) error {
	tape.l.Lock()
	defer tape.l.Unlock()

	b := &bouncer{
		groups:         tape.groups,
		group2vertexes: tape.group2vertexes,
		vertex2groups:  tape.vertex2groups,
	}

	var completed []*Vertex
	var runningAndFailed []*Vertex

	for _, dig := range tape.order {
		vtx := tape.vertexes[dig]

		if vtx.Completed == nil || vtx.Error != nil {
			runningAndFailed = append(runningAndFailed, vtx)
		} else {
			completed = append(completed, vtx)
		}
	}

	order := append(completed, runningAndFailed...)
	groups := progressGroups{}

	for i, vtx := range order {
		for _, g := range groups {
			if g != nil {
				g.Witness(vtx)
			}
		}

		if vtx.Internal && !tape.showInternal {
			// skip internal vertices
			continue
		}

		groups = groups.Reap(w, u, order[i:])

		for _, g := range b.Groups(vtx) {
			groups = groups.AddGroup(w, u, b, g, tape.debug)
		}

		symbol := block
		if vtx.Completed == nil {
			symbol, _, _ = u.Spinner.ViewFrame(pulse)
		}

		groups.VertexPrefix(w, u, vtx, symbol, tape.debug)
		if err := u.RenderVertex(w, vtx); err != nil {
			return err
		}

		tasks := tape.tasks[vtx.Id]
		for _, t := range tasks {
			groups.TaskPrefix(w, u, vtx)
			if err := u.RenderTask(w, t); err != nil {
				return err
			}
		}

		activeExceptCurrent := order[1:]

		var haveInput []*Vertex
		for _, a := range activeExceptCurrent {
			if a.HasInput(vtx) && (!b.IsSibling(a, vtx) || tape.verboseEdges) {
				// avoid forking for vertexes in the same group; often more noisy than helpful
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
			groups = groups.AddVertex(w, u, tape.groups, vtx, haveInput)
		}

		if tape.done || tape.showAllOutput || vtx.Completed == nil || vtx.Error != nil {
			term := tape.vertexLogs(vtx.Id)

			if vtx.Error != nil {
				term.SetHeight(term.UsedHeight())
			} else {
				term.SetHeight(tape.termHeight())
			}

			buf := new(bytes.Buffer)
			groups.TermPrefix(buf, u, vtx)
			term.SetPrefix(buf.String())

			if tape.done {
				term.SetHeight(term.UsedHeight())
			}

			if err := u.RenderTerm(w, term); err != nil {
				return err
			}
		}
	}

	groups.Reap(w, u, nil)

	tape.debug.SetHeight(10)
	fmt.Fprint(w, tape.debug.View())

	return nil
}

func (tape *Tape) EachVertex(f func(*Vertex, *ui.Vterm)) {
	tape.l.Lock()
	defer tape.l.Unlock()
	for _, vtx := range tape.vertexes {
		f(vtx, tape.vertexLogs(vtx.Id))
	}
}

func (tape *Tape) insert(id string, vtx *Vertex) {
	if vtx.Started == nil {
		// skip pending vertices; too complicated to deal with
		return
	}

	tape.vertexes[id] = vtx

	for i, dig := range tape.order {
		other := tape.vertexes[dig]
		if other.Started.AsTime().After(vtx.Started.AsTime()) {
			inserted := make([]string, len(tape.order)+1)
			copy(inserted, tape.order[:i])
			inserted[i] = id
			copy(inserted[i+1:], tape.order[i:])
			tape.order = inserted
			return
		}
	}

	tape.order = append(tape.order, id)
}

func (tape *Tape) termHeight() int {
	return tape.height / 4
}

func (tape *Tape) vertexLogs(vertex string) *ui.Vterm {
	term, found := tape.logs[vertex]
	if !found {
		term = ui.NewVterm(tape.width)
		tape.logs[vertex] = term
	}

	return term
}

type bouncer struct {
	groups         map[string]*Group
	group2vertexes map[string]map[string]struct{}
	vertex2groups  map[string]map[string]struct{}
}

func (b *bouncer) Groups(vtx *Vertex) []*Group {
	groups := make([]*Group, 0, len(b.vertex2groups[vtx.Id]))
	for id := range b.vertex2groups[vtx.Id] {
		group, found := b.groups[id]
		if found {
			groups = append(groups, group)
		}
	}

	// TODO this is in the hot path, would be better to maintain order instead of
	// sorting
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Started.AsTime().Before(groups[j].Started.AsTime())
	})

	return groups
}

func (b *bouncer) Parent(group *Group) (*Group, bool) {
	if group.Parent == nil {
		return nil, false
	}

	parent, found := b.groups[group.GetParent()]
	return parent, found
}

func (b *bouncer) IsInGroup(vtx *Vertex, group *Group) bool {
	groups := b.vertex2groups[vtx.Id]
	if len(groups) == 0 {
		// vertex is not in any group; default it to the root group
		return group.Name == RootGroup
	}

	_, found := groups[group.Id]
	return found
}

func (b *bouncer) IsSubGroup(child, needle *Group) bool {
	for parent, found := b.Parent(child); found; parent, found = b.Parent(parent) {
		if parent.Id == needle.Id {
			return true
		}
	}
	return false
}

func (b *bouncer) IsInGroupOrChild(vtx *Vertex, group *Group) bool {
	if b.IsInGroup(vtx, group) {
		return true
	}

	for groupId := range b.vertex2groups[vtx.Id] {
		g, found := b.groups[groupId]
		if !found {
			continue
		}

		if b.IsSubGroup(g, group) {
			return true
		}
	}

	return false
}

func (b *bouncer) IsSibling(x, y *Vertex) bool {
	groupsA, found := b.vertex2groups[x.Id]
	if !found {
		return false
	}

	groupsB, found := b.vertex2groups[y.Id]
	if !found {
		return false
	}

	for id := range groupsA {
		if _, found := groupsB[id]; found {
			return true
		}
	}

	return false
}

type progressGroups []progressGroup

type progressGroup interface {
	ID() string
	DirectlyContains(*Vertex) bool
	IsActiveVia(*Vertex) bool
	Created(*Vertex) bool
	Witness(*Vertex)
	WitnessedAll() bool
}

type progGroup struct {
	*Group

	bouncer *bouncer
}

func (gg progGroup) ID() string {
	return gg.Id
}

func (gg progGroup) DirectlyContains(vtx *Vertex) bool {
	return gg.bouncer.IsInGroup(vtx, gg.Group)
}

func (gg progGroup) IsActiveVia(vtx *Vertex) bool {
	return gg.bouncer.IsInGroupOrChild(vtx, gg.Group)
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
	return false
}

func (vg vertexGroup) IsActiveVia(other *Vertex) bool {
	return other.HasInput(vg.Vertex)
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

// AddVertex adds a group to the set of groups. It also renders the new groups.
func (groups progressGroups) AddGroup(w io.Writer, u *UI, b *bouncer, group *Group, debug io.Writer) progressGroups {
	pg := progGroup{group, b}

	if len(groups) == 0 && group.Name == RootGroup {
		return progressGroups{pg}
	}

	parentIdx := -1
	for i, g := range groups {
		if g == nil {
			continue
		}
		if g.ID() == group.Id {
			// group already present
			return groups
		}
		if g.ID() == group.GetParent() {
			parentIdx = i
		}
	}

	if parentIdx == -1 && group.Parent != nil {
		parent, found := b.Parent(group)
		if !found {
			fmt.Fprintln(debug, "group", group.Id, "has unknown parent:", group.GetParent())
			return groups
		}

		return groups.AddGroup(w, u, b, parent, debug).
			AddGroup(w, u, b, group, debug)
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
	groups.GroupName(w, u, group)

	return groups
}

// AddVertex adds a vertex-group to the set of groups, which is used to connect
// vertex inputs and outputs. It also renders the new groups.
func (groups progressGroups) AddVertex(w io.Writer, u *UI, allGroups map[string]*Group, vtx *Vertex, outputs []*Vertex) progressGroups {
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

// VertexPrefix prints the prefix for a vertex.
func (groups progressGroups) VertexPrefix(w io.Writer, u *UI, vtx *Vertex, selfSymbol string, debug io.Writer) {
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
		fmt.Fprintln(debug, "vertex has no containing group?", vtx)
	}

	for i, g := range groups {
		var symbol string
		if g == nil {
			if firstParentIdx != -1 && i < vtxIdx && i >= firstParentIdx {
				fmt.Fprint(w, groupColor(firstParentIdx, hBar))
			} else if firstParentIdx != -1 && i >= vtxIdx && i < lastParentIdx {
				fmt.Fprint(w, groupColor(lastParentIdx, hBar))
			} else {
				fmt.Fprint(w, " ")
			}
		} else {
			if g.DirectlyContains(vtx) {
				symbol = selfSymbol
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
					if i == firstParentIdx {
						symbol = blCorner
					} else {
						symbol = htBar
					}
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
		} else if firstParentIdx != -1 && i >= vtxIdx && i < lastParentIdx {
			if i == vtxIdx {
				fmt.Fprint(w, groupColor(lastParentIdx, lCaret))
			} else {
				fmt.Fprint(w, groupColor(lastParentIdx, hBar))
			}
		} else {
			fmt.Fprint(w, " ")
		}
	}
}

// TaskPrefix prints the prefix for a vertex's task.
func (groups progressGroups) TaskPrefix(w io.Writer, u *UI, vtx *Vertex) {
	groups.printPrefix(w, func(g progressGroup, vtx *Vertex) string {
		if g.DirectlyContains(vtx) {
			return taskSymbol
		}
		return inactiveGroupSymbol
	}, vtx)
}

// GroupName prints the prefix and name for newly added group.
func (groups progressGroups) GroupName(w io.Writer, u *UI, group *Group) {
	groups.printPrefix(w, func(g progressGroup, _ *Vertex) string {
		if g.ID() == group.Id {
			if group.Weak {
				return dEmptyCaret
			} else {
				return dCaret
			}
		}
		return inactiveGroupSymbol
	}, nil)
	if group.Weak {
		fmt.Fprintln(w, group.Name)
	} else {
		fmt.Fprintln(w, termenv.String(group.Name).Bold())
	}
}

// TermPrefix prints the prefix for a vertex's terminal output.
func (groups progressGroups) TermPrefix(w io.Writer, u *UI, vtx *Vertex) {
	groups.printPrefix(w, func(g progressGroup, vtx *Vertex) string {
		if g.DirectlyContains(vtx) {
			return vbBar
		}
		return inactiveGroupSymbol
	}, vtx)
}

// Reap removes groups that are no longer active.
func (groups progressGroups) Reap(w io.Writer, u *UI, active []*Vertex) progressGroups {
	reaped := map[int]bool{}
	for i, g := range groups {
		if g == nil {
			continue
		}

		var isActive bool
		for _, vtx := range active {
			if g.IsActiveVia(vtx) {
				isActive = true
				break
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

// shrink removes nil groups from the end of the slice.
func (groups progressGroups) shrink() progressGroups {
	for i := len(groups) - 1; i >= 0; i-- {
		if groups[i] == nil {
			groups = groups[:i]
		} else {
			break
		}
	}
	return groups
}

func (groups progressGroups) printPrefix(w io.Writer, sym func(progressGroup, *Vertex) string, vtx *Vertex) {
	for i, g := range groups {
		if g == nil {
			fmt.Fprint(w, " ")
		} else {
			fmt.Fprint(w, groupColor(i, sym(g, vtx)))
		}
		fmt.Fprint(w, " ")
	}
}

func groupColor(i int, str string) string {
	return termenv.String(str).Foreground(
		termenv.ANSIColor((i+1)%7) + 1,
	).String()
}

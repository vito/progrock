package progrock

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/muesli/termenv"
	"github.com/vito/midterm"
	"github.com/vito/progrock/ui"
)

type Frontend interface {
	Writer
	Render(*Tape, io.Writer, *UI) error
}

// Tape is a Writer that collects all progress output for displaying in a
// terminal UI.
//
// The Tape has a configurable Frontend which is called with a lock held such
// that it is safe to access all exposed fields of the Tape. Otherwise it is
// not generally safe to access its exported fields. This is a conscious
// trade-off to avoid too much lock contention in the fast path.
type Tape struct {
	// a configurable frontend
	frontend Frontend

	// ChronologicalVertexIDs lists the vertex IDs in the order that they were
	// most recently active, with the most recently active last. This is an order
	// suitable for displaying to the user in the terminal, where they will
	// naturally be watching the bottom of the screen.
	ChronologicalVertexIDs []string

	// zoomed vertices
	zoomed   map[string]*zoomState
	lastZoom *zoomState
	zoomHook func(*zoomState)

	// raw vertex/group state from the event stream
	Vertices map[string]*Vertex
	Groups   map[string]*Group

	// vertex <-> group mappings
	GroupVertices map[string]map[string]struct{}
	VertexGroups  map[string]map[string]struct{}

	// group children
	GroupChildren map[string]map[string]struct{}

	// vertex state
	VertexTasks map[string][]*VertexTask
	vertexLogs  map[string]*ui.Vterm

	// whether and when the tape has been IsClosed
	IsClosed bool
	ClosedAt time.Time

	// the time that tape was created and closed
	CreatedAt time.Time

	// color profile loaded from the env when the tape was constructed
	ColorProfile termenv.Profile

	// screen Width and Height
	Width, Height int

	// visible region for active vterms
	ReasonableTermHeight int

	// UI config
	IsFocused                           bool // only show 'focused' vertex output, condensing the rest
	ShowInternalVertices                bool // show internal vertexes
	RevealErroredVertices               bool // show errored vertexes no matter what
	ShowCompletedLogs                   bool // show output even for completed vertexes
	ShowEdgesBetweenVerticesInSameGroup bool // show edges between vertexes in the same group

	// output from messages and internal debugging
	globalLogs  *ui.Vterm
	globalLogsW *termenv.Output

	// minimum message level to display to the user
	messageLevel MessageLevel

	l sync.Mutex
}

type zoomState struct {
	Input  io.Writer
	Output *midterm.Terminal
}

// NewTape returns a new Tape.
func NewTape() *Tape {
	colorProfile := ui.ColorProfile()

	logs := ui.NewVterm()

	return &Tape{
		Vertices:      make(map[string]*Vertex),
		Groups:        make(map[string]*Group),
		GroupVertices: make(map[string]map[string]struct{}),
		VertexGroups:  make(map[string]map[string]struct{}),
		GroupChildren: make(map[string]map[string]struct{}),
		VertexTasks:   make(map[string][]*VertexTask),
		vertexLogs:    make(map[string]*ui.Vterm),
		zoomed:        make(map[string]*zoomState),

		// default to unbounded screen size so we don't arbitrarily wrap log output
		// when no size is given
		Width:  -1,
		Height: -1,

		ColorProfile: colorProfile,

		// sane default before window size is known
		ReasonableTermHeight: 10,

		globalLogs:  logs,
		globalLogsW: ui.NewOutput(logs, termenv.WithProfile(colorProfile)),

		messageLevel: MessageLevel_WARNING,

		CreatedAt: time.Now(),
	}
}

var _ Writer = &Tape{}

// WriteStatus implements Writer by collecting vertex and task updates and
// writing vertex logs to internal virtual terminals.
func (tape *Tape) WriteStatus(status *StatusUpdate) error {
	tape.l.Lock()
	defer tape.l.Unlock()

	for _, g := range status.Groups {
		tape.Groups[g.Id] = g
		if g.Parent != nil {
			parents, found := tape.GroupChildren[g.GetParent()]
			if !found {
				parents = make(map[string]struct{})
				tape.GroupChildren[g.GetParent()] = parents
			}
			parents[g.Id] = struct{}{}
		}
	}

	for _, v := range status.Vertexes {
		existing, found := tape.Vertices[v.Id]
		if !found {
			tape.insert(v.Id, v)
		} else if existing.Completed != nil && v.Cached {
			// don't clobber the "real" vertex with a cache
			// TODO: count cache hits?
		} else {
			tape.Vertices[v.Id] = v
		}

		_, isZoomed := tape.zoomed[v.Id]
		if v.Zoomed && !isZoomed {
			tape.initZoom(v)
		} else if isZoomed {
			tape.releaseZoom(v)
		}
	}

	for _, t := range status.Tasks {
		tape.recordTask(t)
	}

	for _, l := range status.Logs {
		var w io.Writer
		if t, found := tape.zoomed[l.Vertex]; found {
			w = t.Output
		} else {
			w = tape.VertexLogs(l.Vertex)
		}
		_, err := w.Write(l.Data)
		if err != nil {
			return fmt.Errorf("write logs: %w", err)
		}
	}

	for _, ms := range status.Memberships {
		members, found := tape.GroupVertices[ms.Group]
		if !found {
			members = make(map[string]struct{})
			tape.GroupVertices[ms.Group] = members
		}

		for _, vtxId := range ms.Vertexes {
			groups, found := tape.VertexGroups[vtxId]
			if !found {
				groups = make(map[string]struct{})
				tape.VertexGroups[vtxId] = groups
			}

			if len(groups) == 0 {
				// only place vertexes in their first group
				members[vtxId] = struct{}{}
				groups[ms.Group] = struct{}{}
			}
		}
	}

	for _, msg := range status.Messages {
		tape.log(msg)
	}

	if tape.frontend != nil {
		if err := tape.frontend.WriteStatus(status); err != nil {
			return err
		}
	}

	return nil
}

func (tape *Tape) initZoom(v *Vertex) {
	var vt *midterm.Terminal
	if tape.Height == -1 || tape.Width == -1 {
		vt = midterm.NewAutoResizingTerminal()
	} else {
		vt = midterm.NewTerminal(tape.Height, tape.Width)
	}
	w := setupTerm(v.Id, vt)
	tape.zoomed[v.Id] = &zoomState{
		Output: vt,
		Input:  w,
	}
}

func (tape *Tape) releaseZoom(vtx *Vertex) {
	delete(tape.zoomed, vtx.Id)
}

func (tape *Tape) recordTask(t *VertexTask) {
	tasks := tape.VertexTasks[t.Vertex]
	var updated bool
	for i, task := range tasks {
		if task.Name == t.Name {
			tasks[i] = t
			updated = true
		}
	}
	if !updated {
		tasks = append(tasks, t)
		tape.VertexTasks[t.Vertex] = tasks
	}
}

func (tape *Tape) log(msg *Message) {
	if msg.Level < tape.messageLevel {
		return
	}

	WriteMessage(tape.globalLogsW, msg)
}

func WriteMessage(out *termenv.Output, msg *Message) error {
	var prefix termenv.Style
	switch msg.Level {
	case MessageLevel_DEBUG:
		prefix = out.String("DEBUG:").Foreground(termenv.ANSIBlue).Bold()
	case MessageLevel_WARNING:
		prefix = out.String("WARNING:").Foreground(termenv.ANSIYellow).Bold()
	case MessageLevel_ERROR:
		prefix = out.String("ERROR:").Foreground(termenv.ANSIRed).Bold()
	}

	str := msg.Message
	for _, l := range msg.Labels {
		str += " "
		str += out.String(fmt.Sprintf("%s=%q", l.Name, l.Value)).
			Foreground(termenv.ANSIBrightBlack).
			String()
	}

	_, err := fmt.Fprintln(out, prefix, str)
	return err
}

func (tape *Tape) AllVertices() []*Vertex {
	tape.l.Lock()
	defer tape.l.Unlock()

	var vertices []*Vertex
	for _, vid := range tape.ChronologicalVertexIDs {
		vtx, found := tape.Vertices[vid]
		if !found {
			// should be impossible
			continue
		}

		vertices = append(vertices, vtx)
	}

	return vertices
}

func (tape *Tape) RunningVertex() *Vertex {
	tape.l.Lock()
	defer tape.l.Unlock()

	for i := len(tape.ChronologicalVertexIDs) - 1; i >= 0; i-- {
		vid := tape.ChronologicalVertexIDs[i]
		vtx := tape.Vertices[vid]
		if vtx.Started != nil && vtx.Completed == nil {
			return vtx
		}
	}
	return nil
}

type VertexActivity struct {
	LastLine        string
	TasksCompleted  int64
	TasksTotal      int64
	TaskBarsCurrent int64
	TaskBarsTotal   int64
}

func (tape *Tape) Activity(vtx *Vertex) VertexActivity {
	tape.l.Lock()
	defer tape.l.Unlock()

	var activity VertexActivity

	term := tape.vertexLogs[vtx.Id]
	if term != nil {
		activity.LastLine = term.LastLine()
	}

	tasks := tape.VertexTasks[vtx.Id]
	for _, task := range tasks {
		activity.TasksTotal++

		if task.Completed != nil {
			activity.TasksCompleted++
		} else if task.Started != nil && activity.LastLine == "" {
			activity.LastLine = task.GetName()
		}

		activity.TaskBarsTotal += task.GetTotal()
		activity.TaskBarsCurrent += task.GetCurrent()
	}

	return activity
}

// CompletedCount returns the number of completed vertexes.
// TODO: cache this
func (tape *Tape) CompletedCount() int {
	tape.l.Lock()
	defer tape.l.Unlock()
	var completed int
	for _, v := range tape.Vertices {
		if v.Completed != nil {
			completed++
		}
	}
	return completed
}

// UncachedCount returns the number of completed uncached vertexes.
// TODO: cache this
func (tape *Tape) RunningCount() int {
	tape.l.Lock()
	defer tape.l.Unlock()
	var running int
	for _, v := range tape.Vertices {
		if v.Started != nil && v.Completed == nil {
			running++
		}
	}
	return running
}

// UncachedCount returns the number of completed uncached vertexes.
// TODO: cache this
func (tape *Tape) UncachedCount() int {
	tape.l.Lock()
	defer tape.l.Unlock()
	var uncached int
	for _, v := range tape.Vertices {
		if v.Completed != nil && !v.Cached {
			uncached++
		}
	}
	return uncached
}

// CachedCount returns the number of cached vertexes.
// TODO: cache this
func (tape *Tape) CachedCount() int {
	tape.l.Lock()
	defer tape.l.Unlock()
	var cached int
	for _, v := range tape.Vertices {
		if v.Cached {
			cached++
		}
	}
	return cached
}

// ErroredCount returns the number of errored vertexes.
// TODO: cache this
func (tape *Tape) ErroredCount() int {
	tape.l.Lock()
	defer tape.l.Unlock()
	var errored int
	for _, v := range tape.Vertices {
		if v.Error != nil {
			errored++
		}
	}
	return errored
}

// TotalCount returns the total number of vertexes.
func (tape *Tape) TotalCount() int {
	tape.l.Lock()
	defer tape.l.Unlock()
	return len(tape.Vertices)
}

// Duration returns the duration that the Tape has been accepting updates until
// Close has been called.
func (tape *Tape) Duration() time.Duration {
	tape.l.Lock()
	defer tape.l.Unlock()
	if tape.IsClosed {
		return tape.ClosedAt.Sub(tape.CreatedAt)
	}
	return time.Since(tape.CreatedAt)
}

// Close marks the Tape as done, which tells it to display all vertex output
// for the final render.
func (tape *Tape) Close() error {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.IsClosed = true
	tape.ClosedAt = time.Now()
	if tape.frontend != nil {
		return tape.frontend.Close()
	}
	return nil
}

// Closed returns whether the Tape has been closed.
func (tape *Tape) Closed() bool {
	tape.l.Lock()
	defer tape.l.Unlock()
	return tape.IsClosed
}

// SetFrontend configures a custom frontend for the Tape to display.
func (tape *Tape) SetFrontend(f Frontend) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.frontend = f
}

// VerboseEdges sets whether to display edges between vertexes in the same
// group.
func (tape *Tape) VerboseEdges(verbose bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.ShowEdgesBetweenVerticesInSameGroup = verbose
}

// ShowInternal sets whether to show internal vertexes in the output.
func (tape *Tape) ShowInternal(show bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.ShowInternalVertices = show
}

// RevealErrored sets whether to show errored vertexes even when they would
// otherwise not be shown (i.e. internal, or focusing). You may want to set
// this when debugging, or for features that might break when bootstrapping
// before a higher level error can be shown.
func (tape *Tape) RevealErrored(reveal bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.RevealErroredVertices = reveal
}

// ShowAllOutput sets whether to show output even for successful vertexes.
func (tape *Tape) ShowAllOutput(show bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.ShowCompletedLogs = show
}

// Focus sets whether to hide output of non-focused vertexes.
func (tape *Tape) Focus(focused bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.IsFocused = focused
}

// MessageLevel sets the minimum level for messages to display.
func (tape *Tape) MessageLevel(level MessageLevel) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.messageLevel = level
}

// SetWindowSize sets the size of the terminal UI, which influences the
// dimensions for vertex logs, progress bars, etc.
func (tape *Tape) SetWindowSize(w, h int) {
	tape.l.Lock()
	tape.Width = w
	tape.Height = h
	tape.ReasonableTermHeight = h / 4
	for _, l := range tape.vertexLogs {
		l.SetWidth(w)
	}
	for _, t := range tape.zoomed {
		t.Output.Resize(h, w)
	}
	tape.globalLogs.SetWidth(w)
	tape.l.Unlock()
}

var pulse = ui.FadeFrames

func init() {
	framesPerBeat := len(ui.FadeFrames.Frames)
	frames := make([]string, framesPerBeat)
	copy(frames, ui.FadeFrames.Frames)
	lastFrame := (framesPerBeat / 4) * 3
	for i := lastFrame; i < len(frames); i++ {
		frames[i] = ui.FadeFrames.Frames[lastFrame]
	}

	pulse.Frames = frames
}

func (tape *Tape) Render(w io.Writer, u *UI) error {
	tape.l.Lock()
	defer tape.l.Unlock()

	out := ui.NewOutput(w, termenv.WithProfile(tape.ColorProfile))

	zoomSt := tape.currentZoom()

	if zoomSt != tape.lastZoom {
		tape.zoomHook(zoomSt)
		tape.lastZoom = zoomSt
	}

	var err error
	if zoomSt != nil {
		err = tape.renderZoomed(w, u, zoomSt)
	} else if tape.frontend != nil {
		err = tape.frontend.Render(tape, w, u)
	} else if tape.IsFocused {
		err = RenderTree(tape, w, u)
	} else {
		err = tape.renderDAG(out, u)
	}
	if err != nil {
		return err
	}

	tape.globalLogs.SetHeight(10)
	fmt.Fprint(w, tape.globalLogs.View())

	return nil
}

func (tape *Tape) currentZoom() *zoomState {
	var firstZoomed *Vertex
	var firstState *zoomState
	for vId, st := range tape.zoomed {
		v, found := tape.Vertices[vId]
		if !found {
			// should be impossible
			continue
		}

		if v.Started == nil {
			// just being defensive, theoretically this is a valid state
			continue
		}

		if firstZoomed == nil || v.Started.AsTime().Before(firstZoomed.Started.AsTime()) {
			firstZoomed = v
			firstState = st
		}
	}

	return firstState
}

func (tape *Tape) renderZoomed(w io.Writer, u *UI, st *zoomState) error {
	return st.Output.Render(w)
}

func (tape *Tape) renderDAG(out *termenv.Output, u *UI) error {
	b := tape.bouncer()

	var completed []*Vertex
	var runningAndFailed []*Vertex

	for _, dig := range tape.ChronologicalVertexIDs {
		vtx := tape.Vertices[dig]

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

		groups = groups.Reap(out, u, order[i:])

		for _, g := range b.Groups(vtx) {
			groups = groups.AddGroup(out, u, b, g, tape.log)
		}

		if !b.IsVisible(vtx) {
			continue
		}

		symbol := ui.Block
		if vtx.Completed == nil {
			symbol, _, _ = u.Spinner.ViewFrame(pulse)
		}

		groups.VertexPrefix(out, u, vtx, symbol, tape.log)
		if err := u.RenderVertex(out, vtx); err != nil {
			return err
		}

		tasks := tape.VertexTasks[vtx.Id]
		for _, t := range tasks {
			groups.TaskPrefix(out, u, vtx)
			if err := u.RenderTask(out, t); err != nil {
				return err
			}
		}

		activeExceptCurrent := order[1:]

		var haveInput []*Vertex
		for _, a := range activeExceptCurrent {
			if a.HasInput(vtx) && (!b.IsSibling(a, vtx) || tape.ShowEdgesBetweenVerticesInSameGroup) {
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
			groups = groups.AddVertex(out, u, tape.Groups, vtx, haveInput)
		}

		if tape.IsClosed || tape.ShowCompletedLogs || vtx.Completed == nil || vtx.Error != nil {
			term := tape.VertexLogs(vtx.Id)

			if vtx.Error != nil {
				term.SetHeight(term.UsedHeight())
			} else {
				term.SetHeight(tape.ReasonableTermHeight)
			}

			if !tape.IsFocused {
				buf := new(bytes.Buffer)
				prefixOut := ui.NewOutput(buf, termenv.WithProfile(out.Profile))
				groups.TermPrefix(prefixOut, u, vtx)
				term.SetPrefix(buf.String())
			}

			if tape.IsClosed {
				term.SetHeight(term.UsedHeight())
			}

			if err := u.RenderTerm(out, term); err != nil {
				return err
			}
		}
	}

	groups.Reap(out, u, nil)

	return nil
}

func (tape *Tape) bouncer() *bouncer {
	return &bouncer{
		order:          tape.ChronologicalVertexIDs,
		groups:         tape.Groups,
		group2vertexes: tape.GroupVertices,

		vertices:      tape.Vertices,
		vertex2groups: tape.VertexGroups,

		focus:         tape.IsFocused,
		showInternal:  tape.ShowInternalVertices,
		revealErrored: tape.RevealErroredVertices,
	}
}

func (tape *Tape) EachVertex(f func(*Vertex, *ui.Vterm) error) error {
	tape.l.Lock()
	defer tape.l.Unlock()
	for _, vtx := range tape.Vertices {
		if err := f(vtx, tape.VertexLogs(vtx.Id)); err != nil {
			return err
		}
	}
	return nil
}

func (tape *Tape) setZoomHook(fn func(*zoomState)) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.zoomHook = fn
}

func (tape *Tape) insert(id string, vtx *Vertex) {
	if vtx.Started == nil {
		// skip pending vertices; too complicated to deal with
		return
	}

	tape.Vertices[id] = vtx

	for i, dig := range tape.ChronologicalVertexIDs {
		other := tape.Vertices[dig]
		if other.Started.AsTime().After(vtx.Started.AsTime()) {
			inserted := make([]string, len(tape.ChronologicalVertexIDs)+1)
			copy(inserted, tape.ChronologicalVertexIDs[:i])
			inserted[i] = id
			copy(inserted[i+1:], tape.ChronologicalVertexIDs[i:])
			tape.ChronologicalVertexIDs = inserted
			return
		}
	}

	tape.ChronologicalVertexIDs = append(tape.ChronologicalVertexIDs, id)
}

func (tape *Tape) VertexLogs(vertex string) *ui.Vterm {
	term, found := tape.vertexLogs[vertex]
	if !found {
		term = ui.NewVterm()
		if tape.Width != -1 {
			term.SetWidth(tape.Width)
		}
		tape.vertexLogs[vertex] = term
	}

	return term
}

type bouncer struct {
	order          []string
	groups         map[string]*Group
	vertices       map[string]*Vertex
	group2vertexes map[string]map[string]struct{}
	vertex2groups  map[string]map[string]struct{}

	focus         bool
	showInternal  bool
	revealErrored bool
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

func (b *bouncer) IsVisible(vtx *Vertex) bool {
	if vtx.Error != nil && b.revealErrored {
		// show errored vertices no matter what
		return true
	}

	if vtx.Internal && !b.showInternal {
		// filter out internal vertices unless we're showing them
		return false
	}

	if b.focus && !vtx.Focused {
		// if we're focusing, ignore unfocused vertices
		return false
	}

	return true
}

func (b *bouncer) VisibleVertices(group *Group) []*Vertex {
	var visible []*Vertex

	if group == nil {
		for _, vid := range b.order {
			vtx, found := b.vertices[vid]
			if !found {
				// shouldn't happen
				continue
			}

			orphaned := len(b.vertex2groups[vid]) == 0

			if orphaned && b.IsVisible(vtx) {
				visible = append(visible, vtx)
			}
		}

		return visible
	}

	for vid := range b.group2vertexes[group.Id] {
		vtx, found := b.vertices[vid]
		if !found {
			// shouldn't happen
			continue
		}

		if b.IsVisible(vtx) {
			visible = append(visible, vtx)
		}
	}

	return visible
}

func (b *bouncer) Parent(group *Group) (*Group, bool) {
	if group.Parent == nil {
		return nil, false
	}

	parent, found := b.groups[group.GetParent()]
	return parent, found
}

// TODO cache the hell out of this
func (b *bouncer) HiddenAncestors(group *Group) []*Group {
	var hidden []*Group
	for {
		parent, found := b.Parent(group)
		if !found {
			break
		}

		if len(b.VisibleVertices(parent)) == 0 {
			hidden = append(hidden, parent)
		} else {
			break
		}

		group = parent
	}

	return hidden
}

func (b *bouncer) VisibleDepth(group *Group) int {
	if group.Parent == nil {
		return 0
	}

	var depth int
	vs := b.VisibleVertices(group)
	if len(vs) > 0 {
		depth++
	}

	parent, found := b.groups[group.GetParent()]
	if !found {
		return depth
	}

	return depth + b.VisibleDepth(parent)
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
func (groups progressGroups) AddGroup(out *termenv.Output, u *UI, b *bouncer, group *Group, log func(*Message)) progressGroups {
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
			log(&Message{
				Level:   MessageLevel_DEBUG,
				Message: "group has unknown parent",
				Labels: []*Label{
					{Name: "group", Value: group.Id},
					{Name: "parent", Value: group.GetParent()},
				},
			})
			return groups
		}

		return groups.AddGroup(out, u, b, parent, log).
			AddGroup(out, u, b, group, log)
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
			fmt.Fprint(out, groupColor(out, parentIdx, ui.VertRightBoldBar))
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if i == parentIdx && addedIdx < parentIdx {
			// line towards the left of the parent
			fmt.Fprint(out, groupColor(out, parentIdx, ui.VertLeftBoldBar))
			fmt.Fprint(out, " ")
		} else if i == addedIdx && addedIdx > parentIdx {
			// line left from parent and down to added line
			fmt.Fprint(out, groupColor(out, addedIdx, ui.CornerTopRight))
			fmt.Fprint(out, " ")
		} else if i == addedIdx && addedIdx < parentIdx {
			// line up from added line and right to parent
			fmt.Fprint(out, groupColor(out, addedIdx, ui.CornerTopLeft))
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if parentIdx != -1 && addedIdx > parentIdx && i > parentIdx && i < addedIdx {
			// line between parent and added line
			if g != nil {
				fmt.Fprint(out, groupColor(out, i, ui.CrossBar))
			} else {
				fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
			}
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if parentIdx != -1 && addedIdx < parentIdx && i < parentIdx && i > addedIdx {
			// line between parent and added line
			if g != nil {
				fmt.Fprint(out, groupColor(out, i, ui.CrossBar))
			} else {
				fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
			}
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if groups[i] != nil {
			fmt.Fprint(out, groupColor(out, i, ui.InactiveGroupSymbol))
			fmt.Fprint(out, " ")
		} else {
			fmt.Fprint(out, "  ")
		}
	}

	fmt.Fprintln(out)
	groups.GroupName(out, u, group)

	return groups
}

// AddVertex adds a vertex-group to the set of groups, which is used to connect
// vertex inputs and outputs. It also renders the new groups.
func (groups progressGroups) AddVertex(out *termenv.Output, u *UI, allGroups map[string]*Group, vtx *Vertex, outputs []*Vertex) progressGroups {
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
			fmt.Fprint(out, groupColor(out, parentIdx, ui.VertRightBoldBar))
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if i == parentIdx && addedIdx < parentIdx {
			// line towards the left of the parent
			fmt.Fprint(out, groupColor(out, parentIdx, ui.VertLeftBoldBar))
			fmt.Fprint(out, " ")
		} else if parentIdx != -1 && i == addedIdx && addedIdx > parentIdx {
			// line left from parent and down to added line
			fmt.Fprint(out, groupColor(out, addedIdx, ui.CornerTopRight))
			fmt.Fprint(out, " ")
		} else if parentIdx != -1 && i == addedIdx && addedIdx < parentIdx {
			// line up from added line and right to parent
			fmt.Fprint(out, groupColor(out, addedIdx, ui.CornerTopLeft))
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if parentIdx != -1 && addedIdx > parentIdx && i > parentIdx && i < addedIdx {
			// line between parent and added line
			if g != nil {
				fmt.Fprint(out, groupColor(out, i, ui.CrossBar))
			} else {
				fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
			}
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if parentIdx != -1 && addedIdx < parentIdx && i < parentIdx && i > addedIdx {
			// line between parent and added line
			if g != nil {
				fmt.Fprint(out, groupColor(out, i, ui.CrossBar))
			} else {
				fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
			}
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if parentIdx == -1 && i == addedIdx {
			fmt.Fprint(out, groupColor(out, addedIdx, ui.DotEmpty)) // TODO pointless?
			fmt.Fprint(out, groupColor(out, addedIdx, ui.HorizBar))
		} else if groups[i] != nil {
			fmt.Fprint(out, groupColor(out, i, ui.InactiveGroupSymbol))
			fmt.Fprint(out, " ")
		} else {
			fmt.Fprint(out, "  ")
		}
	}

	fmt.Fprintln(out, out.String(vtx.Name).Foreground(termenv.ANSIBrightBlack))

	return groups
}

// VertexPrefix prints the prefix for a vertex.
func (groups progressGroups) VertexPrefix(out *termenv.Output, u *UI, vtx *Vertex, selfSymbol string, log func(*Message)) {
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
		log(&Message{
			Level:   MessageLevel_DEBUG,
			Message: "vertex has no containing group",
			Labels: []*Label{
				{Name: "vertex", Value: vtx.Id},
				{Name: "name", Value: vtx.Name},
			},
		})
	}

	for i, g := range groups {
		var symbol string
		if g == nil {
			if firstParentIdx != -1 && i < vtxIdx && i >= firstParentIdx {
				fmt.Fprint(out, groupColor(out, firstParentIdx, ui.HorizBar))
			} else if firstParentIdx != -1 && i >= vtxIdx && i < lastParentIdx {
				fmt.Fprint(out, groupColor(out, lastParentIdx, ui.HorizBar))
			} else {
				fmt.Fprint(out, " ")
			}
		} else {
			if g.DirectlyContains(vtx) {
				symbol = selfSymbol
				vtxIdx = i
			} else if g.Created(vtx) && i > vtxIdx {
				if g.WitnessedAll() {
					if i == lastParentIdx {
						symbol = ui.CornerBottomRight
					} else {
						symbol = ui.HorizTopBar
					}
				} else {
					symbol = ui.VertLeftBar
				}
			} else if g.Created(vtx) {
				if g.WitnessedAll() {
					if i == firstParentIdx {
						symbol = ui.CornerBottomLeft
					} else {
						symbol = ui.HorizTopBar
					}
				} else {
					symbol = ui.VertRightBar
				}
			} else if firstParentIdx != -1 && i >= firstParentIdx && i < vtxIdx {
				symbol = ui.CrossBar
			} else if firstParentIdx != -1 && i >= vtxIdx && i < lastParentIdx {
				symbol = ui.CrossBar
			} else {
				symbol = ui.InactiveGroupSymbol
			}

			// respect color of the group
			fmt.Fprint(out, groupColor(out, i, symbol))
		}

		if firstParentIdx != -1 && vtxIdx > firstParentIdx && i >= firstParentIdx && i < vtxIdx {
			if i+1 == vtxIdx {
				fmt.Fprint(out, groupColor(out, firstParentIdx, ui.CaretRightFilled))
			} else {
				fmt.Fprint(out, groupColor(out, firstParentIdx, ui.HorizBar))
			}
		} else if firstParentIdx != -1 && vtxIdx < firstParentIdx && i >= vtxIdx && i < lastParentIdx {
			if i == vtxIdx {
				fmt.Fprint(out, groupColor(out, lastParentIdx, ui.CaretLeftFilled))
			} else {
				fmt.Fprint(out, groupColor(out, lastParentIdx, ui.HorizBar))
			}
		} else if firstParentIdx != -1 && i >= vtxIdx && i < lastParentIdx {
			if i == vtxIdx {
				fmt.Fprint(out, groupColor(out, lastParentIdx, ui.CaretLeftFilled))
			} else {
				fmt.Fprint(out, groupColor(out, lastParentIdx, ui.HorizBar))
			}
		} else {
			fmt.Fprint(out, " ")
		}
	}
}

// TaskPrefix prints the prefix for a vertex's task.
func (groups progressGroups) TaskPrefix(out *termenv.Output, u *UI, vtx *Vertex) {
	groups.printPrefix(out, func(g progressGroup, vtx *Vertex) string {
		if g.DirectlyContains(vtx) {
			return ui.TaskSymbol
		}
		return ui.InactiveGroupSymbol
	}, vtx)
}

// GroupName prints the prefix and name for newly added group.
func (groups progressGroups) GroupName(out *termenv.Output, u *UI, group *Group) {
	groups.printPrefix(out, func(g progressGroup, _ *Vertex) string {
		if g.ID() == group.Id {
			if group.Weak {
				return ui.CaretDownEmpty
			} else {
				return ui.CaretDownFilled
			}
		}
		return ui.InactiveGroupSymbol
	}, nil)
	if group.Weak {
		fmt.Fprintln(out, group.Name)
	} else {
		fmt.Fprintln(out, out.String(group.Name).Bold())
	}
}

// TermPrefix prints the prefix for a vertex's terminal output.
func (groups progressGroups) TermPrefix(out *termenv.Output, u *UI, vtx *Vertex) {
	groups.printPrefix(out, func(g progressGroup, vtx *Vertex) string {
		if g.DirectlyContains(vtx) {
			return ui.VertBoldBar
		}
		return ui.InactiveGroupSymbol
	}, vtx)
}

// Reap removes groups that are no longer active.
func (groups progressGroups) Reap(out *termenv.Output, u *UI, active []*Vertex) progressGroups {
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
				fmt.Fprint(out, groupColor(out, i, ui.InactiveGroupSymbol))
				fmt.Fprint(out, " ")
			} else if reaped[i] {
				fmt.Fprint(out, groupColor(out, i, ui.HorizTopBoldBar))
				fmt.Fprint(out, " ")
			} else {
				fmt.Fprint(out, "  ")
			}
		}
		fmt.Fprintln(out)
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

func (groups progressGroups) printPrefix(out *termenv.Output, sym func(progressGroup, *Vertex) string, vtx *Vertex) {
	for i, g := range groups {
		if g == nil {
			fmt.Fprint(out, " ")
		} else {
			fmt.Fprint(out, groupColor(out, i, sym(g, vtx)))
		}
		fmt.Fprint(out, " ")
	}
}

func groupColor(out *termenv.Output, i int, str string) string {
	return out.String(str).Foreground(
		termenv.ANSIColor((i+1)%7) + 1,
	).String()
}

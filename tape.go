package progrock

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/muesli/termenv"
	"github.com/vito/midterm"
	"github.com/vito/progrock/ui"
	"golang.org/x/exp/slices"
)

// Tape is a Writer that collects all progress output for displaying in a
// terminal UI.
type Tape struct {
	// order to display vertices
	order []string

	// zoomed vertices
	zoomed map[string]*midterm.Terminal

	// raw vertex/group state from the event stream
	vertexes map[string]*Vertex
	groups   map[string]*Group

	// vertex <-> group mappings
	group2vertexes map[string]map[string]struct{}
	vertex2groups  map[string]map[string]struct{}

	// vertex state
	tasks map[string][]*VertexTask
	logs  map[string]*ui.Vterm

	// whether and when the tape has been closed
	closed   bool
	closedAt time.Time

	// the time that tape was created and closed
	createdAt time.Time

	// screen width and height
	width, height int

	// visible region for active vterms
	termHeight int

	// UI config
	verboseEdges  bool // show edges between vertexes in the same group
	showInternal  bool // show internal vertexes
	revealErrored bool // show errored vertexes no matter what
	showAllOutput bool // show output even for completed vertexes
	focus         bool // only show 'focused' vertex output, condensing the rest

	// output from messages and internal debugging
	globalLogs *ui.Vterm

	// minimum message level to display to the user
	messageLevel MessageLevel

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
	rEmptyCaret = "▷" //">"
	dCaret      = "▼"
	dEmptyCaret = "▽"

	taskSymbol          = vrbBar
	inactiveGroupSymbol = vBar

	iconSkipped = "∅"
	iconSuccess = "✔"
	iconFailure = "✘"
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
		zoomed:         make(map[string]*midterm.Terminal),

		// default to unbounded screen size so we don't arbitrarily wrap log output
		// when no size is given
		width:  -1,
		height: -1,

		// sane default before window size is known
		termHeight: 10,

		globalLogs: ui.NewVterm(),

		messageLevel: MessageLevel_WARNING,

		createdAt: time.Now(),
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

		_, isZoomed := tape.zoomed[v.Id]
		if v.Zoomed && !isZoomed {
			tape.zoom(v)
		} else if isZoomed {
			tape.unzoom(v)
		}
	}

	for _, t := range status.Tasks {
		tape.recordTask(t)
	}

	for _, l := range status.Logs {
		var w io.Writer
		if t, found := tape.zoomed[l.Vertex]; found {
			w = t
		} else {
			w = tape.vertexLogs(l.Vertex)
		}
		_, err := w.Write(l.Data)
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

	for _, msg := range status.Messages {
		tape.log(msg)
	}

	return nil
}

func (tape *Tape) zoom(v *Vertex) {
	tape.l.Lock()
	defer tape.l.Unlock()
	var vt *midterm.Terminal
	if tape.height == -1 || tape.width != -1 {
		vt = midterm.NewAutoResizingTerminal()
	} else {
		vt = midterm.NewTerminal(tape.height, tape.width)
	}
	tape.zoomed[v.Id] = vt
	setupTerm(v.Id, vt)
}

func (tape *Tape) unzoom(vtx *Vertex) {
	tape.l.Lock()
	defer tape.l.Unlock()
	delete(tape.zoomed, vtx.Id)
}

func (tape *Tape) recordTask(t *VertexTask) {
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

func (tape *Tape) log(msg *Message) {
	if msg.Level < tape.messageLevel {
		return
	}

	WriteMessage(tape.globalLogs, msg)
}

func WriteMessage(w io.Writer, msg *Message) error {
	var prefix termenv.Style
	switch msg.Level {
	case MessageLevel_DEBUG:
		prefix = termenv.String("DEBUG:").Foreground(termenv.ANSIBlue).Bold()
	case MessageLevel_WARNING:
		prefix = termenv.String("WARNING:").Foreground(termenv.ANSIYellow).Bold()
	case MessageLevel_ERROR:
		prefix = termenv.String("ERROR:").Foreground(termenv.ANSIRed).Bold()
	}

	out := msg.Message
	for _, l := range msg.Labels {
		out += " "
		out += termenv.String(fmt.Sprintf("%s=%q", l.Name, l.Value)).
			Foreground(termenv.ANSIBrightBlack).
			String()
	}

	_, err := fmt.Fprintln(w, prefix, out)
	return err
}

func (tape *Tape) Vertices() []*Vertex {
	tape.l.Lock()
	defer tape.l.Unlock()

	var vertices []*Vertex
	for _, vid := range tape.order {
		vtx, found := tape.vertexes[vid]
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

	for i := len(tape.order) - 1; i >= 0; i-- {
		vid := tape.order[i]
		vtx := tape.vertexes[vid]
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

	term := tape.logs[vtx.Id]
	if term != nil {
		activity.LastLine = term.LastLine()
	}

	tasks := tape.tasks[vtx.Id]
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
	for _, v := range tape.vertexes {
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
	for _, v := range tape.vertexes {
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
	for _, v := range tape.vertexes {
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
	for _, v := range tape.vertexes {
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
	for _, v := range tape.vertexes {
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
	return len(tape.vertexes)
}

// Duration returns the duration that the Tape has been accepting updates until
// Close has been called.
func (tape *Tape) Duration() time.Duration {
	tape.l.Lock()
	defer tape.l.Unlock()
	if tape.closed {
		return tape.closedAt.Sub(tape.createdAt)
	}
	return time.Since(tape.createdAt)
}

// Close marks the Tape as done, which tells it to display all vertex output
// for the final render.
func (tape *Tape) Close() error {
	tape.l.Lock()
	tape.closed = true
	tape.closedAt = time.Now()
	tape.l.Unlock()
	return nil
}

// Closed returns whether the Tape has been closed.
func (tape *Tape) Closed() bool {
	tape.l.Lock()
	defer tape.l.Unlock()
	return tape.closed
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

// RevealErrored sets whether to show errored vertexes even when they would
// otherwise not be shown (i.e. internal, or focusing). You may want to set
// this when debugging, or for features that might break when bootstrapping
// before a higher level error can be shown.
func (tape *Tape) RevealErrored(reveal bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.revealErrored = reveal
}

// ShowAllOutput sets whether to show output even for successful vertexes.
func (tape *Tape) ShowAllOutput(show bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.showAllOutput = show
}

// Focus sets whether to hide output of non-focused vertexes.
func (tape *Tape) Focus(focused bool) {
	tape.l.Lock()
	defer tape.l.Unlock()
	tape.focus = focused
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
	tape.width = w
	tape.height = h
	tape.termHeight = h / 4
	for _, l := range tape.logs {
		l.SetWidth(w)
	}
	for _, t := range tape.zoomed {
		t.Resize(h, w)
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

	var err error
	if tape.isZoomed() {
		err = tape.renderZoomed(w, u)
	} else if tape.focus {
		err = tape.renderTree(w, u)
	} else {
		err = tape.renderDAG(w, u)
	}
	if err != nil {
		return err
	}

	tape.globalLogs.SetHeight(10)
	fmt.Fprint(w, tape.globalLogs.View())

	return nil
}

func (tape *Tape) isZoomed() bool {
	for vid := range tape.zoomed {
		if tape.vertexes[vid].Completed == nil {
			return true
		}
	}

	return false
}

func (tape *Tape) renderZoomed(w io.Writer, u *UI) error {
	for _, t := range tape.zoomed {
		// used := t.UsedHeight()
		// if used == 0 {
		// 	return nil
		// }

		for row := range t.Content {
			if err := t.RenderLine(w, row); err != nil {
				return err
			}

			// 			if row > used {
			// 				break
			// 			}
		}

		// height, cols, err := pty.Getsize(os.Stdin)
		// fmt.Fprintln(w, "TW", tape.width, "TH", tape.height, "LEN", len(t.Content), "WIN", height, cols, err)
	}

	return nil
}

func (tape *Tape) renderDAG(w io.Writer, u *UI) error {
	b := tape.bouncer()

	var groupsW io.Writer
	if tape.focus {
		groupsW = io.Discard
	} else {
		groupsW = w
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

		groups = groups.Reap(groupsW, u, order[i:])

		for _, g := range b.Groups(vtx) {
			groups = groups.AddGroup(groupsW, u, b, g, tape.log)
		}

		if !b.IsVisible(vtx) {
			continue
		}

		symbol := block
		if vtx.Completed == nil {
			symbol, _, _ = u.Spinner.ViewFrame(pulse)
		}

		groups.VertexPrefix(groupsW, u, vtx, symbol, tape.log)
		if err := u.RenderVertex(w, vtx); err != nil {
			return err
		}

		tasks := tape.tasks[vtx.Id]
		for _, t := range tasks {
			groups.TaskPrefix(groupsW, u, vtx)
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
			groups = groups.AddVertex(groupsW, u, tape.groups, vtx, haveInput)
		}

		if tape.closed || tape.showAllOutput || vtx.Completed == nil || vtx.Error != nil {
			term := tape.vertexLogs(vtx.Id)

			if vtx.Error != nil {
				term.SetHeight(term.UsedHeight())
			} else {
				term.SetHeight(tape.termHeight)
			}

			if !tape.focus {
				buf := new(bytes.Buffer)
				groups.TermPrefix(buf, u, vtx)
				term.SetPrefix(buf.String())
			}

			if tape.closed {
				term.SetHeight(term.UsedHeight())
			}

			if err := u.RenderTerm(w, term); err != nil {
				return err
			}
		}
	}

	groups.Reap(groupsW, u, nil)

	return nil
}

func (b *bouncer) Ancestry(grp *Group) []*Group {
	if grp == nil {
		return nil
	}

	if grp.Parent == nil {
		return []*Group{grp}
	}

	return append(b.Ancestry(b.groups[*grp.Parent]), grp)
}

func (tape *Tape) renderTree(w io.Writer, u *UI) error {
	b := tape.bouncer()

	var groups []*Group
	for _, group := range b.groups {
		groups = append(groups, group)
	}
	// TODO: (likely) stable order
	sort.Slice(groups, func(i, j int) bool {
		gi := groups[i]
		gj := groups[j]
		return gi.Started.AsTime().Before(gj.Started.AsTime())
	})

	renderVtx := func(vtx *Vertex, indent string) error {
		var symbol string
		var color termenv.Color
		if vtx.Completed != nil {
			if vtx.Error != nil {
				symbol = iconFailure
				color = termenv.ANSIRed
			} else if vtx.Canceled {
				symbol = iconSkipped
				color = termenv.ANSIBrightBlack
			} else {
				symbol = iconSuccess
				color = termenv.ANSIGreen
			}
		} else {
			symbol, _, _ = u.Spinner.ViewFrame(ui.DotFrames)
			color = termenv.ANSIYellow
		}

		symbol = termenv.String(symbol).Foreground(color).String()

		fmt.Fprintf(w, "%s%s ", indent, symbol)

		if err := u.RenderVertexTree(w, vtx); err != nil {
			return err
		}

		// TODO dedup from renderGraph
		if tape.closed || tape.showAllOutput || vtx.Completed == nil || vtx.Error != nil {
			tasks := tape.tasks[vtx.Id]
			for _, t := range tasks {
				fmt.Fprint(w, indent, vrBar, " ")
				if err := u.RenderTask(w, t); err != nil {
					return err
				}
			}

			term := tape.vertexLogs(vtx.Id)

			if vtx.Error != nil {
				term.SetHeight(term.UsedHeight())
			} else {
				term.SetHeight(tape.termHeight)
			}

			term.SetPrefix(indent + termenv.String(vbBar).Foreground(color).String() + " ")

			if tape.closed {
				term.SetHeight(term.UsedHeight())
			}

			if err := u.RenderTerm(w, term); err != nil {
				return err
			}
		}

		return nil
	}

	// TODO: this ended up being super complicated after a lot of flailing and
	// can probably be simplified. the goal is to render the groups depth-first,
	// in a stable order. i tried harder and harder and it ended up being an
	// unrelated mutation bug. -_-
	rendered := map[string]struct{}{}
	var render func(g *Group) error
	render = func(g *Group) error {
		if _, f := rendered[g.Id]; f {
			return nil
		} else {
			rendered[g.Id] = struct{}{}
		}

		defer func() {
			for _, sub := range groups {
				if sub.Parent != nil && *sub.Parent == g.Id {
					render(sub)
				}
			}
		}()

		vs := b.VisibleVertices(g)
		if len(vs) == 0 {
			return nil
		}

		sort.Slice(vs, func(i, j int) bool {
			return slices.Index(tape.order, vs[i].Id) < slices.Index(tape.order, vs[j].Id)
		})

		depth := b.VisibleDepth(g) - 1
		if depth < 0 {
			depth = 0
		}
		indent := strings.Repeat("  ", depth)

		if g.Name != RootGroup {
			var names []string
			for _, ancestor := range b.HiddenAncestors(g) {
				if ancestor.Name == RootGroup {
					continue
				}

				names = append(names, ancestor.Name)
			}
			names = append(names, g.Name)
			groupPath := strings.Join(names, " "+rCaret+" ")
			fmt.Fprintf(w, "%s%s %s\n", indent, rCaret, termenv.String(groupPath).Bold())
			indent += "  "
		}

		for _, vtx := range vs {
			if err := renderVtx(vtx, indent); err != nil {
				return err
			}
		}

		return nil
	}

	for _, v := range b.VisibleVertices(nil) {
		if err := renderVtx(v, ""); err != nil {
			return err
		}
	}

	for _, g := range groups {
		if err := render(g); err != nil {
			return err
		}
	}

	return nil
}

func (tape *Tape) bouncer() *bouncer {
	return &bouncer{
		order:          tape.order,
		groups:         tape.groups,
		group2vertexes: tape.group2vertexes,

		vertices:      tape.vertexes,
		vertex2groups: tape.vertex2groups,

		focus:         tape.focus,
		showInternal:  tape.showInternal,
		revealErrored: tape.revealErrored,
	}
}

func (tape *Tape) EachVertex(f func(*Vertex, *ui.Vterm) error) error {
	tape.l.Lock()
	defer tape.l.Unlock()
	for _, vtx := range tape.vertexes {
		if err := f(vtx, tape.vertexLogs(vtx.Id)); err != nil {
			return err
		}
	}
	return nil
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

func (tape *Tape) vertexLogs(vertex string) *ui.Vterm {
	term, found := tape.logs[vertex]
	if !found {
		term = ui.NewVterm()
		if tape.width != -1 {
			term.SetWidth(tape.width)
		}
		tape.logs[vertex] = term
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
func (groups progressGroups) AddGroup(w io.Writer, u *UI, b *bouncer, group *Group, log func(*Message)) progressGroups {
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

		return groups.AddGroup(w, u, b, parent, log).
			AddGroup(w, u, b, group, log)
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
func (groups progressGroups) VertexPrefix(w io.Writer, u *UI, vtx *Vertex, selfSymbol string, log func(*Message)) {
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
				fmt.Fprint(w, groupColor(lastParentIdx, lCaret))
			} else {
				fmt.Fprint(w, groupColor(lastParentIdx, hBar))
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

package progrock

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/vito/progrock/tmpl"
	"github.com/vito/progrock/ui"
	"golang.org/x/term"
)

type UI struct {
	Spinner ui.Spinner

	colorProfile termenv.Profile

	width, height int

	tmpl *template.Template
}

func NewUI(spinner ui.Spinner) *UI {
	ui := &UI{
		colorProfile: ui.ColorProfile(),
		Spinner:      spinner,
	}
	ui.tmpl = template.New("ui").
		Funcs(termenv.TemplateFuncs(ui.colorProfile)).
		Funcs(template.FuncMap{
			"duration": fmtDuration,
			"bar": func(current, total int64) string {
				bar := progress.New(
					progress.WithColorProfile(ui.colorProfile),
					progress.WithSolidFill("2"),
				)
				bar.Width = ui.width / 8
				bar.EmptyColor = "8"
				bar.ShowPercentage = false
				return bar.ViewAs(float64(current) / float64(total))
			},
			"join": func(delimiter string, elements []string) string {
				return strings.Join(elements, delimiter)
			},
			"words": func(s string) []string {
				return strings.Fields(s)
			},
		})
	return ui
}

func DefaultUI() *UI {
	ui := NewUI(ui.NewRave())
	if err := ui.ParseFS(tmpl.FS, "*.tmpl"); err != nil {
		panic(err)
	}
	return ui
}

func (ui *UI) SetWindowSize(width, height int) {
	ui.width = width
	ui.height = height
}

func (ui *UI) ParseFS(fs fs.FS, globs ...string) error {
	tmpl, err := ui.tmpl.ParseFS(fs, globs...)
	if err != nil {
		return err
	}

	ui.tmpl = tmpl

	return nil
}

func (ui *UI) RenderVertex(w io.Writer, v *Vertex) error {
	return ui.tmpl.Lookup("vertex.tmpl").Execute(w, v)
}

func (ui *UI) RenderVertexTree(w io.Writer, v *Vertex) error {
	return ui.tmpl.Lookup("vertex-tree.tmpl").Execute(w, v)
}

func (ui *UI) RenderTask(w io.Writer, v *VertexTask) error {
	return ui.tmpl.Lookup("task.tmpl").Execute(w, v)
}

func (ui *UI) RenderTerm(w io.Writer, term *ui.Vterm) error {
	_, err := fmt.Fprint(w, term.View())
	return err
}

func (ui *UI) RenderTrailer(w io.Writer, infos []StatusInfo) error {
	return ui.tmpl.Lookup("trailer.tmpl").Execute(w, struct {
		Infos []StatusInfo
	}{
		Infos: infos,
	})
}

func (u *UI) RenderStatus(w io.Writer, tape *Tape, infos []StatusInfo) error {
	spinner, _, _ := u.Spinner.ViewFrame(ui.DotFrames)
	return u.tmpl.Lookup("status.tmpl").Execute(w, struct {
		Spinner      string
		VertexSymbol string
		Tape         *Tape
		Infos        []StatusInfo
	}{
		Spinner:      spinner,
		VertexSymbol: block,
		Tape:         tape,
		Infos:        infos,
	})
}

type swappableWriter struct {
	original io.Writer
	override io.Writer
	sync.Mutex
}

func (w *swappableWriter) SetOverride(to io.Writer) {
	w.Lock()
	w.override = to
	w.Unlock()
}

func (w *swappableWriter) Restore() {
	w.SetOverride(nil)
}

func (w *swappableWriter) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	if w.override != nil {
		return w.override.Write(p)
	}
	return w.original.Write(p)
}

type RunFunc = func(context.Context, UIClient) error

// UIClient is an interface for miscellaneous UI-only knobs.
type UIClient interface {
	SetStatusInfo(StatusInfo)
}

func (u *UI) Run(ctx context.Context, tape *Tape, fn RunFunc) error {
	tty, isTTY := findTTY()

	// NOTE: establish color cache before we start consuming stdin
	out := ui.NewOutput(tty, termenv.WithColorCache(true))

	var ttyFd int
	var oldState *term.State
	var inR io.Reader
	if isTTY {
		ttyFd = int(tty.Fd())

		var err error
		oldState, err = term.MakeRaw(ttyFd)
		if err != nil {
			return err
		}
		defer term.Restore(ttyFd, oldState) // nolint: errcheck

		var inW io.Writer
		inR, inW = io.Pipe()

		sw := &swappableWriter{original: inW}
		tape.setZoomHook(func(st *zoomState) { // TODO weird tight coupling.
			if st == nil {
				sw.Restore()

				// restore scrolling as we transition back to the DAG UI, since an app
				// may have disabled it
				out.EnableMouseCellMotion()
			} else {
				// disable mouse events, can't assume zoomed input wants it (might be
				// regular shell like sh)
				out.DisableMouseCellMotion()

				sw.SetOverride(st.Input) // may be nil, same as Restore
			}
		})

		go io.Copy(sw, tty) // nolint: errcheck
	} else {
		// no TTY found, so no input can be sent to the TUI
		inR = nil
	}

	// DAG UI supports scrolling, since the only other option is to have it cut
	// offscreen
	out.EnableMouseCellMotion()

	model := u.newModel(ctx, tape, fn)

	prog := tea.NewProgram(model, tea.WithInput(inR), tea.WithOutput(out))

	if _, err := prog.Run(); err != nil {
		return err
	}

	if oldState != nil {
		_ = term.Restore(ttyFd, oldState)
	}

	// for the final render, _always_ print to stderr so that it's possible to
	// capture the output, otherwise it'll keep "dodging" the redirected fd. we
	// just don't want to print a ton of escape sequences. while the TUI draws.
	model.Print(os.Stderr)
	model.PrintTrailer(os.Stderr)

	return model.err
}

func findTTY() (*os.File, bool) {
	// some of these may be redirected
	for _, f := range []*os.File{os.Stderr, os.Stdout, os.Stdin} {
		if term.IsTerminal(int(f.Fd())) {
			return f, true
		}
	}
	return nil, false
}

func (u *UI) newModel(ctx context.Context, tape *Tape, run RunFunc) *Model {
	runCtx, interrupt := context.WithCancel(ctx)
	return &Model{
		tape: tape,
		ui:   u,

		run:       run,
		runCtx:    runCtx,
		interrupt: interrupt,

		fps: 10,

		// sane defaults before we have the real window size
		maxWidth:  80,
		maxHeight: 24,
		content:   viewport.New(80, 24),

		contentBuf: new(bytes.Buffer),
		chromeBuf:  new(bytes.Buffer),
	}
}

type Model struct {
	run       RunFunc
	runCtx    context.Context
	interrupt func()
	done      bool
	err       error

	tape *Tape

	ui *UI

	content       viewport.Model
	contentHeight int

	chrome       string
	chromeHeight int

	// screen dimensions
	maxWidth  int
	maxHeight int

	// buffers for async rendering so we're not constantly reallocating
	chromeBuf  *bytes.Buffer
	contentBuf *bytes.Buffer

	// custom info to display, set by the UIClient
	_infos []StatusInfo
	infosL sync.Mutex

	// UI refresh rate
	fps float64
}

func (m *Model) Print(w io.Writer) {
	if err := m.tape.Render(w, m.ui); err != nil {
		fmt.Fprintln(w, "failed to render tape:", err)
		return
	}
}

func (m *Model) PrintTrailer(w io.Writer) {
	if err := m.ui.RenderStatus(w, m.tape, m.infos()); err != nil {
		fmt.Fprintln(w, "failed to render trailer:", err)
		return
	}
	fmt.Fprintln(w)
}

var _ tea.Model = (*Model)(nil)

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		ui.Frame(m.fps),
		m.ui.Spinner.Init(),
		m.spawn,
	)
}

type doneMsg struct {
	err error
}

func (m *Model) spawn() tea.Msg {
	return doneMsg{m.run(m.runCtx, m)}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, ui.Keys.Quit):
			m.interrupt()
			return m, nil
		}

		s, cmd := m.ui.Spinner.Update(msg)
		m.ui.Spinner = s.(ui.Spinner)
		cmds = append(cmds, cmd)

	case doneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.setWindowSize(msg.Width, msg.Height)

	case ui.FrameMsg:
		// NB: take care not to forward Frame downstream, since that will result
		// in runaway ticks. instead inner components should send a SetFpsMsg to
		// adjust the outermost layer.
		m.render()
		cmds = append(cmds, ui.Frame(m.fps))

	case ui.SetFPSMsg:
		m.fps = float64(msg)

	default:
		s, cmd := m.ui.Spinner.Update(msg)
		m.ui.Spinner = s.(ui.Spinner)
		cmds = append(cmds, cmd)

		m.content, cmd = m.content.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	if m.done {
		// print nothing on exit; the outer render loop will call Print one last
		// time, otherwise bubbletea crops out the lines that go offscreen
		return ""
	}

	output := m.content.View()
	if output == "\n" {
		output = ""
	}
	if output == "" {
		return m.chrome
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		output,
		m.chrome,
	)
}

var _ UIClient = (*Model)(nil)

type StatusInfo struct {
	Name  string
	Value string
	Order int
}

func (m *Model) SetStatusInfo(info StatusInfo) {
	m.infosL.Lock()
	defer m.infosL.Unlock()
	infos := append([]StatusInfo{}, m._infos...)
	infos = append(infos, info)
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Order == infos[j].Order {
			return infos[i].Name < infos[j].Name
		}
		return infos[i].Order < infos[j].Order
	})
	m._infos = infos
}

func (m *Model) infos() []StatusInfo {
	m.infosL.Lock()
	defer m.infosL.Unlock()
	return append([]StatusInfo{}, m._infos...)
}

func (m *Model) render() {
	prevHeight := m.chromeHeight

	m.chrome = m.viewChrome()
	m.chromeHeight = lipgloss.Height(m.chrome)

	if m.chromeHeight != prevHeight {
		// resize tape (e.g. for zoomed vertex output)
		m.setWindowSize(m.maxWidth, m.maxHeight)
	}

	m.contentBuf.Reset()
	m.Print(m.contentBuf)
	content := strings.TrimRight(m.contentBuf.String(), "\n")
	m.contentHeight = lipgloss.Height(content)

	atBottom := m.content.AtBottom()

	m.content.SetContent(content)

	max := m.maxHeight - m.chromeHeight
	m.content.Height = m.contentHeight
	if m.content.Height+m.chromeHeight > m.maxHeight {
		m.content.Height = max
	}

	if atBottom {
		m.content.GotoBottom()
	}
}

func (m *Model) viewChrome() string {
	m.chromeBuf.Reset()
	m.ui.RenderStatus(m.chromeBuf, m.tape, m.infos())

	return lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		lipgloss.NewStyle().
			MaxWidth(m.maxWidth).
			Render(m.chromeBuf.String()),
	)
}

func (m *Model) viewportWidth() int {
	width := m.content.Width
	if width == 0 {
		width = 80
	}

	return width
}

func (m *Model) viewportHeight() int {
	height := m.content.Height
	if height == 0 {
		height = 24
	}

	return height
}

func (m *Model) vtermHeight() int {
	return m.maxHeight / 3
}

func (m *Model) setWindowSize(w, h int) {
	m.maxWidth = w
	m.maxHeight = h
	m.content.Width = m.maxWidth
	m.tape.SetWindowSize(w, h-m.chromeHeight)
	m.ui.SetWindowSize(w, h)
}

func fmtDuration(d time.Duration) string {
	days := int64(d.Hours()) / 24
	hours := int64(d.Hours()) % 24
	minutes := int64(d.Minutes()) % 60
	seconds := d.Seconds() - float64(86400*days) - float64(3600*hours) - float64(60*minutes)

	switch {
	case d < time.Minute:
		return fmt.Sprintf("%.2fs", seconds)
	case d < time.Hour:
		return fmt.Sprintf("%dm%.1fs", minutes, seconds)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm%.1fs", hours, minutes, seconds)
	default:
		return fmt.Sprintf("%dd%dh%dm%.1fs", days, hours, minutes, seconds)
	}
}

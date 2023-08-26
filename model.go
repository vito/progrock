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

	width, height int

	tmpl *template.Template
}

func NewUI(spinner ui.Spinner) *UI {
	ui := &UI{Spinner: spinner}
	ui.tmpl = template.New("ui").
		Funcs(termenv.TemplateFuncs(termenv.ANSI)).
		Funcs(template.FuncMap{
			"duration": func(dt time.Duration) string {
				prec := 1
				sec := dt.Seconds()
				if sec < 10 {
					prec = 2
				} else if sec < 100 {
					prec = 1
				}

				return fmt.Sprintf("%.[2]*[1]fs", dt.Seconds(), prec)
			},
			"bar": func(current, total int64) string {
				bar := progress.New(progress.WithSolidFill("2"))
				bar.Width = ui.width / 8
				bar.EmptyColor = "8"
				bar.ShowPercentage = false
				return bar.ViewAs(float64(current) / float64(total))
			},
			"words": func(s string) string {
				return strings.Join(strings.Fields(s), " ")
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

func (w *swappableWriter) Swap(to io.Writer) {
	w.Lock()
	w.override = to
	w.Unlock()
}

func init() {
}

func (w *swappableWriter) Restore() {
	w.Lock()
	w.override = nil
	w.Unlock()
}

func (w *swappableWriter) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	if w.override != nil {
		return w.override.Write(p)
	}
	return w.original.Write(p)
}

func (ui *UI) RenderLoop(interrupt context.CancelFunc, tape *Tape) (*tea.Program, func()) {
	// NOTE: establish color cache before we start consuming stdin
	out := termenv.NewOutput(os.Stderr, termenv.WithColorCache(true))

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}

	inR, inW := io.Pipe()
	sw := &swappableWriter{original: inW}
	go io.Copy(sw, os.Stdin)

	model := ui.newModel(tape, interrupt, sw)

	tape.setZoomHook(func(st *zoomedState) {
		if st == nil {
			sw.Restore()
		} else {
			sw.Swap(st.in)
		}
	})

	opts := []tea.ProgramOption{
		tea.WithMouseCellMotion(),
		tea.WithInput(inR),
		tea.WithOutput(out),
	}

	prog := tea.NewProgram(model, opts...)

	displaying := new(sync.WaitGroup)
	displaying.Add(1)
	go func() {
		defer interrupt() // TODO this feels a little backwards
		defer displaying.Done()
		_, err := prog.Run()
		if err != nil {
			fmt.Fprintf(out, "%s\n", termenv.String(fmt.Sprintf("display error: %s", err)).Foreground(termenv.ANSIRed))
		}
	}()

	return prog, func() {
		prog.Send(EndMsg{})
		displaying.Wait()
		_ = term.Restore(int(os.Stdin.Fd()), oldState) // Best effort.
		model.Print(os.Stderr)
		model.PrintTrailer(os.Stderr)
	}
}

func (ui *UI) newModel(tape *Tape, interrupt context.CancelFunc, sw *swappableWriter) *Model {
	return &Model{
		sw: sw,

		tape: tape,
		ui:   ui,

		interrupt: interrupt,

		fps: 10,

		// sane defaults before we have the real window size
		maxWidth:  80,
		maxHeight: 24,
		viewport:  viewport.New(80, 24),

		contentBuf: new(bytes.Buffer),
		chromeBuf:  new(bytes.Buffer),
	}
}

type Model struct {
	sw *swappableWriter

	tape *Tape

	ui *UI

	interrupt func()

	chrome       string
	chromeHeight int

	viewport      viewport.Model
	maxWidth      int
	maxHeight     int
	contentHeight int

	// buffers for async rendering so we're not constantly reallocating
	chromeBuf  *bytes.Buffer
	contentBuf *bytes.Buffer

	statusInfos []StatusInfo

	// UI refresh rate
	fps float64

	finished bool
}

type StatusInfo struct {
	Name  string
	Value string
	Order int
}

type StatusInfoMsg StatusInfo

func (m *Model) Print(w io.Writer) {
	if err := m.tape.Render(w, m.ui); err != nil {
		fmt.Fprintln(w, "failed to render tape:", err)
		return
	}
}

func (m *Model) PrintTrailer(w io.Writer) {
	if err := m.ui.RenderStatus(w, m.tape, m.statusInfos); err != nil {
		fmt.Fprintln(w, "failed to render trailer:", err)
		return
	}
	fmt.Fprintln(w)
}

type EndMsg struct{}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		ui.Frame(m.fps),
		m.ui.Spinner.Init(),
	)
}

func (m *Model) viewportWidth() int {
	width := m.viewport.Width
	if width == 0 {
		width = 80
	}

	return width
}

func (m *Model) viewportHeight() int {
	height := m.viewport.Height
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
	m.viewport.Width = m.maxWidth
	m.tape.SetWindowSize(w, h-m.chromeHeight)
	m.ui.SetWindowSize(w, h)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, ui.Keys.Quit):
			// don't tea.Quit, let the UI finish
			m.interrupt()
		}

		s, cmd := m.ui.Spinner.Update(msg)
		m.ui.Spinner = s.(ui.Spinner)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.setWindowSize(msg.Width, msg.Height)

	case StatusInfoMsg:
		infos := append([]StatusInfo{}, m.statusInfos...)
		infos = append(infos, StatusInfo(msg))
		sort.Slice(infos, func(i, j int) bool {
			if infos[i].Order == infos[j].Order {
				return infos[i].Name < infos[j].Name
			}
			return infos[i].Order < infos[j].Order
		})
		m.statusInfos = infos

	case EndMsg:
		m.finished = true
		// m.render()
		cmds = append(cmds, tea.Quit)

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

		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
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

	atBottom := m.viewport.AtBottom()

	m.viewport.SetContent(content)

	max := m.maxHeight - m.chromeHeight
	m.viewport.Height = m.contentHeight
	if m.viewport.Height+m.chromeHeight > m.maxHeight {
		m.viewport.Height = max
	}

	if atBottom {
		m.viewport.GotoBottom()
	}
}

func (m *Model) viewChrome() string {
	m.chromeBuf.Reset()
	m.ui.RenderStatus(m.chromeBuf, m.tape, m.statusInfos)

	return lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		lipgloss.NewStyle().
			MaxWidth(m.maxWidth).
			Render(m.chromeBuf.String()),
	)
}

func (m *Model) View() string {
	if m.finished {
		// print nothing on exit; the outer render loop will call Print one last
		// time, otherwise bubbletea crops out the lines that go offscreen
		return ""
	}

	output := m.viewport.View()
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

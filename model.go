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

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/vito/progrock/tmpl"
	"github.com/vito/progrock/ui"
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

func (u *UI) RenderStatus(w io.Writer, tape *Tape, infos []StatusInfo, helpView string) error {
	spinner, _, _ := u.Spinner.ViewFrame(ui.DotFrames)
	return u.tmpl.Lookup("status.tmpl").Execute(w, struct {
		Spinner      string
		VertexSymbol string
		Tape         *Tape
		Infos        []StatusInfo
		Help         string
	}{
		Spinner:      spinner,
		VertexSymbol: block,
		Tape:         tape,
		Infos:        infos,
		Help:         helpView,
	})
}

func (ui *UI) RenderLoop(interrupt context.CancelFunc, tape *Tape, w io.Writer, tui bool) (*tea.Program, func()) {
	model := ui.NewModel(tape, interrupt, w)

	opts := []tea.ProgramOption{tea.WithOutput(w)}

	if tui {
		opts = append(opts, tea.WithMouseCellMotion())
	} else {
		opts = append(opts, tea.WithInput(new(bytes.Buffer)), tea.WithoutRenderer())
	}

	prog := tea.NewProgram(model, opts...)

	displaying := new(sync.WaitGroup)
	displaying.Add(1)
	go func() {
		defer displaying.Done()
		_, err := prog.Run()
		if err != nil {
			fmt.Fprintf(w, "%s\n", termenv.String(fmt.Sprintf("display error: %s", err)).Foreground(termenv.ANSIRed))
		}
	}()

	return prog, func() {
		prog.Send(EndMsg{})
		displaying.Wait()
		model.Print(os.Stderr)
		model.PrintTrailer(os.Stderr)
	}
}

func (ui *UI) NewModel(tape *Tape, interrupt context.CancelFunc, w io.Writer) *Model {
	helpModel := help.New()
	helpModel.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(termenv.ANSIBrightBlack))
	helpModel.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(termenv.ANSIBrightBlack))
	helpModel.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(termenv.ANSIBrightBlack))
	helpModel.Styles.Ellipsis = helpModel.Styles.ShortSeparator.Copy()
	helpModel.Styles.FullKey = helpModel.Styles.ShortKey.Copy()
	helpModel.Styles.FullDesc = helpModel.Styles.ShortDesc.Copy()
	helpModel.Styles.FullSeparator = helpModel.Styles.ShortSeparator.Copy()

	return &Model{
		tape: tape,
		ui:   ui,

		interrupt: interrupt,

		fps: 10,

		// sane defaults before we have the real window size
		maxWidth:  80,
		maxHeight: 24,
		viewport:  viewport.New(80, 24),

		help: helpModel,
	}
}

type Model struct {
	tape *Tape

	ui *UI

	interrupt func()

	viewport      viewport.Model
	maxWidth      int
	maxHeight     int
	contentHeight int

	statusInfos []StatusInfo

	// UI refresh rate
	fps float64

	finished bool

	help help.Model
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
	if err := m.ui.RenderStatus(w, m.tape, m.statusInfos, ""); err != nil {
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

// SetWindowSize is exposed so that the UI can be manually driven.
func (m *Model) SetWindowSize(w, h int) {
	m.maxWidth = w
	m.maxHeight = h
	m.viewport.Width = m.maxWidth
	m.help.Width = m.maxWidth / 2
	m.tape.SetWindowSize(w, h)
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
		case key.Matches(msg, ui.Keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}

		s, cmd := m.ui.Spinner.Update(msg)
		m.ui.Spinner = s.(ui.Spinner)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.SetWindowSize(msg.Width, msg.Height)

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
	buf := new(bytes.Buffer)
	m.Print(buf)

	content := strings.TrimRight(buf.String(), "\n")

	atBottom := m.viewport.AtBottom()

	m.viewport.SetContent(content)
	m.contentHeight = lipgloss.Height(content)

	if atBottom {
		m.viewport.GotoBottom()
	}
}

func (m *Model) View() string {
	if m.finished {
		// print nothing on exit; the outer render loop will call Print one last
		// time, otherwise bubbletea crops out the lines that go offscreen
		return ""
	}

	helpView := m.help.View(ui.Keys)

	helpSep := " "
	widthMinusHelp := m.maxWidth - lipgloss.Width(helpView)
	widthMinusHelp -= len(helpSep)

	statusBuf := new(bytes.Buffer)
	m.ui.RenderStatus(statusBuf, m.tape, m.statusInfos, helpView)

	footer := lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		lipgloss.NewStyle().
			MaxWidth(widthMinusHelp).
			Render(statusBuf.String()),
	)

	chromeHeight := lipgloss.Height(footer)

	max := m.maxHeight - chromeHeight
	m.viewport.Height = m.contentHeight
	if m.viewport.Height+chromeHeight > m.maxHeight {
		m.viewport.Height = max
	}

	output := m.viewport.View()
	if output == "\n" {
		output = ""
	}
	if output == "" {
		return footer
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		output,
		footer,
	)
}

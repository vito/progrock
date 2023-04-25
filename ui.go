package progrock

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
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
	"github.com/opencontainers/go-digest"
	"github.com/vito/progrock/tmpl"
	"github.com/vito/progrock/ui"
)

type UI struct {
	Spinner ui.Spinner

	Logs map[digest.Digest]*ui.Vterm

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

func (ui *UI) RenderTask(w io.Writer, v *VertexTask) error {
	return ui.tmpl.Lookup("task.tmpl").Execute(w, v)
}

func (ui *UI) RenderTerm(w io.Writer, term *ui.Vterm) error {
	_, err := fmt.Fprint(w, term.View())
	return err
}

func (u *UI) RenderStatus(w io.Writer, casette *Casette) error {
	return u.tmpl.Lookup("status.tmpl").Execute(w, struct {
		Spinner ui.Spinner
		Casette *Casette
	}{
		Spinner: u.Spinner,
		Casette: casette,
	})
}

func (ui *UI) RenderLoop(interrupt context.CancelFunc, casette *Casette, w io.Writer, tui bool) func() {
	model := ui.NewModel(casette, interrupt, w)

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
		err := prog.Start()
		if err != nil {
			fmt.Fprintf(w, "%s\n", termenv.String(fmt.Sprintf("display error: %s", err)).Foreground(termenv.ANSIRed))
		}
	}()

	return func() {
		prog.Send(EndMsg{})
		displaying.Wait()
		model.Print(os.Stderr)
	}
}

func (ui *UI) NewModel(casette *Casette, interrupt context.CancelFunc, w io.Writer) *Model {
	return &Model{
		casette: casette,
		ui:      ui,

		interrupt: interrupt,

		fps: 10,

		// sane defaults before we have the real window size
		maxWidth:  80,
		maxHeight: 24,
		viewport:  viewport.New(80, 24),

		help: help.New(),
	}
}

type Model struct {
	casette *Casette

	ui *UI

	interrupt func()

	viewport      viewport.Model
	maxWidth      int
	maxHeight     int
	contentHeight int

	// UI refresh rate
	fps float64

	finished bool

	help help.Model
}

func (model *Model) Print(w io.Writer) {
	if err := model.casette.Render(w, model.ui); err != nil {
		fmt.Fprintln(w, "failed to render graph:", err)
		return
	}
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
	m.casette.SetWindowSize(w, h)
	m.ui.SetWindowSize(w, h)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, ui.Keys.Quit):
			// don't tea.Quit, let the UI finish
			m.interrupt()
		case key.Matches(msg, ui.Keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}

		m.ui.Spinner, cmd = m.ui.Spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.SetWindowSize(msg.Width, msg.Height)

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
		m.ui.Spinner, cmd = m.ui.Spinner.Update(msg)
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

	buf := new(bytes.Buffer)
	m.ui.RenderStatus(buf, m.casette)

	footer := lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		lipgloss.NewStyle().
			MaxWidth(widthMinusHelp).
			Render(buf.String()),
		helpSep,
		helpView,
	)

	chromeHeight := lipgloss.Height(footer)

	max := m.maxHeight - chromeHeight
	m.viewport.Height = m.contentHeight
	if m.viewport.Height+chromeHeight > m.maxHeight {
		m.viewport.Height = max
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		footer,
	)
}

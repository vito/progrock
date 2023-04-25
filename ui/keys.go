package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Rave         key.Binding
	EndRave      key.Binding
	ForwardRave  key.Binding
	BackwardRave key.Binding
	Debug        key.Binding
	Help         key.Binding
	Quit         key.Binding

	Up   key.Binding
	Down key.Binding

	Home, End        key.Binding
	PageUp, PageDown key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit, k.Debug},
		{k.Rave, k.EndRave, k.ForwardRave, k.BackwardRave},
	}
}

var Keys = KeyMap{
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	// NB: this isn't rave-specific; one key to debug them all
	Debug: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "debug ui"),
	),
	Rave: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "rave"),
	),
	EndRave: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "end rave"),
	),
	ForwardRave: key.NewBinding(
		key.WithKeys("+", "="),
		key.WithHelp("+/=", "seek forward"),
	),
	BackwardRave: key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "seek backward"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Home: key.NewBinding(
		key.WithKeys("home"),
		key.WithHelp("home", "go to top"),
	),
	End: key.NewBinding(
		key.WithKeys("end"),
		key.WithHelp("end", "go to bottom"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "page down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
}

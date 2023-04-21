package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

type Components struct {
	ConsoleRunning, ConsoleDone        string
	ConsoleLogFormat                   string
	ConsoleVertexRunning               string
	ConsoleVertexCanceled              string
	ConsoleVertexErrored               string
	ConsoleVertexCached                string
	ConsoleVertexDone                  string
	ConsoleVertexStatus                string
	ConsoleVertexStatusProgressBound   string
	ConsoleVertexStatusProgressUnbound string

	TextContextSwitched             string
	TextLogFormat                   string
	TextVertexRunning               string
	TextVertexCanceled              string
	TextVertexErrored               string
	TextVertexCached                string
	TextVertexDone                  string
	TextVertexDoneDuration          string
	TextVertexStatus                string
	TextVertexStatusDuration        string
	TextVertexStatusProgressBound   string
	TextVertexStatusProgressUnbound string

	RunningDuration, DoneDuration string

	ErrorHeader, ErrorFooter string
	ErrorLogFormat           string

	Spinner tea.Model
}

var vertexID = termenv.String("%d:").Foreground(termenv.ANSIMagenta).String()

var Default = Components{
	ConsoleRunning: "Building %s (%d/%d)",
	ConsoleDone:    "Building %s (%d/%d) " + termenv.String("done").Foreground(termenv.ANSIGreen).String(),

	ConsoleLogFormat:                   " " + termenv.String("▕").Foreground(termenv.ANSIBrightBlack).String() + " %s",
	ConsoleVertexRunning:               termenv.String("=> %s").Foreground(termenv.ANSIYellow).String(),
	ConsoleVertexCanceled:              termenv.String("=> %s CANCELED").Foreground(termenv.ANSIYellow).String(),
	ConsoleVertexErrored:               termenv.String("=> %s ERROR").Foreground(termenv.ANSIRed).String(),
	ConsoleVertexCached:                termenv.String("=> %s CACHED").Foreground(termenv.ANSIBlue).String(),
	ConsoleVertexDone:                  termenv.String("=> %s").Foreground(termenv.ANSIGreen).String(),
	ConsoleVertexStatus:                "-> %s",
	ConsoleVertexStatusProgressBound:   "%.2f / %.2f",
	ConsoleVertexStatusProgressUnbound: "%.2f",

	TextLogFormat:                   vertexID + " %s %s",
	TextContextSwitched:             vertexID + " ...\n",
	TextVertexRunning:               vertexID + " %s",
	TextVertexCanceled:              vertexID + " %s " + termenv.String("CANCELED").Foreground(termenv.ANSIYellow).String(),
	TextVertexErrored:               vertexID + " %s " + termenv.String("ERROR: %s").Foreground(termenv.ANSIRed).String(),
	TextVertexCached:                vertexID + " %s " + termenv.String("CACHED").Foreground(termenv.ANSICyan).String(),
	TextVertexDone:                  vertexID + " %s " + termenv.String("DONE").Foreground(termenv.ANSIGreen).String(),
	TextVertexStatus:                vertexID + " %[3]s %[2]s",
	TextVertexStatusProgressBound:   "%.2f / %.2f",
	TextVertexStatusProgressUnbound: "%.2f",
	TextVertexStatusDuration:        "%.1fs",

	RunningDuration: "[%.[2]*[1]fs]",
	DoneDuration:    termenv.String("[%.[2]*[1]fs]").Faint().String(),

	ErrorLogFormat: "  " + termenv.String("!").Foreground(termenv.ANSIYellow).String() + " %[2]s %[3]s",
	ErrorHeader:    termenv.String("!!!").Foreground(termenv.ANSIYellow).String() + " " + termenv.String("%s").Foreground(termenv.ANSIRed).String() + "\n",
	ErrorFooter:    "",

	Spinner: NewRave(),
}

package ui

import "github.com/morikuni/aec"

type Components struct {
	ConsolePhase                       string
	ConsoleRunning, ConsoleDone        string
	ConsoleTermPrefix                  string
	ConsoleVertexRunning               string
	ConsoleVertexRunningDuration       string
	ConsoleVertexCanceled              string
	ConsoleVertexErrored               string
	ConsoleVertexCached                string
	ConsoleVertexDone                  string
	ConsoleVertexDoneDuration          string
	ConsoleVertexStatus                string
	ConsoleVertexStatusProgressBound   string
	ConsoleVertexStatusProgressUnbound string

	TextContextSwitched             string
	TextLogFormat                   string
	TextVertexCanceled              string
	TextVertexErrored               string
	TextVertexCached                string
	TextVertexDone                  string
	TextVertexDoneDuration          string
	TextVertexStatusDuration        string
	TextVertexStatusProgressBound   string
	TextVertexStatusProgressUnbound string
	TextVertexStatusComplete        string

	LogTimingFormat, LogFormat string
	ErrorHeader, ErrorFooter   string
}

var vertexID = aec.MagentaF.Apply("%d:")

var Default = Components{
	ConsolePhase:      "Building",
	ConsoleTermPrefix: "   | ",
	ConsoleRunning:    "%s %.1fs (%d/%d)",
	ConsoleDone:       "%s %.1fs (%d/%d) " + aec.GreenF.Apply("done"),

	ConsoleVertexRunning:               aec.YellowF.Apply("=> %s"),
	ConsoleVertexCanceled:              aec.YellowF.Apply("=> %s CANCELED"),
	ConsoleVertexErrored:               aec.RedF.Apply("=> %s ERROR"),
	ConsoleVertexCached:                aec.BlueF.Apply("=> %s CACHED"),
	ConsoleVertexDone:                  aec.GreenF.Apply("=> %s"),
	ConsoleVertexRunningDuration:       "[%s]",
	ConsoleVertexDoneDuration:          "[%s]",
	ConsoleVertexStatus:                "   => %s",
	ConsoleVertexStatusProgressBound:   "%.2f / %.2f",
	ConsoleVertexStatusProgressUnbound: "%.2f",

	TextContextSwitched:             vertexID + " ...\n",
	TextLogFormat:                   vertexID + " %s\n",
	TextVertexCanceled:              vertexID + " " + aec.YellowF.Apply("CANCELED") + "\n",
	TextVertexErrored:               vertexID + " " + aec.RedF.Apply("ERROR: %s") + "\n",
	TextVertexCached:                vertexID + " " + aec.CyanF.Apply("CACHED") + "\n",
	TextVertexDone:                  vertexID + " " + aec.GreenF.Apply("DONE") + "\n",
	TextVertexDoneDuration:          vertexID + " " + aec.GreenF.Apply("DONE") + " %.1fs\n",
	TextVertexStatusProgressBound:   "%.2f / %.2f",
	TextVertexStatusProgressUnbound: "%.2f",
	TextVertexStatusDuration:        "%.1fs",
	TextVertexStatusComplete:        aec.GreenF.Apply("done"),

	LogTimingFormat: "[%.[2]*[1]f]",
	LogFormat:       vertexID + " %s %s",
	ErrorHeader:     "------\n > %s:\n",
	ErrorFooter:     "------\n",
}

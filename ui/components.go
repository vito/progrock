package ui

import "github.com/morikuni/aec"

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
}

var vertexID = aec.MagentaF.Apply("%d:")

var Default = Components{
	ConsoleRunning: "Building %s (%d/%d)",
	ConsoleDone:    "Building %s (%d/%d) " + aec.GreenF.Apply("done"),

	ConsoleLogFormat:                   " " + aec.LightBlackF.Apply("▕") + " %s",
	ConsoleVertexRunning:               aec.YellowF.Apply("=> %s"),
	ConsoleVertexCanceled:              aec.YellowF.Apply("=> %s CANCELED"),
	ConsoleVertexErrored:               aec.RedF.Apply("=> %s ERROR"),
	ConsoleVertexCached:                aec.BlueF.Apply("=> %s CACHED"),
	ConsoleVertexDone:                  aec.GreenF.Apply("=> %s"),
	ConsoleVertexStatus:                "-> %s",
	ConsoleVertexStatusProgressBound:   "%.2f / %.2f",
	ConsoleVertexStatusProgressUnbound: "%.2f",

	TextLogFormat:                   vertexID + " %s %s",
	TextContextSwitched:             vertexID + " ...\n",
	TextVertexRunning:               vertexID + " %s",
	TextVertexCanceled:              vertexID + " %s " + aec.YellowF.Apply("CANCELED"),
	TextVertexErrored:               vertexID + " %s " + aec.RedF.Apply("ERROR: %s"),
	TextVertexCached:                vertexID + " %s " + aec.CyanF.Apply("CACHED"),
	TextVertexDone:                  vertexID + " %s " + aec.GreenF.Apply("DONE"),
	TextVertexStatus:                vertexID + " %[3]s %[2]s",
	TextVertexStatusProgressBound:   "%.2f / %.2f",
	TextVertexStatusProgressUnbound: "%.2f",
	TextVertexStatusDuration:        "%.1fs",

	RunningDuration: "[%.[2]*[1]fs]",
	DoneDuration:    aec.Faint.Apply("[%.[2]*[1]fs]"),

	ErrorLogFormat: "  " + aec.YellowF.Apply("!") + " %[2]s %[3]s",
	ErrorHeader:    aec.YellowF.Apply("!!!") + " " + aec.RedF.Apply("%s") + "\n",
	ErrorFooter:    "",
}

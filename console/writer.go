package console

import (
	"io"
	"sync"

	"github.com/jonboulle/clockwork"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
)

type Writer struct {
	clock clockwork.Clock
	ui    Components

	messageLevel progrock.MessageLevel
	showInternal bool

	trace *trace
	mux   *textMux
	l     sync.Mutex
}

type WriterOpt func(*Writer)

func WithClock(clock clockwork.Clock) WriterOpt {
	return func(w *Writer) {
		w.clock = clock
	}
}

func WithUI(ui Components) WriterOpt {
	return func(w *Writer) {
		w.ui = ui
	}
}

func ShowInternal(show bool) WriterOpt {
	return func(w *Writer) {
		w.showInternal = show
	}
}

func WithMessageLevel(level progrock.MessageLevel) WriterOpt {
	return func(w *Writer) {
		w.messageLevel = level
	}
}

func NewWriter(w io.Writer, opts ...WriterOpt) progrock.Writer {
	out := ui.NewOutput(w)

	progW := &Writer{
		clock:        clockwork.NewRealClock(),
		ui:           DefaultUI(out),
		showInternal: false,
		messageLevel: progrock.MessageLevel_WARNING,
	}

	for _, opt := range opts {
		opt(progW)
	}

	progW.trace = newTrace(progW.ui, progW.clock)
	progW.mux = &textMux{
		w:            out,
		ui:           progW.ui,
		showInternal: progW.showInternal,
		messageLevel: progW.messageLevel,
	}

	return progW
}

func (w *Writer) WriteStatus(status *progrock.StatusUpdate) error {
	w.l.Lock()
	defer w.l.Unlock()

	w.trace.update(status)
	w.mux.print(w.trace)
	w.mux.printMessages(status.Messages)

	return nil
}

func (w *Writer) Close() error {
	w.mux.print(w.trace)
	return nil
}

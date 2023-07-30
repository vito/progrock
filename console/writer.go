package console

import (
	"io"
	"sync"

	"github.com/jonboulle/clockwork"
	"github.com/vito/progrock"
)

type Writer struct {
	clock        clockwork.Clock
	ui           Components
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

func NewWriter(dest io.Writer, opts ...WriterOpt) progrock.Writer {
	w := &Writer{
		clock:        clockwork.NewRealClock(),
		ui:           DefaultUI,
		showInternal: false,
	}

	for _, opt := range opts {
		opt(w)
	}

	w.trace = newTrace(w.ui, w.clock)
	w.mux = &textMux{w: dest, ui: w.ui, showInternal: w.showInternal}

	return w
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

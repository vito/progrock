package progrock

import (
	"sync"

	"github.com/vito/progrock/graph"
)

// PipeBuffer is the number of writes to allow before a read must occur.
//
// This value is arbitrary. I don't want to leak this implementation detail to
// the API because it feels like a bit of a smell that this is even necessary,
// and perhaps the rest of the API can be refactored instead.
//
// The main idea is to be nonzero to prevent any weird deadlocks that occur
// around initialization/exit time due to an error in an inopportune moment.
// I'm tired of chasing down deadlocks and this seems like a low risk band-aid.
// Refactors welcome.
const PipeBuffer = 100

func Pipe() (ChanReader, Writer) {
	ch := make(chan *graph.SolveStatus, PipeBuffer)
	return ch, &ChanWriter{ch: ch}
}

type ChanWriter struct {
	ch chan<- *graph.SolveStatus

	sync.Mutex
}

func (doc *ChanWriter) WriteStatus(v *graph.SolveStatus) {
	doc.Lock()
	defer doc.Unlock()

	if doc.ch == nil {
		// discard
		return
	}

	doc.ch <- v
}

func (doc *ChanWriter) Close() {
	doc.Lock()
	if doc.ch != nil {
		close(doc.ch)
		doc.ch = nil
	}
	doc.Unlock()
}

type ChanReader <-chan *graph.SolveStatus

func (ch ChanReader) ReadStatus() (*graph.SolveStatus, bool) {
	val, ok := <-ch
	return val, ok
}

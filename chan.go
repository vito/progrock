package progrock

import (
	"sync"

	"github.com/vito/progrock/graph"
)

func Pipe() (ChanReader, Writer) {
	ch := make(chan *graph.SolveStatus)
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

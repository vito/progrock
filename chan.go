package progrock

import (
	"sync"
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

func BlockingPipe() (Reader, Writer) {
	ch := make(chan *StatusUpdate, PipeBuffer)
	return ChanReader(ch), &ChanWriter{ch: ch}
}

type ChanWriter struct {
	ch chan<- *StatusUpdate

	sync.Mutex
}

func (doc *ChanWriter) WriteStatus(v *StatusUpdate) error {
	doc.Lock()
	defer doc.Unlock()

	if doc.ch == nil {
		// discard
		return nil
	}

	doc.ch <- v
	return nil
}

func (doc *ChanWriter) Close() error {
	doc.Lock()
	if doc.ch != nil {
		close(doc.ch)
		doc.ch = nil
	}
	doc.Unlock()
	return nil
}

type ChanReader <-chan *StatusUpdate

var _ Reader = ChanReader(nil)

func (ch ChanReader) ReadStatus() (*StatusUpdate, bool) {
	val, ok := <-ch
	return val, ok
}

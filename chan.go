package progrock

import "github.com/vito/progrock/graph"

func Pipe() (ChanReader, ChanWriter) {
	ch := make(chan *graph.SolveStatus)
	return ch, ch
}

type ChanWriter chan<- *graph.SolveStatus

func (ch ChanWriter) WriteStatus(status *graph.SolveStatus) {
	ch <- status
}

func (ch ChanWriter) Close() {
	close(ch)
}

type ChanReader <-chan *graph.SolveStatus

func (ch ChanReader) ReadStatus() (*graph.SolveStatus, bool) {
	val, ok := <-ch
	return val, ok
}

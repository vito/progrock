package progrock

import (
	"github.com/vito/progrock/graph"
)

type MultiWriter []Writer

func (mw MultiWriter) WriteStatus(v *graph.SolveStatus) {
	for _, w := range mw {
		w.WriteStatus(v)
	}
}

func (mw MultiWriter) Close() {
	for _, w := range mw {
		w.Close()
	}
}

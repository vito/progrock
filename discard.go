package progrock

import "github.com/vito/progrock/graph"

type Discard struct{}

func (Discard) WriteStatus(*graph.SolveStatus) {}
func (Discard) Close()                         {}

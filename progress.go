package progrock

import (
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func (vertex *Vertex) Duration() time.Duration {
	return dt(vertex.Started, vertex.Completed)
}

func (task *VertexTask) Duration() time.Duration {
	return dt(task.Started, task.Completed)
}

func dt(started, completed *timestamppb.Timestamp) time.Duration {
	if started == nil {
		return 0
	}

	var end time.Time
	if completed != nil {
		end = completed.AsTime()
	} else {
		end = time.Now()
	}

	return end.Sub(started.AsTime())
}

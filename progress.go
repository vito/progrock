package progrock

import (
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

//go:generate protoc -I=./ --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative progress.proto

// VertexInstance identifies a vertex at a specific point in time.
type VertexInstance struct {
	VertexId string
	GroupId  string
}

func (v *Vertex) HasInput(o *Vertex) bool {
	for _, oid := range o.Outputs {
		if oid == v.Id {
			return true
		}
		for _, iid := range v.Inputs {
			if iid == oid {
				return true
			}
		}
	}
	for _, id := range v.Inputs {
		if id == o.Id {
			return true
		}
	}
	return false
}

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
		end = Clock.Now()
	}

	return end.Sub(started.AsTime())
}

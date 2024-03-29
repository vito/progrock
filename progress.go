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

func (v *Vertex) Label(name string) string {
	for _, label := range v.Labels {
		if label.Name == name {
			return label.Value
		}
	}
	return ""
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
	return Duration(vertex.Started, vertex.Completed)
}

func (task *VertexTask) Duration() time.Duration {
	return Duration(task.Started, task.Completed)
}

func Duration(started, completed *timestamppb.Timestamp) time.Duration {
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

package progrock

import (
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// VertexInstance identifies a vertex at a specific point in time.
type VertexInstance struct {
	VertexId string
	GroupId  string
}

func (v *Vertex) Instance() VertexInstance {
	return VertexInstance{
		VertexId: v.Id,
		GroupId:  v.GetGroup(),
	}
}

func (v *Vertex) IsInGroup(g *Group) bool {
	return v.GetGroup() == g.Id
}

func (v *Vertex) IsInGroupOrParent(g *Group, allGroups map[string]*Group) bool {
	if v.GetGroup() == g.Id {
		return true
	}
	for vg := v.GetGroup(); vg != ""; vg = allGroups[vg].GetParent() {
		if vg == g.Id {
			return true
		}
	}
	return false
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

func (v *Vertex) IsSibling(o *Vertex) bool {
	return v.GetGroup() == o.GetGroup()
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
		end = time.Now()
	}

	return end.Sub(started.AsTime())
}

package progrock

import (
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func (v *Vertex) IsInGroup(g *Group) bool {
	for _, id := range v.Groups {
		if id == g.Id {
			return true
		}
	}
	return false
}

func (v *Vertex) IsInGroupOrParent(g *Group, allGroups map[string]*Group) bool {
	for _, id := range v.Groups {
		if id == g.Id {
			return true
		}
		for vg := id; vg != ""; vg = allGroups[vg].GetParent() {
			if vg == g.Id {
				return true
			}
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
	for _, id := range v.Groups {
		for _, oid := range o.Groups {
			if id == oid {
				return true
			}
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
		end = time.Now()
	}

	return end.Sub(started.AsTime())
}

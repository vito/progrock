package progrock

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/muesli/termenv"
	"github.com/vito/dagql/idproto"
	"github.com/vito/progrock/ui"
)

func RenderIDs(tape *Tape, w io.Writer, u *UI) error {
	leafIDs := make(map[string]*idproto.ID)
	for vid, id := range tape.allIDs {
		leafIDs[vid] = id
	}

	for _, idp := range tape.allIDs {
		for parent := idp.Parent; parent != nil; parent = parent.Parent {
			pdig, err := parent.Digest()
			if err != nil {
				return fmt.Errorf("digest parent: %w", err)
			}
			delete(leafIDs, pdig.String())
		}
		for _, arg := range idp.Args {
			switch x := arg.Value.GetValue().(type) {
			case *idproto.Literal_Id:
				adig, err := x.Id.Canonical().Digest()
				if err != nil {
					return fmt.Errorf("digest parent: %w", err)
				}
				delete(leafIDs, adig.String())
			default:
			}
		}
	}

	var ids []vertexAndID
	for vid, id := range leafIDs {
		vtx := tape.vertexes[vid]
		if vtx == nil {
			continue
		}
		vtxAndID := vertexAndID{
			vtx: vtx,
			id:  id,
		}
		if id.Field == "id" {
			// skip selecting ID since that means it'll show up somewhere else that's
			// more interesting
			continue
		}
		// if vtxAndID.totalDuration(tape) < 100*time.Millisecond {
		// 	continue
		// }
		ids = append(ids, vtxAndID)
	}
	sort.Slice(ids, func(i, j int) bool {
		vi := ids[i].vtx
		vj := ids[j].vtx
		return vi.Started.AsTime().Before(vj.Started.AsTime())
	})

	out := ui.NewOutput(w, termenv.WithProfile(tape.colorProfile))

	for _, id := range ids {
		if err := renderID(tape, out, u, id.vtx, id.id, 0); err != nil {
			return err
		}
		fmt.Fprintln(out)
	}

	return nil
}

func DebugRenderID(out *termenv.Output, u *UI, id *idproto.ID, depth int) error {
	if id.Parent != nil {
		if err := DebugRenderID(out, u, id.Parent, depth); err != nil {
			return err
		}
	}

	indent := func() {
		fmt.Fprintf(out, strings.Repeat("  ", depth))
	}

	indent()

	fmt.Fprint(out, id.Field)

	kwColor := termenv.ANSIBlue

	if len(id.Args) > 0 {
		fmt.Fprint(out, "(")
		var needIndent bool
		for _, arg := range id.Args {
			if _, ok := arg.Value.ToInput().(*idproto.ID); ok {
				needIndent = true
				break
			}
		}
		if needIndent {
			fmt.Fprintln(out)
			depth++
			for _, arg := range id.Args {
				indent()
				fmt.Fprintf(out, out.String("%s:").Foreground(kwColor).String(), arg.Name)
				val := arg.Value.GetValue()
				switch x := val.(type) {
				case *idproto.Literal_Id:
					argVertexID, err := x.Id.Digest()
					if err != nil {
						return err
					}
					fmt.Fprintln(out, " "+x.Id.Type.ToAST().Name()+"@"+argVertexID.String()+"{")
					depth++
					if err := DebugRenderID(out, u, x.Id, depth); err != nil {
						return err
					}
					depth--
					indent()
					fmt.Fprintln(out, "}")
				default:
					fmt.Fprint(out, " ")
					renderLiteral(out, arg.Value)
					fmt.Fprintln(out)
				}
			}
			depth--
			indent()
		} else {
			for i, arg := range id.Args {
				if i > 0 {
					fmt.Fprint(out, ", ")
				}
				fmt.Fprintf(out, out.String("%s:").Foreground(kwColor).String()+" ", arg.Name)
				renderLiteral(out, arg.Value)
			}
		}
		fmt.Fprint(out, ")")
	}

	typeStr := out.String(": " + id.Type.ToAST().String()).Foreground(termenv.ANSIBrightBlack)
	fmt.Fprintln(out, typeStr)

	return nil
}

func renderID(tape *Tape, out *termenv.Output, u *UI, vtx *Vertex, id *idproto.ID, depth int) error {
	if id.Parent != nil {
		parentVtxID, err := id.Parent.Digest()
		if err != nil {
			return err
		}
		parentVtx := tape.vertexes[parentVtxID.String()]
		if err := renderID(tape, out, u, parentVtx, id.Parent, depth); err != nil {
			return err
		}
	}

	indent := func() {
		fmt.Fprintf(out, strings.Repeat("  ", depth))
	}

	indent()

	dig, err := id.Canonical().Digest()
	if err != nil {
		return err
	}

	// if id.Parent == nil {
	// 	fmt.Fprintln(out, out.String(dot+" id: "+dig.String()).Bold())
	// 	indent()
	// }

	if vtx != nil {
		renderStatus(out, u, vtx)
	}

	fmt.Fprint(out, id.Field)

	kwColor := termenv.ANSIBlue

	if len(id.Args) > 0 {
		fmt.Fprint(out, "(")
		var needIndent bool
		for _, arg := range id.Args {
			if _, ok := arg.Value.ToInput().(*idproto.ID); ok {
				needIndent = true
				break
			}
		}
		if needIndent {
			fmt.Fprintln(out)
			depth++
			depth++
			for _, arg := range id.Args {
				indent()
				fmt.Fprintf(out, out.String("%s:").Foreground(kwColor).String(), arg.Name)
				val := arg.Value.GetValue()
				switch x := val.(type) {
				case *idproto.Literal_Id:
					argVertexID, err := x.Id.Digest()
					if err != nil {
						return err
					}
					fmt.Fprintln(out, " "+x.Id.Type.ToAST().Name()+"@"+argVertexID.String()+"{")
					depth++
					argVtx := tape.vertexes[argVertexID.String()]
					if err := renderID(tape, out, u, argVtx, x.Id, depth); err != nil {
						return err
					}
					depth--
					indent()
					fmt.Fprintln(out, "}")
				default:
					fmt.Fprint(out, " ")
					renderLiteral(out, arg.Value)
					fmt.Fprintln(out)
				}
			}
			depth--
			indent()
			depth--
		} else {
			for i, arg := range id.Args {
				if i > 0 {
					fmt.Fprint(out, ", ")
				}
				fmt.Fprintf(out, out.String("%s:").Foreground(kwColor).String()+" ", arg.Name)
				renderLiteral(out, arg.Value)
			}
		}
		fmt.Fprint(out, ")")
	}

	typeStr := out.String(": " + id.Type.ToAST().String()).Foreground(termenv.ANSIBrightBlack)
	fmt.Fprintln(out, typeStr)

	if vtx != nil && (tape.closed || tape.showAllOutput || vtx.Completed == nil || vtx.Error != nil) {
		renderVertexTasksAndLogs(out, u, tape, vtx, depth)
	}

	if os.Getenv("SHOW_VERTICES") == "" {
		return nil
	}

	children := collectTransitiveChildren(tape, dig.String())
	sort.Slice(children, func(i, j int) bool {
		vi := children[i]
		vj := children[j]
		if vi.Cached && vj.Cached {
			return vi.Name < vj.Name
		}
		return vi.Started.AsTime().Before(vj.Started.AsTime())
	})

	depth++

	for _, vtx := range children {
		if vtx.Completed != nil && vtx.Error == nil {
			continue
		}

		indent()
		renderStatus(out, u, vtx)
		if err := u.RenderVertexTree(out, vtx); err != nil {
			return err
		}
		renderVertexTasksAndLogs(out, u, tape, vtx, depth)
	}

	return nil
}

func collectTransitiveChildren(tape *Tape, groupID string) []*Vertex {
	var children []*Vertex
	for id := range tape.group2vertexes[groupID] {
		child := tape.vertexes[id]
		if child == nil {
			continue
		}
		children = append(children, child)
	}
	for sub := range tape.group2groups[groupID] {
		children = append(children, collectTransitiveChildren(tape, sub)...)
	}
	return children
}

func rootOf(id *idproto.ID) *idproto.ID {
	if id.Parent == nil {
		return id
	}
	return rootOf(id.Parent)
}

type vertexAndID struct {
	vtx *Vertex
	id  *idproto.ID
}

func (vid vertexAndID) totalDuration(tape *Tape) time.Duration {
	root := rootOf(vid.id)
	rootVtxID, err := root.Digest()
	if err != nil {
		return 0
	}
	rootVtx := tape.vertexes[rootVtxID.String()]
	if rootVtx == nil {
		return 0
	}
	return dt(rootVtx.Started, vid.vtx.Completed)
}

func renderLiteral(out *termenv.Output, lit *idproto.Literal) {
	var color termenv.Color
	switch lit.GetValue().(type) {
	case *idproto.Literal_Bool:
		color = termenv.ANSIBrightRed
	case *idproto.Literal_Int:
		color = termenv.ANSIRed
	case *idproto.Literal_Float:
		color = termenv.ANSIRed
	case *idproto.Literal_String_:
		color = termenv.ANSIYellow
	case *idproto.Literal_Id:
		color = termenv.ANSIYellow
	case *idproto.Literal_Enum:
		color = termenv.ANSIYellow
	case *idproto.Literal_Null:
		color = termenv.ANSIBrightBlack
	case *idproto.Literal_List:
		fmt.Fprint(out, "[")
		for i, item := range lit.GetList().Values {
			if i > 0 {
				fmt.Fprint(out, ", ")
			}
			renderLiteral(out, item)
		}
		fmt.Fprint(out, "]")
		return
	case *idproto.Literal_Object:
		fmt.Fprint(out, "{")
		for i, item := range lit.GetObject().Values {
			if i > 0 {
				fmt.Fprint(out, ", ")
			}
			fmt.Fprintf(out, "%s: ", item.GetName())
			renderLiteral(out, item.Value)
		}
		fmt.Fprint(out, "}")
		return
	}
	fmt.Fprint(out, out.String(lit.ToAST().String()).Foreground(color))
}

func renderStatus(out *termenv.Output, u *UI, vtx *Vertex) {
	var symbol string
	var color termenv.Color
	if vtx.Completed != nil {
		if vtx.Error != nil {
			symbol = iconFailure
			color = termenv.ANSIRed
		} else if vtx.Canceled {
			symbol = iconSkipped
			color = termenv.ANSIBrightBlack
		} else {
			symbol = iconSuccess
			color = termenv.ANSIGreen
		}
	} else {
		symbol, _, _ = u.Spinner.ViewFrame(ui.DotFrames)
		color = termenv.ANSIYellow
	}

	symbol = out.String(symbol).Foreground(color).String()

	fmt.Fprintf(out, "%s ", symbol) // NB: has to match indent level
}

func renderVertexTasksAndLogs(out *termenv.Output, u *UI, tape *Tape, vtx *Vertex, depth int) error {
	indent := func() {
		fmt.Fprintf(out, strings.Repeat("  ", depth))
	}

	tasks := tape.tasks[vtx.Id]
	for _, t := range tasks {
		if t.Completed != nil {
			continue
		}
		indent()
		fmt.Fprint(out, vrBar, " ")
		if err := u.RenderTask(out, t); err != nil {
			return err
		}
	}

	term := tape.vertexLogs(vtx.Id)

	if vtx.Error != nil {
		term.SetHeight(term.UsedHeight())
	} else {
		term.SetHeight(tape.termHeight)
	}

	term.SetPrefix(strings.Repeat("  ", depth) + vbBar + " ")

	if tape.closed {
		term.SetHeight(term.UsedHeight())
	}

	return u.RenderTerm(out, term)
}

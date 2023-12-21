package progrock

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/muesli/termenv"
	"github.com/vito/progrock/ui"
	"golang.org/x/exp/slices"
)

func RenderTree(tape *Tape, w io.Writer, u *UI) error {
	b := tape.bouncer()

	out := ui.NewOutput(w, termenv.WithProfile(tape.ColorProfile))

	var groups []*Group
	for _, group := range b.groups {
		groups = append(groups, group)
	}
	// TODO: (likely) stable order
	sort.Slice(groups, func(i, j int) bool {
		gi := groups[i]
		gj := groups[j]
		return gi.Started.AsTime().Before(gj.Started.AsTime())
	})

	renderVtx := func(vtx *Vertex, indent string) error {
		var symbol string
		var color termenv.Color
		if vtx.Completed != nil {
			if vtx.Error != nil {
				symbol = ui.IconFailure
				color = termenv.ANSIRed
			} else if vtx.Canceled {
				symbol = ui.IconSkipped
				color = termenv.ANSIBrightBlack
			} else {
				symbol = ui.IconSuccess
				color = termenv.ANSIGreen
			}
		} else {
			symbol, _, _ = u.Spinner.ViewFrame(ui.DotFrames)
			color = termenv.ANSIYellow
		}

		symbol = out.String(symbol).Foreground(color).String()

		fmt.Fprintf(w, "%s%s ", indent, symbol)

		if err := u.RenderVertexTree(w, vtx); err != nil {
			return err
		}

		// TODO dedup from renderGraph
		if tape.IsClosed || tape.ShowCompletedLogs || vtx.Completed == nil || vtx.Error != nil {
			tasks := tape.VertexTasks[vtx.Id]
			for _, t := range tasks {
				fmt.Fprint(w, indent, ui.VertRightBar, " ")
				if err := u.RenderTask(w, t); err != nil {
					return err
				}
			}

			term := tape.VertexLogs(vtx.Id)

			if vtx.Error != nil {
				term.SetHeight(term.UsedHeight())
			} else {
				term.SetHeight(tape.ReasonableTermHeight)
			}

			term.SetPrefix(indent + out.String(ui.VertBoldBar).Foreground(color).String() + " ")

			if tape.IsClosed {
				term.SetHeight(term.UsedHeight())
			}

			if err := u.RenderTerm(w, term); err != nil {
				return err
			}
		}

		return nil
	}

	// TODO: this ended up being super complicated after a lot of flailing and
	// can probably be simplified. the goal is to render the groups depth-first,
	// in a stable order. i tried harder and harder and it ended up being an
	// unrelated mutation bug. -_-
	rendered := map[string]struct{}{}
	var render func(g *Group) error
	render = func(g *Group) error {
		if _, f := rendered[g.Id]; f {
			return nil
		} else {
			rendered[g.Id] = struct{}{}
		}

		defer func() {
			for _, sub := range groups {
				if sub.Parent != nil && *sub.Parent == g.Id {
					render(sub)
				}
			}
		}()

		vs := b.VisibleVertices(g)
		if len(vs) == 0 {
			return nil
		}

		sort.Slice(vs, func(i, j int) bool {
			return slices.Index(tape.ChronologicalVertexIDs, vs[i].Id) < slices.Index(tape.ChronologicalVertexIDs, vs[j].Id)
		})

		depth := b.VisibleDepth(g) - 1
		if depth < 0 {
			depth = 0
		}
		indent := strings.Repeat("  ", depth)

		if g.Name != RootGroup {
			var names []string
			for _, ancestor := range b.HiddenAncestors(g) {
				if ancestor.Name == RootGroup {
					continue
				}

				names = append(names, ancestor.Name)
			}
			names = append(names, g.Name)
			groupPath := strings.Join(names, " "+ui.CaretRightFilled+" ")
			fmt.Fprintf(w, "%s%s %s\n", indent, ui.CaretRightFilled, out.String(groupPath).Bold())
			indent += "  "
		}

		for _, vtx := range vs {
			if err := renderVtx(vtx, indent); err != nil {
				return err
			}
		}

		return nil
	}

	for _, v := range b.VisibleVertices(nil) {
		if err := renderVtx(v, ""); err != nil {
			return err
		}
	}

	for _, g := range groups {
		if err := render(g); err != nil {
			return err
		}
	}

	return nil
}

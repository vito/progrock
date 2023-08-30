package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/opencontainers/go-digest"
	"github.com/vito/midterm"
	"github.com/vito/progrock"
)

func cmdVtx(ctx context.Context, rec *progrock.Recorder, exe string, args ...string) {
	cmdline := strings.Join(append([]string{exe}, args...), " ")

	okVtx := rec.Vertex(
		digest.FromString(fmt.Sprintf("%d", time.Now().UnixNano())),
		cmdline,
		progrock.Focused(),
	)

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdout = okVtx.Stdout()
	cmd.Stderr = okVtx.Stderr()
	okVtx.Done(cmd.Run())
}

var focus bool
var showInternal bool

func init() {
	flag.BoolVar(&focus, "focus", false, "focus mode")
	flag.BoolVar(&showInternal, "debug", false, "show internal vertices")
}

func main() {
	flag.Parse()

	tape := progrock.NewTape()
	tape.Focus(focus)
	tape.ShowInternal(showInternal)

	ctx := context.Background()
	rec := progrock.NewRecorder(tape)
	ctx = progrock.ToContext(ctx, rec)

	err := progrock.DefaultUI().Run(ctx, tape, demo)
	if err != nil {
		panic(err)
	}
}

func demo(ctx context.Context, ui progrock.UIClient) error {
	rec := progrock.FromContext(ctx)

	if true {
		cmdPane(rec, "htop")
		cmdPane(rec, "vim")

		go func() {
			for i := 1; i <= 5; i++ {
				time.Sleep(1 * time.Second)
				ui.SetStatusInfo(progrock.StatusInfo{
					Name:  fmt.Sprintf("More info %d", i),
					Value: "https://example.com",
				})
			}
		}()
	}

	ui.SetStatusInfo(progrock.StatusInfo{
		Name:  "Foo",
		Value: "abcdef",
		Order: 2,
	})

	ui.SetStatusInfo(progrock.StatusInfo{
		Name:  "Info",
		Value: "https://example.com",
		Order: 1,
	})

	failedVtx := rec.Vertex("failed", "failed task in vertex")
	// failedVtx.Task("errored task")
	fmt.Fprintln(failedVtx.Stderr(), "some failed task logs")
	failedVtx.Done(fmt.Errorf("oh noes"))

	cmdVtx(ctx, rec, "ls", "-al")
	cmdVtx(ctx, rec, "sh", "-c", "echo imma fail && echo any moment now && exit 1")

	wg := new(sync.WaitGroup)

dance:
	for v := 0; v < 20; v++ {
		v := v

		group := rec.WithGroup(fmt.Sprintf("group %d", v))

		succeeds := group.Vertex(
			digest.Digest(fmt.Sprintf("log-and-count-%d", v)),
			fmt.Sprintf("count and log: #%d", v+1),
		)

		fmt.Fprintf(succeeds.Stdout(), "group: %s\n", group.Group.Id)

		count := succeeds.Task("counting task")
		count.Start()

		prog := succeeds.ProgressTask(42, "barring task")
		prog.Start()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer prog.Done(nil)
			for i := int64(0); i <= prog.Task.Total; i++ {
				select {
				case <-time.After(100 * time.Millisecond):
				case <-ctx.Done():
					return
				}

				prog.Current(i)
			}
		}()

		subCtx, cancel := context.WithTimeout(ctx, time.Duration(v)*time.Second)

		multiGroup := group.Vertex(
			digest.Digest(fmt.Sprintf("log-and-count-%d", v%2)),
			fmt.Sprintf("count and log: %d", v%2),
		)

		wg.Add(1)
		go func() {
			defer cancel()
			defer wg.Done()

			time.Sleep(500 * time.Millisecond)

			for i := int64(0); subCtx.Err() == nil; i++ {
				if i%2 == 0 {
					fmt.Fprintf(succeeds.Stdout(), "stdout %d\n", i)
				} else {
					fmt.Fprintf(succeeds.Stderr(), "stderr %d\n", i)
				}

				select {
				case <-time.After(500 * time.Millisecond):
				case <-subCtx.Done():
				}
			}

			fmt.Fprintln(succeeds.Stdout(), "stdout done")
			fmt.Fprintln(succeeds.Stderr(), "stderr done")
			count.Complete()

			succeeds.Complete()
			multiGroup.Complete()
		}()

		if v%2 == 0 {
			subGroup := group.WithGroup(fmt.Sprintf("sub-group %d", v))
			go cmdVtx(subCtx, subGroup, "sh", "-c", "echo hello")
		}

		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			break dance
		}
	}

	wg.Wait()

	return rec.Close()
}

func cmdPane(rec *progrock.Recorder, exe string, args ...string) {
	cmd := exec.Command(exe, args...)

	name := strings.Join(append([]string{exe}, args...), " ")

	var vtx *progrock.VertexRecorder
	vtx = rec.Vertex(digest.FromString(name), name, progrock.Zoomed(func(term *midterm.Terminal) io.Writer {
		p, err := pty.StartWithSize(cmd, &pty.Winsize{
			Rows: uint16(term.Height),
			Cols: uint16(term.Width),
		})
		if err != nil {
			panic(err)
		}

		go func() {
			vtx.Done(cmd.Wait())
		}()

		term.ForwardRequests = os.Stdin
		term.ForwardResponses = p

		go io.Copy(term, p)

		term.OnResize(func(rows, cols int) {
			pty.Setsize(p, &pty.Winsize{
				Rows: uint16(rows),
				Cols: uint16(cols),
			})
		})

		return p
	}))
}

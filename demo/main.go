package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/vito/progrock"
)

func cmdVtx(ctx context.Context, rec *progrock.Recorder, exe string, args ...string) {
	cmdline := strings.Join(append([]string{exe}, args...), " ")

	okVtx := rec.Vertex(
		digest.FromString(fmt.Sprintf("%d", time.Now().UnixNano())),
		cmdline,
	)

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdout = okVtx.Stdout()
	cmd.Stderr = okVtx.Stderr()
	okVtx.Done(cmd.Run())
}

func main() {
	tape := progrock.NewTape()
	rec := progrock.NewRecorder(tape)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prog, stop := progrock.DefaultUI().RenderLoop(cancel, tape, os.Stderr, true)
	defer stop()

	prog.Send(progrock.StatusInfoMsg{
		Name:  "Foo",
		Value: "abcdef",
		Order: 2,
	})

	prog.Send(progrock.StatusInfoMsg{
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
	for v := 0; v < 10; v++ {
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

		subVtx, cancel := context.WithTimeout(ctx, time.Duration(v)*time.Second)

		multiGroup := group.Vertex(
			digest.Digest(fmt.Sprintf("log-and-count-%d", v%2)),
			fmt.Sprintf("count and log: %d", v%2),
		)

		wg.Add(1)
		go func() {
			defer cancel()
			defer wg.Done()

			time.Sleep(500 * time.Millisecond)

			for i := int64(0); subVtx.Err() == nil; i++ {
				if i%2 == 0 {
					fmt.Fprintf(succeeds.Stdout(), "stdout %d\n", i)
				} else {
					fmt.Fprintf(succeeds.Stderr(), "stderr %d\n", i)
				}

				select {
				case <-time.After(500 * time.Millisecond):
				case <-subVtx.Done():
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
			go cmdVtx(subVtx, subGroup, "sh", "-c", "echo hello")
		}

		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			break dance
		}
	}

	wg.Wait()

	rec.Close()
}

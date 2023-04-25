package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/vito/progrock"
)

func main() {
	casette := progrock.NewCasette()
	rec := progrock.NewRecorder(casette)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := progrock.DefaultUI().RenderLoop(cancel, casette, os.Stderr, true)
	defer stop()

	failed := rec.Vertex("failed vertex", "failed vertex")
	failed.Task("finished task").Complete()
	fmt.Fprintln(failed.Stderr(), "some failed vertex logs")
	fmt.Fprintln(failed.Stderr(), "some more failed vertex logs")
	fmt.Fprintln(failed.Stderr(), "even more failed vertex logs")
	failed.Done(fmt.Errorf("bam"))

	failedTask := rec.Vertex("failed", "failed task in vertex")
	failedTask.Task("errored task")
	fmt.Fprintln(failedTask.Stderr(), "some failed task logs")
	fmt.Fprintln(failedTask.Stderr(), "some more failed task logs")
	failedTask.Done(fmt.Errorf("oh noes"))

	wg := new(sync.WaitGroup)

dance:
	for v := 0; v < 30; v++ {
		v := v

		succeeds := rec.Vertex(
			digest.Digest(fmt.Sprintf("log-and-count-%d", v)),
			fmt.Sprintf("count and log: #%d", v+1),
		)

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
		}()

		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			break dance
		}
	}

	wg.Wait()

	rec.Close()
}

package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
)

func main() {
	r, w := progrock.Pipe()
	rec := progrock.NewRecorder(w)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec.Display(cancel, ui.Default, os.Stderr, r, true)
	defer rec.Stop()

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

	for v := 0; v < 3; v++ {
		v := v

		succeeds := rec.Vertex(
			digest.Digest(fmt.Sprintf("log-and-count-%d", v)),
			fmt.Sprintf("count and log: #%d", v+1),
		)

		count := succeeds.Task("counting task")
		count.Start()

		wg.Add(1)
		go func() {
			defer wg.Done()

			time.Sleep(500 * time.Millisecond)

			for i := int64(0); ctx.Err() == nil; i++ {
				fmt.Fprintf(succeeds.Stdout(), "stdout %d\n", i)
				fmt.Fprintf(succeeds.Stderr(), "stderr %d\n", i)

				select {
				case <-time.After(500 * time.Millisecond):
				case <-ctx.Done():
				}
			}

			fmt.Fprintln(succeeds.Stdout(), "done")
			fmt.Fprintln(succeeds.Stderr(), "done")
			count.Complete()

			succeeds.Complete()
		}()
	}

	wg.Wait()

	w.Close()
}

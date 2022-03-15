package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
)

func main() {
	r, w := progrock.Pipe()
	rec := progrock.NewRecorder(w)

	_, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	rec.Display(stop, ui.Default, os.Stderr, r, false)
	defer rec.Stop()

	failed := rec.Vertex("failed vertex", "failed vertex")
	fmt.Fprintln(failed.Stderr(), "some logs")
	fmt.Fprintln(failed.Stderr(), "some more logs")
	failed.Error(fmt.Errorf("bam"))

	failedTask := rec.Vertex("failed", "failed task in vertex")
	failedTask.Task("errored task")
	fmt.Fprintln(failedTask.Stderr(), "some logs")
	fmt.Fprintln(failedTask.Stderr(), "some more logs")
	failedTask.Error(fmt.Errorf("oh noes"))

	succeeds := rec.Vertex("log-and-count", "banana")

	time.Sleep(500 * time.Millisecond)

	succeeds.Task("finished task").Complete()

	count := succeeds.Task("counting task")
	count.Start()

	var total int64 = 10
	for i := int64(0); i < total; i++ {
		fmt.Fprintf(succeeds.Stdout(), "stdout %d\n", i)
		fmt.Fprintf(succeeds.Stderr(), "stderr %d\n", i)
		count.Progress(i, total)
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Fprintln(succeeds.Stdout(), "done")
	fmt.Fprintln(succeeds.Stderr(), "done")
	count.Progress(total, total)
	count.Complete()

	succeeds.Complete()

	w.Close()
}

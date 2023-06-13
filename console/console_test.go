package console_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/sebdah/goldie/v2"
	"github.com/vito/progrock"
	"github.com/vito/progrock/console"
)

func TestEmpty(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	progrock.NewRecorder(writer)

	testGolden(t, buf)
}

func TestVertexStart(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	rec := progrock.NewRecorder(writer)

	rec.Vertex("hey", "sup")

	testGolden(t, buf)
}

func TestVertexStartStdoutStderr(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	rec := progrock.NewRecorder(writer)

	vtx := rec.Vertex("hey", "sup")
	fmt.Fprintln(vtx.Stdout(), "hi stdout")
	fmt.Fprintln(vtx.Stderr(), "hi stderr")
	fmt.Fprintln(vtx.Stdout(), "hi again stdout")

	testGolden(t, buf)
}

func TestVertexStartStdoutStderrComplete(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	rec := progrock.NewRecorder(writer)

	vtx := rec.Vertex("hey", "sup")
	fmt.Fprintln(vtx.Stdout(), "hi stdout")
	fmt.Fprintln(vtx.Stderr(), "hi stderr")
	fmt.Fprintln(vtx.Stdout(), "hi again stdout")
	vtx.Done(nil)

	testGolden(t, buf)
}

func TestVertexStartStdoutStderrError(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	rec := progrock.NewRecorder(writer)

	vtx := rec.Vertex("hey", "sup")
	fmt.Fprintln(vtx.Stdout(), "hi stdout")
	fmt.Fprintln(vtx.Stderr(), "hi stderr")
	fmt.Fprintln(vtx.Stdout(), "hi again stdout")
	vtx.Done(errors.New("oh no!"))

	testGolden(t, buf)
}

func TestMultipleVertices(t *testing.T) {
	clock := clockwork.NewFakeClock()

	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf, console.WithClock(clock))

	rec := progrock.NewRecorder(writer)

	vtx1 := rec.Vertex("vtx1", "vtx1")
	vtx2 := rec.Vertex("vtx2", "vtx2")

	for i := 0; i < 10; i++ {
		fmt.Fprintf(vtx1.Stdout(), "1.%d hi\n", i)
		fmt.Fprintf(vtx1.Stdout(), "1.%d hi again\n", i)
		fmt.Fprintf(vtx1.Stdout(), "1.%d im very chatty\n", i)
		fmt.Fprintf(vtx2.Stdout(), "2.%d im less chatty\n", i)
		clock.Advance(console.AntiFlicker)
	}

	vtx1.Done(nil)
	vtx2.Done(nil)

	testGolden(t, buf)
}

func TestSingleCompletedTasks(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	recorder := progrock.NewRecorder(writer)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	vtx.Task("task 2").Done(nil)

	testGolden(t, buf)
}

func TestSingleRunningTasks(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	recorder := progrock.NewRecorder(writer)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1")
	vtx.Task("task 2")
	vtx.Task("task 3").Done(nil)

	testGolden(t, buf)
}

func TestSingleRunningTasksProgress(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	recorder := progrock.NewRecorder(writer)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	task := vtx.ProgressTask(100, "task 2")
	task.Current(25)
	vtx.Task("task 2").Done(nil)

	testGolden(t, buf)
}

func testGolden(t *testing.T, buf *bytes.Buffer) {
	g := goldie.New(t)
	g.Assert(t, t.Name(), buf.Bytes())
}

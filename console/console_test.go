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

func TestVertexGroups(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf)

	rec := progrock.NewRecorder(writer)

	g1 := rec.WithGroup("group1")
	g2 := rec.WithGroup("group2")

	level1 := g1.Vertex("vtx1", "first vertex")
	fmt.Fprintln(level1.Stdout(), "hello 1")

	level2 := g1.WithGroup("subgroup1").Vertex("vtx2", "second vertex")
	fmt.Fprintln(level2.Stdout(), "hello 2")

	level1.Done(nil)
	level2.Done(nil)

	g2.Join("vtx3")
	multi := g1.Vertex("vtx3", "third vertex")

	multi.Done(nil)

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

func TestMultipleVerticesInternal(t *testing.T) {
	clock := clockwork.NewFakeClock()

	buf := bytes.NewBuffer(nil)
	writer := console.NewWriter(buf, console.WithClock(clock))

	rec := progrock.NewRecorder(writer)

	vtx1 := rec.Vertex("vtx1", "vtx1")
	vtx2 := rec.Vertex("vtx2", "vtx2", progrock.Internal())

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

func TestMessages(t *testing.T) {
	t.Run("debug messages are not shown by default", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		writer := console.NewWriter(buf)
		recorder := progrock.NewRecorder(writer)
		recorder.Vertex("a", "some vertex").Done(nil)
		recorder.Debug("hello")
		testGolden(t, buf)
	})

	t.Run("debug messages are shown if level is set", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		writer := console.NewWriter(buf,
			console.WithMessageLevel(progrock.MessageLevel_DEBUG))
		recorder := progrock.NewRecorder(writer)
		recorder.Vertex("a", "some vertex").Done(nil)
		recorder.Debug("hello")
		testGolden(t, buf)
	})

	t.Run("warnings are shown by default", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		writer := console.NewWriter(buf)
		recorder := progrock.NewRecorder(writer)
		recorder.Vertex("a", "some vertex").Done(nil)
		recorder.Warn("uh oh")
		testGolden(t, buf)
	})

	t.Run("errors are shown by default", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		writer := console.NewWriter(buf)
		recorder := progrock.NewRecorder(writer)
		recorder.Vertex("a", "some vertex").Done(nil)
		recorder.Error("oh no")
		testGolden(t, buf)
	})
}

func testGolden(t *testing.T, buf *bytes.Buffer) {
	g := goldie.New(t)
	g.Assert(t, t.Name(), buf.Bytes())
}

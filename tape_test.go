package progrock_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"github.com/vito/progrock"
)

var ui = progrock.DefaultUI()

func TestEmpty(t *testing.T) {
	tape := progrock.NewTape()
	testGolden(t, tape)
}

func TestSingle(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	recorder.Vertex("a", "vertex a").Done(nil)

	testGolden(t, tape)
}

func TestSingleNoGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewPassthroughRecorder(tape)
	recorder.Vertex("a", "vertex a").Done(nil)

	testGolden(t, tape)
}

func TestSingleRunning(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder, "a", "vertex a")

	testGolden(t, tape)
}

func TestSingleRunningNoGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewPassthroughRecorder(tape)
	runningVtx(recorder, "a", "vertex a")

	testGolden(t, tape)
}

func TestSinglePending(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)

	recorder.Record(&progrock.StatusUpdate{
		Vertexes: []*progrock.Vertex{
			{
				Id:   "a",
				Name: "vertex a",
			},
		},
	})

	testGolden(t, tape)
}

func TestSingleCached(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	vtx := runningVtx(recorder, "a", "vertex a")
	vtx.Cached()
	vtx.Done(nil)

	testGolden(t, tape)
}

func TestSingleErrored(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder, "a", "vertex a").Done(fmt.Errorf("nope"))

	testGolden(t, tape)
}

func TestSingleErroredNoGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewPassthroughRecorder(tape)
	runningVtx(recorder, "a", "vertex a").Done(fmt.Errorf("nope"))

	testGolden(t, tape)
}

func TestSingleCanceled(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder, "a", "vertex a").Done(context.Canceled)

	testGolden(t, tape)
}

func TestSingleCompleted(t *testing.T) {
	t.Run("no show all output", func(t *testing.T) {
		tape := progrock.NewTape()
		recorder := progrock.NewRecorder(tape)
		runningVtx(recorder, "a", "vertex a").Done(nil)
		testGolden(t, tape)
	})
	t.Run("show all output", func(t *testing.T) {
		tape := progrock.NewTape()
		tape.ShowAllOutput(true)
		recorder := progrock.NewRecorder(tape)
		runningVtx(recorder, "a", "vertex a").Done(nil)
		testGolden(t, tape)
	})
	t.Run("done", func(t *testing.T) {
		tape := progrock.NewTape()
		recorder := progrock.NewRecorder(tape)
		runningVtx(recorder, "a", "vertex a").Done(nil)
		tape.Close()
		testGolden(t, tape)
	})
}

func TestSingleCompletedTasks(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	vtx.Task("task 2").Done(nil)

	testGolden(t, tape)
}

func TestSingleRunningTasks(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1")
	vtx.Task("task 2")
	vtx.Task("task 3").Done(nil)

	testGolden(t, tape)
}

func TestSingleRunningTasksProgress(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	task := vtx.ProgressTask(100, "task 2")
	task.Current(25)
	vtx.Task("task 2").Done(nil)

	testGolden(t, tape)
}

func TestDoubleRunning(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder, "a", "vertex a")
	runningVtx(recorder, "b", "vertex b")

	testGolden(t, tape)
}

func TestRunningDone(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder, "a", "vertex a")
	runningVtx(recorder, "b", "vertex b").Done(nil)

	testGolden(t, tape)
}

func TestDouble(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	recorder.Vertex("a", "vertex a").Done(nil)
	recorder.Vertex("b", "vertex b").Done(nil)

	testGolden(t, tape)
}

func TestGroupedUngrouped(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	recorder.WithGroup("group 1").Vertex("a", "vertex a").Done(nil)
	recorder.Vertex("b", "vertex b").Done(nil)

	testGolden(t, tape)
}

func TestGroupedWeakStrong(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	recorder.WithGroup("strong group").Vertex("a", "vertex a").Done(nil)
	recorder.WithGroup("weak group", progrock.Weak()).Vertex("b", "vertex b").Done(nil)

	testGolden(t, tape)
}

func TestInputSameGroup(t *testing.T) {
	t.Run("no verbose edges", func(t *testing.T) {
		tape := progrock.NewTape()
		recorder := progrock.NewRecorder(tape)
		recorder.Vertex("a", "vertex a").Done(nil)
		recorder.Vertex("b", "vertex b", progrock.WithInputs("a")).Done(nil)
		testGolden(t, tape)
	})

	t.Run("verbose edges", func(t *testing.T) {
		tape := progrock.NewTape()
		tape.VerboseEdges(true)
		recorder := progrock.NewRecorder(tape)
		recorder.Vertex("a", "vertex a").Done(nil)
		recorder.Vertex("b", "vertex b", progrock.WithInputs("a")).Done(nil)
		testGolden(t, tape)
	})
}

func TestOutputInputSameGroup(t *testing.T) {
	t.Run("no verbose edges", func(t *testing.T) {
		tape := progrock.NewTape()
		recorder := progrock.NewRecorder(tape)
		vtx := recorder.Vertex("a", "vertex a")
		vtx.Output("foo")
		vtx.Done(nil)
		recorder.Vertex("b", "vertex b", progrock.WithInputs("foo")).Done(nil)
		testGolden(t, tape)
	})

	t.Run("verbose edges", func(t *testing.T) {
		tape := progrock.NewTape()
		tape.VerboseEdges(true)
		recorder := progrock.NewRecorder(tape)
		vtx := recorder.Vertex("a", "vertex a")
		vtx.Output("foo")
		vtx.Done(nil)
		recorder.Vertex("b", "vertex b", progrock.WithInputs("foo")).Done(nil)
		testGolden(t, tape)
	})
}

func TestInputDifferentGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	recorder.Vertex("a", "vertex a").Done(nil)
	recorder.WithGroup("group 1").Vertex("b", "vertex b", progrock.WithInputs("a")).Done(nil)

	testGolden(t, tape)
}

func TestOutputInputDifferentGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Output("foo")
	vtx.Done(nil)
	recorder.WithGroup("group 1").Vertex("b", "vertex b", progrock.WithInputs("foo")).Done(nil)

	testGolden(t, tape)
}

func TestInternal(t *testing.T) {
	t.Run("no show internal", func(t *testing.T) {
		tape := progrock.NewTape()
		recorder := progrock.NewRecorder(tape)
		recorder.Vertex("a", "vertex a").Done(nil)
		recorder.Vertex("internal", "internal vertex", progrock.Internal()).Done(nil)
		testGolden(t, tape)
	})

	t.Run("show internal", func(t *testing.T) {
		tape := progrock.NewTape()
		tape.ShowInternal(true)
		recorder := progrock.NewRecorder(tape)
		recorder.Vertex("a", "vertex a").Done(nil)
		recorder.Vertex("internal", "internal vertex", progrock.Internal()).Done(nil)
		testGolden(t, tape)
	})
}

func TestInternalErrored(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder, "a", "vertex a", progrock.Internal()).Done(fmt.Errorf("nope"))

	testGolden(t, tape)
}

func TestSingleDoneTasks(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	vtx.Task("task 2").Done(nil)
	vtx.Done(nil)

	testGolden(t, tape)
}

func TestSingleDoneTasksProgress(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	task := vtx.ProgressTask(100, "task 2")
	task.Current(25)
	vtx.Task("task 2").Done(nil)
	vtx.Done(nil)

	testGolden(t, tape)
}

func TestRunningDifferentGroups(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a")
	runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c")

	testGolden(t, tape)
}

func TestRunningDifferentGroupsTask(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a")
	runningVtx(recorder.WithGroup("group b"), "b", "vertex b").Task("b task").Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c")

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinue(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a")
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c")
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2")

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromRoot(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a")
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c")
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2")

	runningVtx(recorder.WithGroup("group d"), "d", "vertex d")

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromFirstGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a").Done(nil)
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d", "vertex d").Done(nil)
	runningVtx(recorder.WithGroup("group e"), "e", "vertex e").Done(nil)
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2").Done(nil)

	runningVtx(recorder.WithGroup("group a").WithGroup("group a.a"), "z", "vertex z")

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromLastGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a").Done(nil)
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d", "vertex d").Done(nil)
	runningVtx(recorder.WithGroup("group e"), "e", "vertex e").Done(nil)
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2").Done(nil)

	runningVtx(recorder.WithGroup("group e").WithGroup("group e.a"), "z", "vertex z")

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromFirstGroupSingleInputFirstGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)

	runningVtx(recorder.WithGroup("group a"), "a", "vertex a").Done(nil)
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d", "vertex d").Done(nil)
	runningVtx(recorder.WithGroup("group e"), "e", "vertex e").Done(nil)
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2").Done(nil)

	runningVtx(
		recorder.WithGroup("group a").WithGroup("group a.a"),
		"z",
		"vertex z",
		progrock.WithInputs("a"),
	)

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromFirstGroupSingleInputLastGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)

	runningVtx(recorder.WithGroup("group a"), "a", "vertex a").Done(nil)
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d", "vertex d").Done(nil)
	runningVtx(recorder.WithGroup("group e"), "e", "vertex e").Done(nil)
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2").Done(nil)

	runningVtx(
		recorder.WithGroup("group a").WithGroup("group a.a"),
		"z",
		"vertex z",
		progrock.WithInputs("e"),
	)

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromLastGroupSingleInputFirstGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)

	runningVtx(recorder.WithGroup("group a"), "a", "vertex a").Done(nil)
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d", "vertex d").Done(nil)
	runningVtx(recorder.WithGroup("group e"), "e", "vertex e").Done(nil)
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2").Done(nil)

	runningVtx(
		recorder.WithGroup("group e").WithGroup("group e.a"),
		"z",
		"vertex z",
		progrock.WithInputs("a"),
	)

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromLastGroupSingleInputLastGroup(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)

	runningVtx(recorder.WithGroup("group a"), "a", "vertex a").Done(nil)
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d", "vertex d").Done(nil)
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group e"), "e", "vertex e")
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d2", "vertex d2").Done(nil)

	runningVtx(
		recorder.WithGroup("group e").WithGroup("group e.a"),
		"z",
		"vertex z",
		progrock.WithInputs("d2"),
	)

	testGolden(t, tape)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromLastGroupAllInputs(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)

	runningVtx(recorder.WithGroup("group a"), "a", "vertex a").Done(nil)
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d", "vertex d").Done(nil)
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group e"), "e", "vertex e")
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2").Done(nil)
	runningVtx(recorder.WithGroup("group d"), "d2", "vertex d2").Done(nil)

	runningVtx(
		recorder.WithGroup("group a").WithGroup("group a.a"),
		"z",
		"vertex z",
		progrock.WithInputs("a", "b", "c", "d", "e", "c2", "d2"),
	)

	testGolden(t, tape)
}

func TestVertexInputsCrossGapLeft(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)

	ag := recorder.WithGroup("a")
	bg := ag.WithGroup("b")
	cg := bg.WithGroup("c")

	ag.Vertex("a1", "vertex a1").Done(nil)
	bg.Vertex("b1", "vertex b1").Done(nil)

	ag.WithGroup("group z").Vertex(
		"z1",
		"vertex z1",
		progrock.WithInputs("a1"),
	).Done(nil)

	cg.Vertex("b2", "vertex b2").Done(nil)

	ag.WithGroup("group z").Vertex(
		"z2",
		"vertex z2",
		progrock.WithInputs("a1", "b1"),
	).Done(nil)

	testGolden(t, tape)
}

func TestVertexInputsCrossGapRight(t *testing.T) {
	tape := progrock.NewTape()

	recorder := progrock.NewRecorder(tape)

	ag := recorder.WithGroup("a")
	bg := ag.WithGroup("b")
	cg := bg.WithGroup("c")
	dg := cg.WithGroup("d")

	ag.Vertex("a1", "vertex a1").Done(nil)
	bg.Vertex("b1", "vertex b1").Done(nil)
	cg.Vertex("c1", "vertex c1").Done(nil)
	dg.Vertex("d1", "vertex d1").Done(nil)

	ag.WithGroup("group z").Vertex(
		"z",
		"vertex z",
		progrock.WithInputs("c1", "d1"),
	).Done(nil)

	testGolden(t, tape)
}

func TestAutoResize(t *testing.T) {
	tape := progrock.NewTape()
	recorder := progrock.NewRecorder(tape)

	t.Run("initially unbounded", func(t *testing.T) {
		vtx := recorder.Vertex("long", "long lines")
		for i := 0; i < 200; i++ {
			fmt.Fprint(vtx.Stdout(), "x")
		}

		testGoldenAutoResize(t, tape)
	})

	t.Run("respects width for existing and new", func(t *testing.T) {
		tape.SetWindowSize(80, 24)

		vtx := recorder.Vertex("long2", "more long lines")
		for i := 0; i < 200; i++ {
			fmt.Fprint(vtx.Stdout(), "y")
		}

		testGoldenAutoResize(t, tape)
	})
}

func TestMessages(t *testing.T) {
	t.Run("debug messages are not shown by default", func(t *testing.T) {
		tape := progrock.NewTape()
		recorder := progrock.NewRecorder(tape)
		recorder.Vertex("a", "some vertex").Done(nil)
		recorder.Debug("hello")
		testGolden(t, tape)
	})

	t.Run("debug messages are shown if level is set", func(t *testing.T) {
		tape := progrock.NewTape()
		tape.MessageLevel(progrock.MessageLevel_DEBUG)
		recorder := progrock.NewRecorder(tape)
		recorder.Vertex("a", "some vertex").Done(nil)
		recorder.Debug("hello")
		testGolden(t, tape)
	})

	t.Run("warnings are shown by default", func(t *testing.T) {
		tape := progrock.NewTape()
		recorder := progrock.NewRecorder(tape)
		recorder.Vertex("a", "some vertex").Done(nil)
		recorder.Warn("uh oh")
		testGolden(t, tape)
	})

	t.Run("errors are shown by default", func(t *testing.T) {
		tape := progrock.NewTape()
		recorder := progrock.NewRecorder(tape)
		recorder.Vertex("a", "some vertex").Done(nil)
		recorder.Error("oh no")
		testGolden(t, tape)
	})
}

func testGolden(t *testing.T, tape *progrock.Tape) {
	buf := new(bytes.Buffer)
	tape.SetWindowSize(80, 24)

	err := tape.Render(buf, ui)
	require.NoError(t, err)

	t.Log(buf.String())

	g := goldie.New(t)
	g.Assert(t, t.Name(), buf.Bytes())
}

func testGoldenAutoResize(t *testing.T, tape *progrock.Tape) {
	buf := new(bytes.Buffer)
	tape.Render(buf, ui)

	g := goldie.New(t)
	g.Assert(t, t.Name(), buf.Bytes())
}

func runningVtx(rec *progrock.Recorder, digest digest.Digest, name string, opts ...progrock.VertexOpt) *progrock.VertexRecorder {
	vtx := rec.Vertex(digest, name, opts...)
	fmt.Fprintln(vtx.Stdout(), "stdout 1")
	fmt.Fprintln(vtx.Stderr(), "stderr 1")
	fmt.Fprintln(vtx.Stdout(), "stdout 2")
	fmt.Fprintln(vtx.Stderr(), "stderr 2")
	return vtx
}

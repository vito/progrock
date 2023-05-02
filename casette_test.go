package progrock_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/sebdah/goldie/v2"
	"github.com/vito/progrock"
)

var ui = progrock.DefaultUI()

func testGolden(t *testing.T, casette *progrock.Casette) {
	buf := new(bytes.Buffer)
	casette.Render(buf, ui)
	casette.SetWindowSize(100, 100)

	// t.Log("ui:\n" + buf.String())

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

func TestEmpty(t *testing.T) {
	casette := progrock.NewCasette()
	testGolden(t, casette)
}

func TestSingle(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	recorder.Vertex("a", "vertex a").Done(nil)

	testGolden(t, casette)
}

func TestSingleRunning(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	runningVtx(recorder, "a", "vertex a")

	testGolden(t, casette)
}

func TestSinglePending(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)

	recorder.Record(&progrock.StatusUpdate{
		Vertexes: []*progrock.Vertex{
			{
				Id:   "a",
				Name: "vertex a",
			},
		},
	})

	testGolden(t, casette)
}

func TestSingleErrored(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	runningVtx(recorder, "a", "vertex a").Error(fmt.Errorf("nope"))

	testGolden(t, casette)
}

func TestSingleRunningTasks(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	vtx.Task("task 2").Done(nil)

	testGolden(t, casette)
}

func TestSingleRunningTasksProgress(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	task := vtx.ProgressTask(100, "task 2")
	task.Current(25)
	vtx.Task("task 2").Done(nil)

	testGolden(t, casette)
}

func TestDoubleRunning(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	runningVtx(recorder, "a", "vertex a")
	runningVtx(recorder, "b", "vertex b")

	testGolden(t, casette)
}

func TestRunningDone(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	runningVtx(recorder, "a", "vertex a")
	runningVtx(recorder, "b", "vertex b").Done(nil)

	testGolden(t, casette)
}

func TestDouble(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	recorder.Vertex("a", "vertex a").Done(nil)
	recorder.Vertex("b", "vertex b").Done(nil)

	testGolden(t, casette)
}

func TestGroupedUngrouped(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	recorder.WithGroup("group 1").Vertex("a", "vertex a").Done(nil)
	recorder.Vertex("b", "vertex b").Done(nil)

	testGolden(t, casette)
}

func TestInputSameGroup(t *testing.T) {
	t.Run("no verbose edges", func(t *testing.T) {
		casette := progrock.NewCasette()
		recorder := progrock.NewRecorder(casette)
		recorder.Vertex("a", "vertex a").Done(nil)
		recorder.Vertex("b", "vertex b", progrock.WithInputs("a")).Done(nil)
		testGolden(t, casette)
	})

	t.Run("verbose edges", func(t *testing.T) {
		casette := progrock.NewCasette()
		casette.VerboseEdges(true)
		recorder := progrock.NewRecorder(casette)
		recorder.Vertex("a", "vertex a").Done(nil)
		recorder.Vertex("b", "vertex b", progrock.WithInputs("a")).Done(nil)
		testGolden(t, casette)
	})
}

func TestOutputInputSameGroup(t *testing.T) {
	t.Run("no verbose edges", func(t *testing.T) {
		casette := progrock.NewCasette()
		recorder := progrock.NewRecorder(casette)
		vtx := recorder.Vertex("a", "vertex a")
		vtx.Output("foo")
		vtx.Done(nil)
		recorder.Vertex("b", "vertex b", progrock.WithInputs("foo")).Done(nil)
		testGolden(t, casette)
	})

	t.Run("verbose edges", func(t *testing.T) {
		casette := progrock.NewCasette()
		casette.VerboseEdges(true)
		recorder := progrock.NewRecorder(casette)
		vtx := recorder.Vertex("a", "vertex a")
		vtx.Output("foo")
		vtx.Done(nil)
		recorder.Vertex("b", "vertex b", progrock.WithInputs("foo")).Done(nil)
		testGolden(t, casette)
	})
}

func TestInputDifferentGroup(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	recorder.Vertex("a", "vertex a").Done(nil)
	recorder.WithGroup("group 1").Vertex("b", "vertex b", progrock.WithInputs("a")).Done(nil)

	testGolden(t, casette)
}

func TestOutputInputDifferentGroup(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Output("foo")
	vtx.Done(nil)
	recorder.WithGroup("group 1").Vertex("b", "vertex b", progrock.WithInputs("foo")).Done(nil)

	testGolden(t, casette)
}

func TestInternal(t *testing.T) {
	t.Run("no show internal", func(t *testing.T) {
		casette := progrock.NewCasette()
		recorder := progrock.NewRecorder(casette)
		recorder.Vertex("a", "vertex a").Done(nil)
		recorder.Vertex("internal", "internal vertex", progrock.Internal()).Done(nil)
		testGolden(t, casette)
	})

	t.Run("show internal", func(t *testing.T) {
		casette := progrock.NewCasette()
		casette.ShowInternal(true)
		recorder := progrock.NewRecorder(casette)
		recorder.Vertex("a", "vertex a").Done(nil)
		recorder.Vertex("internal", "internal vertex", progrock.Internal()).Done(nil)
		testGolden(t, casette)
	})
}

func TestSingleDoneTasks(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	vtx.Task("task 2").Done(nil)
	vtx.Done(nil)

	testGolden(t, casette)
}

func TestSingleDoneTasksProgress(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	vtx := recorder.Vertex("a", "vertex a")
	vtx.Task("task 1").Done(nil)
	task := vtx.ProgressTask(100, "task 2")
	task.Current(25)
	vtx.Task("task 2").Done(nil)
	vtx.Done(nil)

	testGolden(t, casette)
}

func TestRunningDifferentGroups(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a")
	runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c")

	testGolden(t, casette)
}

func TestRunningDifferentGroupsTask(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a")
	runningVtx(recorder.WithGroup("group b"), "b", "vertex b").Task("b task").Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c")

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinue(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a")
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c")
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2")

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromRoot(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
	runningVtx(recorder.WithGroup("group a"), "a", "vertex a")
	b1 := runningVtx(recorder.WithGroup("group b"), "b", "vertex b")
	runningVtx(recorder.WithGroup("group c"), "c", "vertex c")
	b2 := runningVtx(recorder.WithGroup("group b"), "b2", "vertex b2")
	b1.Done(nil)
	b2.Done(nil)
	runningVtx(recorder.WithGroup("group c"), "c2", "vertex c2")

	runningVtx(recorder.WithGroup("group d"), "d", "vertex d")

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromFirstGroup(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
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

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromLastGroup(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)
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

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromFirstGroupSingleInputFirstGroup(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)

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

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromFirstGroupSingleInputLastGroup(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)

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

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromLastGroupSingleInputFirstGroup(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)

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

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromLastGroupSingleInputLastGroup(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)

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

	testGolden(t, casette)
}

func TestRunningGroupEndsOthersContinueNewSpawnsFromLastGroupAllInputs(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)

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

	testGolden(t, casette)
}

func TestVertexInputsCrossGapLeft(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)

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
		progrock.WithInputs("a1"),
	).Done(nil)

	testGolden(t, casette)
}

func TestVertexInputsCrossGapRight(t *testing.T) {
	casette := progrock.NewCasette()

	recorder := progrock.NewRecorder(casette)

	ag := recorder.WithGroup("a")
	bg := ag.WithGroup("b")
	cg := bg.WithGroup("c")
	dg := cg.WithGroup("d")

	ag.Vertex("a1", "vertex a1").Done(nil)
	bg.Vertex("b1", "vertex b1").Done(nil)
	cg.Vertex("c1", "vertex c1").Done(nil)
	dg.Vertex("d1", "vertex d1").Done(nil)

	bg.WithGroup("group z").Vertex(
		"z",
		"vertex z",
		progrock.WithInputs("a1", "d1"),
	).Done(nil)

	testGolden(t, casette)
}

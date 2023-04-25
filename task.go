package progrock

import timestamppb "google.golang.org/protobuf/types/known/timestamppb"

type TaskRecorder struct {
	*VertexRecorder

	Task *VertexTask
}

func (recorder *TaskRecorder) Wrap(f func() error) error {
	recorder.Start()
	err := f()
	recorder.Done(err)
	return err
}

func (recorder *TaskRecorder) Done(err error) {
	if err != nil {
		recorder.Error(err)
	}

	recorder.Complete()
}

func (recorder *TaskRecorder) Start() {
	now := Clock.Now()
	recorder.Task.Started = timestamppb.New(now)
	recorder.sync()
}

func (recorder *TaskRecorder) Complete() {
	now := Clock.Now()

	if recorder.Task.Started == nil {
		recorder.Task.Started = timestamppb.New(now)
	}

	recorder.Task.Completed = timestamppb.New(now)

	recorder.sync()
}

func (recorder *TaskRecorder) Progress(cur, total int64) {
	recorder.Task.Current = cur
	recorder.Task.Total = total
	recorder.sync()
}

func (recorder *TaskRecorder) Current(cur int64) {
	recorder.Task.Current = cur
	recorder.sync()
}

func (recorder *TaskRecorder) sync() {
	recorder.Recorder.Record(&StatusUpdate{
		Tasks: []*VertexTask{
			recorder.Task,
		},
	})
}

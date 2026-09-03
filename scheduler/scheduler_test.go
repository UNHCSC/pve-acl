package scheduler

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
	"github.com/z46-dev/gasket"
	"github.com/z46-dev/golog"
)

type waitOutcome struct {
	Result gasket.TaskConsumerResult
	Err    error
}

func TestNoopJobRunsThroughGasket(t *testing.T) {
	initSchedulerTestDB(t)
	var (
		service *Service
		err     error
	)

	service, err = New(ResolveDatabaseFile(config.Config.Database.File, config.Config.Scheduler.DatabaseFile))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer service.Close()
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	var job *db.Job

	job, err = db.CreateJob(db.JobCreateInput{
		JobType: db.JobTypeCleanup,
	})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	var taskInfo *gasket.TaskInfo

	taskInfo, err = service.EnqueueJobTask(job, TaskTypeSystemNoop, JobPayload{})
	if err != nil {
		t.Fatalf("EnqueueJobTask returned error: %v", err)
	}
	if job.QueueID == "" {
		t.Fatal("expected queue id to be recorded")
	}
	var runDone chan error = make(chan error, 1)
	go func() { runDone <- service.Run(ctx) }()
	var completed chan waitOutcome

	completed = make(chan waitOutcome, 1)
	go func() {
		var outcome waitOutcome
		outcome.Result, outcome.Err = taskInfo.WaitForCompletion()
		completed <- outcome
	}()
	var outcome waitOutcome

	select {
	case outcome = <-completed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for noop task completion")
	}
	if outcome.Err != nil {
		t.Fatalf("WaitForCompletion returned error: %v", outcome.Err)
	}
	if !outcome.Result.Success {
		t.Fatalf("expected successful result, got %#v", outcome.Result)
	}
	cancel()
	if err = <-runDone; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned error: %v", err)
	}
	var found bool

	job, found, err = db.GetJobByID(job.ID)
	if err != nil || !found {
		t.Fatalf("GetJobByID found=%v err=%v", found, err)
	}
	if job.Status != db.JobStatusSucceeded || job.StartedAt == nil || job.FinishedAt == nil {
		t.Fatalf("expected job to be succeeded, got %#v", job)
	}
	var logs []*db.JobLog

	logs, err = db.JobLogsForJob(job.ID)
	if err != nil {
		t.Fatalf("JobLogsForJob returned error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one scheduler log, got %#v", logs)
	}
}

func TestMultiStepJobExposesProgressAndCancellation(t *testing.T) {
	initSchedulerTestDB(t)
	var service *Service
	var err error
	if service, err = New(ResolveDatabaseFile(config.Config.Database.File, "")); err != nil {
		t.Fatalf("New: %v", err)
	}
	defer service.Close()
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	var job *db.Job
	if job, err = db.CreateJob(db.JobCreateInput{JobType: db.JobTypeCleanup, Operation: "system.demo"}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	var task *gasket.TaskInfo
	if task, err = service.EnqueueJobTask(job, TaskTypeSystemDemo, JobPayload{}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	var runDone chan error = make(chan error, 1)
	go func() { runDone <- service.Run(ctx) }()
	var deadline time.Time = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, _, err = db.GetJobByID(job.ID)
		if job.Status == db.JobStatusRunning && job.Progress > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.Progress == 0 {
		t.Fatal("job did not expose progress")
	}
	if _, err = db.RequestJobCancellation(job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	_, _ = task.WaitForCompletion()
	job, _, err = db.GetJobByID(job.ID)
	if job.Status != db.JobStatusCancelled || job.CancelRequestedAt == nil {
		t.Fatalf("expected cancelled job, got %#v", job)
	}
	cancel()
	<-runDone
}

func initSchedulerTestDB(t *testing.T) {
	t.Helper()
	var err error

	if db.Driver != nil {
		if err = db.Driver.Close(); err != nil {
			t.Fatalf("close previous database: %v", err)
		}
	}
	config.Config = config.Configuration{}
	config.Config.Database.File = filepath.Join(t.TempDir(), "scheduler-test.db")
	if err = db.Init(golog.New()); err != nil {
		t.Fatalf("db.Init returned error: %v", err)
	}
	t.Cleanup(func() {
		if db.Driver != nil {
			_ = db.Driver.Close()
		}
	})
}

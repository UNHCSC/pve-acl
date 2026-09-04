package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
	"github.com/z46-dev/gasket"
)

const (
	TaskTypeSystemNoop    = "system.noop"
	TaskTypeSystemDemo    = "system.demo"
	TaskTypeProxmoxAction = "proxmox.action"
	TaskTypeRunnerAction  = "runner.action"
)

type (
	JobPayload struct {
		JobID             int               `json:"jobID"`
		ProjectID         *int              `json:"projectID,omitempty"`
		ResourceID        *int              `json:"resourceID,omitempty"`
		RequestedByUserID *int              `json:"requestedByUserID,omitempty"`
		Operation         string            `json:"operation,omitempty"`
		Metadata          map[string]string `json:"metadata,omitempty"`
	}

	Service struct {
		client       *gasket.Client
		mu           sync.Mutex
		globalSlots  chan struct{}
		perNodeLimit int
		nodeSlots    map[string]chan struct{}
		drainTimeout time.Duration
		closing      bool
	}
)

var defaultService *Service

// Init initializes the process-wide scheduler service.
func Init(databaseFile string) (serviceResult *Service, errResult error) {
	var (
		service *Service
		err     error
	)

	service, err = newWithConcurrency(databaseFile, config.Config.Scheduler.GlobalConcurrency, config.Config.Scheduler.PerNodeConcurrency, time.Duration(config.Config.Scheduler.ShutdownDrainSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	defaultService = service
	return service, nil
}

// Default returns the process-wide scheduler service.
func Default() (serviceResult *Service) {
	return defaultService
}

// ResolveDatabaseFile chooses the scheduler database path.
func ResolveDatabaseFile(applicationDatabaseFile string, configuredSchedulerDatabaseFile string) (valueResult string) {
	configuredSchedulerDatabaseFile = strings.TrimSpace(configuredSchedulerDatabaseFile)
	if configuredSchedulerDatabaseFile != "" {
		return configuredSchedulerDatabaseFile
	}
	applicationDatabaseFile = strings.TrimSpace(applicationDatabaseFile)
	if applicationDatabaseFile == "" {
		return "organesson-tasks.db"
	}
	return applicationDatabaseFile + ".tasks"
}

// New creates a scheduler service backed by gasket.
func New(databaseFile string) (serviceResult *Service, errResult error) {
	return newWithConcurrency(databaseFile, 1, 1, 20*time.Second)
}

func newWithConcurrency(databaseFile string, globalLimit, perNodeLimit int, drainTimeout time.Duration) (serviceResult *Service, errResult error) {
	var (
		client *gasket.Client
		err    error
	)

	client, err = gasket.NewClient(
		databaseFile,
		gasket.PollInterval(250*time.Millisecond),
		gasket.DatabaseLockRetry(50, 10*time.Millisecond),
		gasket.TaskRecoveryTimeout(10*time.Minute),
	)
	if err != nil {
		return nil, err
	}
	var service *Service

	if globalLimit <= 0 {
		globalLimit = 1
	}
	if perNodeLimit <= 0 {
		perNodeLimit = 1
	}
	if drainTimeout <= 0 {
		drainTimeout = 20 * time.Second
	}
	service = &Service{client: client, globalSlots: make(chan struct{}, globalLimit), perNodeLimit: perNodeLimit, nodeSlots: make(map[string]chan struct{}), drainTimeout: drainTimeout}
	if err = service.RegisterConsumer(TaskTypeSystemNoop, consumeSystemNoop); err != nil {
		_ = client.Close()
		return nil, err
	}
	if err = service.RegisterConsumer(TaskTypeSystemDemo, consumeSystemDemo); err != nil {
		_ = client.Close()
		return nil, err
	}
	return service, nil
}

// AcquireOperation reserves configured global and per-node capacity until release is called.
func (s *Service) AcquireOperation(ctx context.Context, node string) (releaseResult func(), errResult error) {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return nil, fmt.Errorf("scheduler is shutting down")
	}
	s.mu.Unlock()
	select {
	case s.globalSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.mu.Lock()
	var nodeSlots chan struct{} = s.nodeSlots[node]
	if nodeSlots == nil {
		nodeSlots = make(chan struct{}, s.perNodeLimit)
		s.nodeSlots[node] = nodeSlots
	}
	s.mu.Unlock()
	select {
	case nodeSlots <- struct{}{}:
	case <-ctx.Done():
		<-s.globalSlots
		return nil, ctx.Err()
	}
	var once sync.Once
	releaseResult = func() { once.Do(func() { <-nodeSlots; <-s.globalSlots }) }
	return
}

// Close closes the scheduler client.
func (s *Service) Close() (errResult error) {
	if s == nil || s.client == nil {
		return nil
	}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return nil
	}
	s.closing = true
	s.mu.Unlock()
	var timer *time.Timer = time.NewTimer(s.drainTimeout)
	defer timer.Stop()
	for slot := 0; slot < cap(s.globalSlots); slot++ {
		select {
		case s.globalSlots <- struct{}{}:
		case <-timer.C:
			return s.client.Close()
		}
	}
	errResult = s.client.Close()
	if defaultService == s {
		defaultService = nil
	}
	return
}

// Run starts the scheduler loop.
func (s *Service) Run(ctx context.Context) (errResult error) {
	if s == nil || s.client == nil {
		return fmt.Errorf("scheduler is not initialized")
	}
	return s.client.Run(ctx)
}

// RegisterConsumer registers a task consumer.
func (s *Service) RegisterConsumer(taskType string, consumer gasket.TaskConsumerFunc) (errResult error) {
	if s == nil || s.client == nil {
		return fmt.Errorf("scheduler is not initialized")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client.RegisterConsumer(taskType, consumer)
}

// EnqueueJobTask creates a gasket task and links it to a job.
func (s *Service) EnqueueJobTask(job *db.Job, taskType string, payload JobPayload, opts ...gasket.TaskOption) (taskInfoResult *gasket.TaskInfo, errResult error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("scheduler is not initialized")
	}
	if job == nil {
		return nil, fmt.Errorf("job is required")
	}
	payload.JobID = job.ID
	payload.ProjectID = job.ProjectID
	payload.ResourceID = job.ResourceID
	payload.RequestedByUserID = job.RequestedByUserID
	var encoded []byte
	var err error

	encoded, err = json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var taskInfo *gasket.TaskInfo

	taskInfo, err = s.client.NewTask(taskType, encoded, opts...)
	if err != nil {
		return nil, err
	}
	if err = db.SetJobQueueID(job.ID, strconv.Itoa(taskInfo.ID())); err != nil {
		return nil, err
	}
	job.QueueID = strconv.Itoa(taskInfo.ID())
	return taskInfo, nil
}

// EnqueueJobTask creates a task on the default scheduler.
func EnqueueJobTask(job *db.Job, taskType string, payload JobPayload, opts ...gasket.TaskOption) (taskInfoResult *gasket.TaskInfo, errResult error) {
	if defaultService == nil {
		return nil, fmt.Errorf("scheduler is not initialized")
	}
	return defaultService.EnqueueJobTask(job, taskType, payload, opts...)
}

// NewDefaultRetryPolicy returns the standard retry policy for operational jobs.
func NewDefaultRetryPolicy() (optionResult gasket.TaskOption) {
	return gasket.RetryPolicy(3, 10*time.Second)
}

func consumeSystemNoop(id int, payload []byte) (result gasket.TaskConsumerResult) {
	var jobPayload JobPayload
	var err error

	if err = json.Unmarshal(payload, &jobPayload); err != nil {
		result.Success = false
		result.Error = err
		return
	}
	if jobPayload.JobID > 0 {
		var job *db.Job
		var found bool
		if job, found, err = db.GetJobByID(jobPayload.JobID); err != nil || !found {
			result.Error = err
			return
		}
		if job.Status == db.JobStatusCancelled {
			result.Success = true
			return
		}
		if err = db.MarkJobRunning(jobPayload.JobID); err != nil {
			result.Success = false
			result.Error = err
			return
		}
		_ = db.AppendJobLog(jobPayload.JobID, db.JobLogStreamSystem, "scheduler task "+strconv.Itoa(id)+" completed noop work")
		if err = db.MarkJobFinished(jobPayload.JobID, db.JobStatusSucceeded); err != nil {
			result.Success = false
			result.Error = err
			return
		}
	}
	result.Success = true
	result.Data = []byte(`{"status":"ok"}`)
	return
}

func consumeSystemDemo(id int, payload []byte) (result gasket.TaskConsumerResult) {
	var jobPayload JobPayload
	var err error
	if err = json.Unmarshal(payload, &jobPayload); err != nil {
		result.Error = err
		return
	}
	var job *db.Job
	var found bool
	if job, found, err = db.GetJobByID(jobPayload.JobID); err != nil || !found {
		result.Error = err
		return
	}
	if job.Status == db.JobStatusCancelled {
		result.Success = true
		return
	}
	if err = db.MarkJobRunning(jobPayload.JobID); err != nil {
		result.Error = err
		return
	}
	for step := 1; step <= 4; step++ {
		var cancelled bool
		if cancelled, err = db.JobCancellationRequested(jobPayload.JobID); err != nil {
			result.Error = err
			return
		}
		if cancelled {
			_ = db.AppendJobLog(jobPayload.JobID, db.JobLogStreamSystem, "cancellation confirmed at a safe checkpoint")
			_ = db.MarkJobFinished(jobPayload.JobID, db.JobStatusCancelled)
			result.Success = true
			return
		}
		_ = db.AppendJobLog(jobPayload.JobID, db.JobLogStreamStdout, fmt.Sprintf("completed demonstration step %d of 4", step))
		if err = db.UpdateJobProgress(jobPayload.JobID, step*25, 2*time.Minute); err != nil {
			result.Error = err
			return
		}
		time.Sleep(75 * time.Millisecond)
	}
	if err = db.MarkJobFinished(jobPayload.JobID, db.JobStatusSucceeded); err != nil {
		result.Error = err
		return
	}
	result.Success = true
	result.Data = []byte(`{"status":"ok"}`)
	_ = id
	return
}

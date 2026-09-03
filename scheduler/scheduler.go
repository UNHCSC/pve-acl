package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/UNHCSC/organesson/db"
	"github.com/z46-dev/gasket"
)

const (
	TaskTypeSystemNoop    = "system.noop"
	TaskTypeProxmoxAction = "proxmox.action"
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
		client *gasket.Client
		mu     sync.Mutex
	}
)

var defaultService *Service

// Init initializes the process-wide scheduler service.
func Init(databaseFile string) (serviceResult *Service, errResult error) {
	var (
		service *Service
		err     error
	)

	service, err = New(databaseFile)
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

	service = &Service{
		client: client,
	}
	if err = service.RegisterConsumer(TaskTypeSystemNoop, consumeSystemNoop); err != nil {
		_ = client.Close()
		return nil, err
	}
	return service, nil
}

// Close closes the scheduler client.
func (s *Service) Close() (errResult error) {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
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

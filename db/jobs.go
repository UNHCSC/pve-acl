package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/z46-dev/gosqlite"
)

type JobCreateInput struct {
	JobType           JobType
	RequestedByUserID *int
	ProjectID         *int
	ResourceID        *int
	QueueID           string
	Operation         string
	Input             map[string]any
	IdempotencyKey    string
	OperationKey      string
	Node              string
	MaxAttempts       int
}

// CreateJob creates a user-visible job record.
func CreateJob(input JobCreateInput) (jobResult *Job, errResult error) {
	var (
		uuid string
		err  error
	)

	uuid, err = randomUUID()
	if err != nil {
		return nil, err
	}
	if input.ProjectID != nil {
		var (
			project *Project
			found   bool
		)

		project, found, err = GetProjectByID(*input.ProjectID)
		if err != nil {
			return nil, err
		}
		if !found || !project.IsActive {
			return nil, fmt.Errorf("project was not found")
		}
	}
	if input.ResourceID != nil {
		var (
			resource *Resource
			found    bool
		)

		resource, found, err = GetResourceByID(*input.ResourceID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("resource was not found")
		}
		if input.ProjectID != nil && resource.ProjectID != *input.ProjectID {
			return nil, fmt.Errorf("resource is not owned by the job project")
		}
	}
	var now time.Time

	now = time.Now().UTC()
	var job *Job

	var immutableInput []byte
	if immutableInput, err = json.Marshal(redactAuditMap(input.Input)); err != nil {
		return nil, err
	}
	if input.MaxAttempts <= 0 {
		input.MaxAttempts = 1
	}
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = "job:" + uuid
	}
	job = &Job{
		UUID:              uuid,
		JobType:           input.JobType,
		Status:            JobStatusQueued,
		RequestedByUserID: input.RequestedByUserID,
		ProjectID:         input.ProjectID,
		ResourceID:        input.ResourceID,
		QueueID:           strings.TrimSpace(input.QueueID),
		Operation:         strings.TrimSpace(input.Operation),
		InputJSON:         string(immutableInput),
		IdempotencyKey:    input.IdempotencyKey,
		OperationKey:      strings.TrimSpace(input.OperationKey),
		Node:              strings.TrimSpace(input.Node),
		MaxAttempts:       input.MaxAttempts,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err = Jobs.Insert(job); err != nil {
		return nil, err
	}
	if _, err = WriteAudit(AuditInput{ActorUserID: input.RequestedByUserID, Action: "job.create", TargetType: "job", TargetID: &job.ID, ProjectID: input.ProjectID}); err != nil {
		return nil, err
	}
	return job, nil
}

// FindJobByIdempotencyKey returns the existing job for a stable API request key.
func FindJobByIdempotencyKey(key string) (jobResult *Job, okResult bool, errResult error) {
	var jobs []*Job
	if jobs, errResult = Jobs.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(Jobs.FieldBySQLName("idempotency_key"), gosqlite.OpEqual, strings.TrimSpace(key)).Limit(1)); errResult != nil || len(jobs) == 0 {
		return nil, false, errResult
	}
	return jobs[0], true, nil
}

// ListJobs returns durable user-visible job history.
func ListJobs() (itemsResult []*Job, errResult error) {
	return Jobs.SelectAll()
}

// GetJobByID returns a job by id.
func GetJobByID(id int) (jobResult *Job, okResult bool, errResult error) {
	var (
		job *Job
		err error
	)

	job, err = Jobs.Select(id)
	if err != nil {
		return nil, false, err
	}
	if job == nil {
		return nil, false, nil
	}
	return job, true, nil
}

// UpdateJob updates a job timestamp and saves it.
func UpdateJob(job *Job) (errResult error) {
	var existing *Job
	if job != nil && job.ID > 0 {
		if existing, errResult = Jobs.Select(job.ID); errResult != nil {
			return
		}
		if existing != nil {
			job.Operation, job.InputJSON, job.IdempotencyKey, job.OperationKey = existing.Operation, existing.InputJSON, existing.IdempotencyKey, existing.OperationKey
		}
	}
	job.QueueID = strings.TrimSpace(job.QueueID)
	job.UpdatedAt = time.Now().UTC()
	return Jobs.Update(job)
}

// MarkJobRunning marks a job as running.
func MarkJobRunning(jobID int) (errResult error) {
	var (
		job   *Job
		found bool
		err   error
	)

	job, found, err = GetJobByID(jobID)
	if err != nil || !found {
		return err
	}
	var now time.Time

	now = time.Now().UTC()
	job.Status = JobStatusRunning
	job.AttemptCount++
	job.StartedAt = &now
	job.HeartbeatAt = &now
	var lease time.Time = now.Add(2 * time.Minute)
	job.LeaseExpiresAt = &lease
	job.UpdatedAt = now
	return Jobs.Update(job)
}

// UpdateJobProgress records bounded progress and renews the worker lease.
func UpdateJobProgress(jobID, progress int, leaseDuration time.Duration) (errResult error) {
	var job *Job
	var found bool
	if job, found, errResult = GetJobByID(jobID); errResult != nil || !found {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	var now time.Time = time.Now().UTC()
	var lease time.Time = now.Add(leaseDuration)
	job.Progress, job.HeartbeatAt, job.LeaseExpiresAt = progress, &now, &lease
	return UpdateJob(job)
}

// RequestJobCancellation records a cooperative cancellation request.
func RequestJobCancellation(jobID int) (jobResult *Job, errResult error) {
	var found bool
	if jobResult, found, errResult = GetJobByID(jobID); errResult != nil || !found {
		return
	}
	if jobResult.Status != JobStatusQueued && jobResult.Status != JobStatusRunning {
		return
	}
	var now time.Time = time.Now().UTC()
	jobResult.CancelRequestedAt = &now
	if jobResult.Status == JobStatusQueued {
		jobResult.Status, jobResult.FinishedAt = JobStatusCancelled, &now
	}
	errResult = UpdateJob(jobResult)
	return
}

// JobCancellationRequested reports whether a worker should stop safely.
func JobCancellationRequested(jobID int) (requestedResult bool, errResult error) {
	var job *Job
	var found bool
	if job, found, errResult = GetJobByID(jobID); errResult != nil || !found {
		return
	}
	return job.CancelRequestedAt != nil || job.Status == JobStatusCancelled, nil
}

// RecoverAbandonedJobs fails running jobs whose worker lease expired.
func RecoverAbandonedJobs(now time.Time) (countResult int, errResult error) {
	var jobs []*Job
	if jobs, errResult = Jobs.SelectAll(); errResult != nil {
		return
	}
	for _, job := range jobs {
		if job.Status != JobStatusRunning || job.LeaseExpiresAt == nil || job.LeaseExpiresAt.After(now) {
			continue
		}
		job.Status, job.ErrorCode, job.ErrorSummary, job.RetryClass = JobStatusFailed, "worker_abandoned", "Worker stopped before confirming completion; retry the operation after verifying target state.", "permanent"
		job.FinishedAt, job.UpdatedAt = &now, now
		if errResult = Jobs.Update(job); errResult != nil {
			return
		}
		_ = AppendJobLog(job.ID, JobLogStreamSystem, "worker lease expired; job marked failed for safe recovery")
		countResult++
	}
	return
}

// MarkJobFinished marks a job as succeeded, failed, or cancelled.
func MarkJobFinished(jobID int, status JobStatus) (errResult error) {
	var (
		job   *Job
		found bool
		err   error
	)

	job, found, err = GetJobByID(jobID)
	if err != nil || !found {
		return err
	}
	var now time.Time

	now = time.Now().UTC()
	job.Status = status
	if status == JobStatusSucceeded {
		job.Progress = 100
	}
	job.FinishedAt = &now
	job.LeaseExpiresAt = nil
	job.UpdatedAt = now
	return Jobs.Update(job)
}

// FailJob stores a safe classified failure without exposing raw provider output.
func FailJob(jobID int, code, summary, retryClass string) (errResult error) {
	var job *Job
	var found bool
	if job, found, errResult = GetJobByID(jobID); errResult != nil || !found {
		return
	}
	job.ErrorCode = strings.TrimSpace(code)
	job.ErrorSummary = RedactSensitiveText(strings.TrimSpace(summary))
	job.RetryClass = strings.TrimSpace(retryClass)
	var now time.Time = time.Now().UTC()
	job.Status, job.FinishedAt, job.LeaseExpiresAt = JobStatusFailed, &now, nil
	return UpdateJob(job)
}

// SetJobQueueID records the scheduler task id for a job.
func SetJobQueueID(jobID int, queueID string) (errResult error) {
	var (
		job   *Job
		found bool
		err   error
	)

	job, found, err = GetJobByID(jobID)
	if err != nil || !found {
		return err
	}
	job.QueueID = strings.TrimSpace(queueID)
	return UpdateJob(job)
}

// AppendJobLog appends a log line to a job.
func AppendJobLog(jobID int, stream JobLogStream, message string) (errResult error) {
	message = strings.TrimSpace(RedactSensitiveText(message))
	if message == "" {
		return nil
	}
	return JobLogs.Insert(&JobLog{
		JobID:     jobID,
		Stream:    stream,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	})
}

// JobLogsForJob returns all log lines for a job.
func JobLogsForJob(jobID int) (itemsResult []*JobLog, errResult error) {
	return JobLogs.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(JobLogs.FieldBySQLName("job_id"), gosqlite.OpEqual, jobID))
}

package db

import (
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

	job = &Job{
		UUID:              uuid,
		JobType:           input.JobType,
		Status:            JobStatusQueued,
		RequestedByUserID: input.RequestedByUserID,
		ProjectID:         input.ProjectID,
		ResourceID:        input.ResourceID,
		QueueID:           strings.TrimSpace(input.QueueID),
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
	job.StartedAt = &now
	job.UpdatedAt = now
	return Jobs.Update(job)
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
	job.FinishedAt = &now
	job.UpdatedAt = now
	return Jobs.Update(job)
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

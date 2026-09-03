package db

import (
	"testing"
	"time"
)

func TestCreateJobValidatesProjectAndResourceScope(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var org *Organization

	org = insertTestOrganization(t, "Lab", "lab", nil, now)
	var project *Project

	project = insertTestProject(t, "IT666", "it666", org.ID, now)
	var otherProject *Project

	otherProject = insertTestProject(t, "CS527", "cs527", org.ID, now)
	var resource *Resource

	resource = insertTestResource(t, project.ID, "student-vm", now)
	var job *Job
	var err error

	job, err = CreateJob(JobCreateInput{
		JobType:    JobTypeProxmox,
		ProjectID:  &project.ID,
		ResourceID: &resource.ID,
	})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if job.Status != JobStatusQueued || job.ProjectID == nil || *job.ProjectID != project.ID {
		t.Fatalf("expected queued project job, got %#v", job)
	}
	if _, err = CreateJob(JobCreateInput{
		JobType:    JobTypeProxmox,
		ProjectID:  &otherProject.ID,
		ResourceID: &resource.ID,
	}); err == nil {
		t.Fatal("expected cross-project job resource to be rejected")
	}
}

func TestJobStatusAndLogs(t *testing.T) {
	initTestDB(t)
	var (
		job *Job
		err error
	)

	job, err = CreateJob(JobCreateInput{
		JobType: JobTypeCleanup,
	})
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if err = SetJobQueueID(job.ID, "42"); err != nil {
		t.Fatalf("SetJobQueueID returned error: %v", err)
	}
	if err = MarkJobRunning(job.ID); err != nil {
		t.Fatalf("MarkJobRunning returned error: %v", err)
	}
	if err = AppendJobLog(job.ID, JobLogStreamSystem, "started token=do-not-log"); err != nil {
		t.Fatalf("AppendJobLog returned error: %v", err)
	}
	if err = MarkJobFinished(job.ID, JobStatusSucceeded); err != nil {
		t.Fatalf("MarkJobFinished returned error: %v", err)
	}
	var found bool

	job, found, err = GetJobByID(job.ID)
	if err != nil || !found {
		t.Fatalf("GetJobByID found=%v err=%v", found, err)
	}
	if job.QueueID != "42" || job.Status != JobStatusSucceeded || job.StartedAt == nil || job.FinishedAt == nil {
		t.Fatalf("expected completed queued job, got %#v", job)
	}
	var logs []*JobLog

	logs, err = JobLogsForJob(job.ID)
	if err != nil {
		t.Fatalf("JobLogsForJob returned error: %v", err)
	}
	if len(logs) != 1 || logs[0].Message != "started token=[REDACTED]" {
		t.Fatalf("expected one job log, got %#v", logs)
	}
}

func TestRecoverAbandonedJobsRetainsSafeHistory(t *testing.T) {
	initTestDB(t)
	var job *Job
	var err error
	if job, err = CreateJob(JobCreateInput{JobType: JobTypeCleanup, Operation: "test.recovery"}); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if err = MarkJobRunning(job.ID); err != nil {
		t.Fatalf("MarkJobRunning returned error: %v", err)
	}
	var past time.Time = time.Now().UTC().Add(-time.Minute)
	job, _, err = GetJobByID(job.ID)
	job.LeaseExpiresAt = &past
	if err = Jobs.Update(job); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	var count int
	if count, err = RecoverAbandonedJobs(time.Now().UTC()); err != nil || count != 1 {
		t.Fatalf("RecoverAbandonedJobs count=%d err=%v", count, err)
	}
	job, _, err = GetJobByID(job.ID)
	if job.Status != JobStatusFailed || job.ErrorCode != "worker_abandoned" || job.FinishedAt == nil {
		t.Fatalf("unexpected recovered job: %#v", job)
	}
	var logs []*JobLog
	if logs, err = JobLogsForJob(job.ID); err != nil || len(logs) != 1 {
		t.Fatalf("expected recovery log, logs=%#v err=%v", logs, err)
	}
}

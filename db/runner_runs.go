package db

import (
	"time"

	"github.com/z46-dev/gosqlite"
)

type RunnerRun struct {
	ID           int        `gosqlite:"id,primary,increment" json:"id"`
	UUID         string     `gosqlite:"uuid,unique,notnull" json:"uuid"`
	JobID        int        `gosqlite:"job_id,fkey:Job.id,unique,notnull" json:"job_id"`
	DeploymentID int        `gosqlite:"deployment_id,fkey:Deployment.id,notnull" json:"deployment_id"`
	Tool         string     `gosqlite:"tool,notnull" json:"tool"`
	Action       string     `gosqlite:"action,notnull" json:"action"`
	Workspace    string     `gosqlite:"workspace,notnull" json:"workspace"`
	SourceDigest string     `gosqlite:"source_digest" json:"source_digest,omitempty"`
	StateRef     string     `gosqlite:"state_ref" json:"state_ref,omitempty"`
	SummaryJSON  string     `gosqlite:"summary_json" json:"summary_json,omitempty"`
	StartedAt    *time.Time `gosqlite:"started_at" json:"started_at,omitempty"`
	FinishedAt   *time.Time `gosqlite:"finished_at" json:"finished_at,omitempty"`
	CreatedAt    time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
}

// CreateRunnerRun persists a durable runner execution record.
func CreateRunnerRun(jobID, deploymentID int, tool, action, workspace string) (runResult *RunnerRun, errResult error) {
	var uuid string
	if uuid, errResult = randomUUID(); errResult != nil {
		return
	}
	runResult = &RunnerRun{UUID: uuid, JobID: jobID, DeploymentID: deploymentID, Tool: tool, Action: action, Workspace: workspace, CreatedAt: time.Now().UTC()}
	errResult = RunnerRuns.Insert(runResult)
	return
}

// RunnerRunsForDeployment lists durable run records.
func RunnerRunsForDeployment(deploymentID int) (results []*RunnerRun, errResult error) {
	return RunnerRuns.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(RunnerRuns.FieldBySQLName("deployment_id"), gosqlite.OpEqual, deploymentID))
}

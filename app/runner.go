package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
	"github.com/UNHCSC/organesson/runner"
	jobscheduler "github.com/UNHCSC/organesson/scheduler"
	"github.com/gofiber/fiber/v2"
	"github.com/z46-dev/gasket"
)

type runnerIntegrationState struct {
	executor *runner.LocalExecutor
	tools    *runner.ToolRunner
}

var runnerIntegration runnerIntegrationState

// getRunnerHealth reports actual executable availability to administrators.
func getRunnerHealth(c *fiber.Ctx) (errResult error) {
	if !currentUserIsSiteAdmin(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "site administrator access required"})
	}
	var tofuErr error
	var ansibleErr error
	tofuErr = runnerIntegration.tools.OpenTofuHealth(context.Background(), nil)
	ansibleErr = runnerIntegration.tools.AnsibleHealth(context.Background(), nil)
	return c.JSON(fiber.Map{"opentofu": fiber.Map{"healthy": tofuErr == nil, "error": safeRunnerHealthError(tofuErr)}, "ansible": fiber.Map{"healthy": ansibleErr == nil, "error": safeRunnerHealthError(ansibleErr)}})
}

func safeRunnerHealthError(err error) string {
	if err == nil {
		return ""
	}
	return db.RedactSensitiveText(err.Error())
}

// configureRunnerIntegration builds and registers the bounded infrastructure runner.
func configureRunnerIntegration() (errResult error) {
	var timeout time.Duration = time.Duration(config.Config.Runner.TimeoutSeconds) * time.Second
	var outputLimit int = config.Config.Runner.MaxOutputBytes
	var workRoot string = config.Config.Runner.WorkDir
	var stateRoot string = config.Config.Runner.StateDir
	var sources []string = config.Config.Runner.AllowedSourceRoots
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	if outputLimit <= 0 {
		outputLimit = 1024 * 1024
	}
	if workRoot == "" {
		workRoot = "runner-data/work"
	}
	if stateRoot == "" {
		stateRoot = "runner-data/state"
	}
	if len(sources) == 0 {
		var exampleRoot string = "examples"
		var statErr error
		if _, statErr = os.Stat(exampleRoot); statErr != nil {
			exampleRoot = "../examples"
		}
		sources = []string{exampleRoot}
	}
	if config.Config.Runner.OpenTofuExecutable == "" {
		config.Config.Runner.OpenTofuExecutable = "tofu"
	}
	if config.Config.Runner.AnsibleExecutable == "" {
		config.Config.Runner.AnsibleExecutable = "ansible-playbook"
	}
	var executor *runner.LocalExecutor
	if executor, errResult = runner.NewLocalExecutor(runner.Config{WorkRoot: workRoot, StateRoot: stateRoot, AllowedSources: sources, OpenTofuBinary: config.Config.Runner.OpenTofuExecutable, AnsibleBinary: config.Config.Runner.AnsibleExecutable, Timeout: timeout, MaxOutputBytes: outputLimit}); errResult != nil {
		return
	}
	var tools *runner.ToolRunner
	if tools, errResult = runner.NewToolRunner(executor); errResult != nil {
		return
	}
	runnerIntegration = runnerIntegrationState{executor: executor, tools: tools}
	if jobscheduler.Default() != nil {
		errResult = jobscheduler.Default().RegisterConsumer(jobscheduler.TaskTypeRunnerAction, consumeRunnerAction)
	}
	return
}

// postDeploymentRun queues a runner action with explicit apply authorization.
func postDeploymentRun(c *fiber.Ctx) (errResult error) {
	var deploymentID int
	if deploymentID, errResult = c.ParamsInt("id"); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid deployment id"})
	}
	var deployment *db.Deployment
	if deployment, errResult = db.Deployments.Select(deploymentID); errResult != nil || deployment == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "deployment not found"})
	}
	var request struct {
		Action    string `json:"action"`
		Confirm   bool   `json:"confirm"`
		PlanRunID int    `json:"planRunID"`
	}
	if errResult = c.BodyParser(&request); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	var permission db.PermissionKey
	var jobType db.JobType
	switch request.Action {
	case "tofu.plan":
		permission, jobType = db.PermissionTerraformPlan, db.JobTypeTerraform
	case "tofu.apply":
		if !request.Confirm || request.PlanRunID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "apply requires confirmation and a planRunID"})
		}
		permission, jobType = db.PermissionTerraformApply, db.JobTypeTerraform
	case "ansible.run", "ansible.check":
		permission, jobType = db.PermissionAnsibleRun, db.JobTypeAnsible
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported runner action"})
	}
	var allowed bool
	if allowed, errResult = currentUserCan(c, permission, db.RoleBindingScopeProject, &deployment.ProjectID); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var key string = strings.TrimSpace(c.Get("Idempotency-Key"))
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Idempotency-Key header is required"})
	}
	var user *db.User = currentDBUser(c)
	var digest [sha256.Size]byte = sha256.Sum256([]byte(strconv.Itoa(user.ID) + ":" + strconv.Itoa(deployment.ID) + ":" + request.Action + ":" + key))
	key = hex.EncodeToString(digest[:])
	var job *db.Job
	var found bool
	if job, found, errResult = db.FindJobByIdempotencyKey(key); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to check idempotency key"})
	}
	if found {
		return c.JSON(job)
	}
	if job, errResult = db.CreateJob(db.JobCreateInput{JobType: jobType, RequestedByUserID: &user.ID, ProjectID: &deployment.ProjectID, Operation: request.Action, IdempotencyKey: key, OperationKey: fmt.Sprintf("deployment:%d:%s:%s", deployment.ID, request.Action, key), Input: map[string]any{"deployment_id": deployment.ID, "action": request.Action, "plan_run_id": request.PlanRunID}}); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create runner job"})
	}
	if _, errResult = jobscheduler.EnqueueJobTask(job, jobscheduler.TaskTypeRunnerAction, jobscheduler.JobPayload{Operation: request.Action, Metadata: map[string]string{"deploymentID": strconv.Itoa(deployment.ID), "planRunID": strconv.Itoa(request.PlanRunID)}}); errResult != nil {
		_ = db.FailJob(job.ID, "enqueue_failed", "The runner action could not be queued.", "transient")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "failed to enqueue runner job"})
	}
	return c.Status(fiber.StatusAccepted).JSON(job)
}

// getDeploymentRuns lists durable artifacts and summaries for a visible deployment.
func getDeploymentRuns(c *fiber.Ctx) (errResult error) {
	var deploymentID int
	if deploymentID, errResult = c.ParamsInt("id"); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid deployment id"})
	}
	var deployment *db.Deployment
	if deployment, errResult = db.Deployments.Select(deploymentID); errResult != nil || deployment == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "deployment not found"})
	}
	var allowed bool
	if allowed, errResult = currentUserCanViewProjectID(c, deployment.ProjectID); errResult != nil || !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var runs []*db.RunnerRun
	if runs, errResult = db.RunnerRunsForDeployment(deployment.ID); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load runs"})
	}
	return c.JSON(runs)
}

func consumeRunnerAction(_ int, payload []byte) (result gasket.TaskConsumerResult) {
	var jobPayload jobscheduler.JobPayload
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
	var deploymentID int
	if deploymentID, err = strconv.Atoi(jobPayload.Metadata["deploymentID"]); err != nil {
		result.Error = err
		return
	}
	var deployment *db.Deployment
	if deployment, err = db.Deployments.Select(deploymentID); err != nil || deployment == nil || deployment.ProjectID != *job.ProjectID {
		_ = db.FailJob(job.ID, "deployment_missing", "The deployment no longer exists.", "permanent")
		result.Success = true
		return
	}
	if err = revalidateRunnerPermission(job, deployment, jobPayload.Operation); err != nil {
		_ = db.FailJob(job.ID, "authorization_revoked", "Runner permission was revoked before execution.", "permanent")
		result.Success = true
		return
	}
	if err = db.MarkJobRunning(job.ID); err != nil {
		result.Error = err
		return
	}
	var run *db.RunnerRun
	var workspace string = fmt.Sprintf("deployment-%d/run-%d", deployment.ID, job.ID)
	if jobPayload.Operation == "tofu.apply" {
		var planRunID int
		if planRunID, err = strconv.Atoi(jobPayload.Metadata["planRunID"]); err != nil {
			_ = db.FailJob(job.ID, "plan_missing", "The selected plan is invalid.", "permanent")
			result.Success = true
			return
		}
		var planRun *db.RunnerRun
		if planRun, err = db.RunnerRuns.Select(planRunID); err != nil || planRun == nil || planRun.DeploymentID != deployment.ID || planRun.Action != "tofu.plan" || planRun.FinishedAt == nil {
			_ = db.FailJob(job.ID, "plan_missing", "A successful plan from this deployment is required.", "permanent")
			result.Success = true
			return
		}
		var planJob *db.Job
		if planJob, found, err = db.GetJobByID(planRun.JobID); err != nil || !found || planJob.Status != db.JobStatusSucceeded {
			_ = db.FailJob(job.ID, "plan_failed", "Only a successful plan may be applied.", "permanent")
			result.Success = true
			return
		}
		workspace = planRun.Workspace
	}
	var tool string = strings.SplitN(jobPayload.Operation, ".", 2)[0]
	if run, err = db.CreateRunnerRun(job.ID, deployment.ID, tool, jobPayload.Operation, workspace); err != nil {
		result.Error = err
		return
	}
	var started time.Time = time.Now().UTC()
	run.StartedAt = &started
	_ = db.RunnerRuns.Update(run)
	var commandResult runner.Result
	var log runner.LogFunc = func(stream db.JobLogStream, line string) error { return db.AppendJobLog(job.ID, stream, line) }
	var runContext context.Context
	var cancel context.CancelFunc
	var stopCancellationWatch chan struct{}
	runContext, cancel, stopCancellationWatch = runnerJobContext(job.ID)
	if err = executeRunnerAction(runContext, deployment, run, jobPayload.Operation, &commandResult, log); err != nil {
		var cancelled bool
		cancelled, _ = db.JobCancellationRequested(job.ID)
		if cancelled {
			_ = db.AppendJobLog(job.ID, db.JobLogStreamSystem, "runner process group terminated after cancellation")
			_ = db.MarkJobFinished(job.ID, db.JobStatusCancelled)
		} else {
			_ = db.FailJob(job.ID, "runner_failed", db.RedactSensitiveText(err.Error()), "permanent")
		}
	} else {
		_ = db.MarkJobFinished(job.ID, db.JobStatusSucceeded)
	}
	close(stopCancellationWatch)
	cancel()
	var finished time.Time = time.Now().UTC()
	run.FinishedAt = &finished
	var summary []byte
	var persistedSummary map[string]any = map[string]any{"command": commandResult}
	if run.SummaryJSON != "" {
		var planSummary runner.PlanSummary
		if json.Unmarshal([]byte(run.SummaryJSON), &planSummary) == nil {
			persistedSummary["plan"] = planSummary
		}
	}
	summary, _ = json.Marshal(persistedSummary)
	run.SummaryJSON = string(summary)
	_ = db.RunnerRuns.Update(run)
	result.Success = true
	return
}

func executeRunnerAction(ctx context.Context, deployment *db.Deployment, run *db.RunnerRun, action string, commandResult *runner.Result, log runner.LogFunc) (errResult error) {
	var version *db.BlueprintVersion
	var document db.BlueprintDocument
	if version, errResult = db.BlueprintVersions.Select(deployment.BlueprintVersionID); errResult != nil || version == nil {
		return fmt.Errorf("blueprint version was not found")
	}
	if document, errResult = db.BlueprintDocumentForVersion(version); errResult != nil {
		return
	}
	switch action {
	case "tofu.plan":
		if run.SourceDigest, errResult = runnerIntegration.executor.MaterializeSource(ctx, document.OpenTofuModule, run.Workspace, log); errResult != nil {
			return
		}
		var variables []byte
		if variables, errResult = deploymentVariables(deployment.ID); errResult != nil {
			return
		}
		var variablePath string = filepath.Join(config.Config.Runner.WorkDir, run.Workspace, "organesson.auto.tfvars.json")
		if errResult = os.WriteFile(variablePath, variables, 0600); errResult != nil {
			return
		}
		*commandResult, errResult = runnerIntegration.tools.Plan(ctx, run.Workspace, "organesson.auto.tfvars.json", log)
		if errResult == nil {
			run.StateRef = filepath.Join(run.Workspace, "planned.tfplan")
			var planSummary runner.PlanSummary
			if planSummary, errResult = runnerIntegration.tools.PlanSummary(ctx, run.Workspace, "planned.tfplan"); errResult == nil {
				var summary []byte
				summary, _ = json.Marshal(planSummary)
				run.SummaryJSON = string(summary)
			}
		}
	case "tofu.apply":
		*commandResult, errResult = runnerIntegration.tools.Apply(ctx, run.Workspace, "planned.tfplan", log)
	case "ansible.run", "ansible.check":
		if run.SourceDigest, errResult = runnerIntegration.executor.MaterializeSource(ctx, document.AnsibleProject, run.Workspace, log); errResult != nil {
			return
		}
		*commandResult, errResult = runnerIntegration.tools.Run(ctx, run.Workspace, "inventory.yml", "site.yml", "", action == "ansible.check", log)
	}
	return
}

func runnerJobContext(jobID int) (contextResult context.Context, cancelResult context.CancelFunc, stopResult chan struct{}) {
	contextResult, cancelResult = context.WithCancel(context.Background())
	stopResult = make(chan struct{})
	go func() {
		var ticker *time.Ticker = time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var requested bool
				if requested, _ = db.JobCancellationRequested(jobID); requested {
					cancelResult()
					return
				}
			case <-stopResult:
				return
			}
		}
	}()
	return
}

func deploymentVariables(deploymentID int) (result []byte, errResult error) {
	var resources []*db.DeploymentResource
	var allocations []*db.Allocation
	if resources, errResult = db.DeploymentResources.SelectAll(); errResult != nil {
		return
	}
	if allocations, errResult = db.Allocations.SelectAll(); errResult != nil {
		return
	}
	var filteredResources []*db.DeploymentResource
	var filteredAllocations []*db.Allocation
	for _, resource := range resources {
		if resource.DeploymentID == deploymentID {
			filteredResources = append(filteredResources, resource)
		}
	}
	for _, allocation := range allocations {
		if allocation.DeploymentID == deploymentID && allocation.ReleasedAt == nil {
			filteredAllocations = append(filteredAllocations, allocation)
		}
	}
	return json.Marshal(map[string]any{"deployment_id": deploymentID, "resources": filteredResources, "allocations": filteredAllocations})
}

func revalidateRunnerPermission(job *db.Job, deployment *db.Deployment, action string) (errResult error) {
	var user *db.User
	if user, errResult = db.Users.Select(*job.RequestedByUserID); errResult != nil || user == nil {
		return fmt.Errorf("requester was not found")
	}
	if user.IsSystemAdmin {
		return nil
	}
	var permission db.PermissionKey = db.PermissionTerraformPlan
	if action == "tofu.apply" {
		permission = db.PermissionTerraformApply
	} else if strings.HasPrefix(action, "ansible.") {
		permission = db.PermissionAnsibleRun
	}
	var groups []int
	var allowed bool
	if groups, errResult = db.CloudGroupIDsForUser(user.ID); errResult != nil {
		return
	}
	if allowed, errResult = db.HasPermission(db.PermissionCheck{UserID: user.ID, GroupIDs: groups, Permission: permission, ScopeType: db.RoleBindingScopeProject, ScopeID: &deployment.ProjectID}); errResult != nil || !allowed {
		return fmt.Errorf("permission denied")
	}
	return
}

func currentUserCanViewProjectID(c *fiber.Ctx, projectID int) (allowedResult bool, errResult error) {
	var project *db.Project
	var found bool
	if project, found, errResult = db.GetProjectByID(projectID); errResult != nil || !found {
		return false, errResult
	}
	return currentUserCanViewProject(c, project)
}

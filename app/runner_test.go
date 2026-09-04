package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
	"github.com/UNHCSC/organesson/runner"
	jobscheduler "github.com/UNHCSC/organesson/scheduler"
	"github.com/gofiber/fiber/v2"
)

func TestRunnerActionsRunThroughSchedulerAndPersistArtifacts(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var root string = t.TempDir()
	var source string = filepath.Join(root, "source")
	var work string = filepath.Join(root, "work")
	var state string = filepath.Join(root, "state")
	var err error
	if err = os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(source, "main.tf"), []byte("terraform {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var executable string = filepath.Join(root, "tofu-test")
	if err = os.WriteFile(executable, []byte("#!/bin/sh\nif [ \"$1\" = show ]; then printf '%s\\n' '{\"resource_changes\":[{\"change\":{\"actions\":[\"create\"]}}]}'; exit 0; fi\nfor argument in \"$@\"; do case \"$argument\" in -out=*) touch \"${argument#-out=}\";; esac; done\nprintf '%s\\n' 'token=runner-secret' \"tofu $*\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	var executor *runner.LocalExecutor
	if executor, err = runner.NewLocalExecutor(runner.Config{WorkRoot: work, StateRoot: state, AllowedSources: []string{source}, OpenTofuBinary: executable, AnsibleBinary: executable, Timeout: 5 * time.Second, MaxOutputBytes: 4096}); err != nil {
		t.Fatal(err)
	}
	var tools *runner.ToolRunner
	if tools, err = runner.NewToolRunner(executor); err != nil {
		t.Fatal(err)
	}
	var originalIntegration runnerIntegrationState = runnerIntegration
	var originalRunnerConfig = config.Config.Runner
	runnerIntegration = runnerIntegrationState{executor: executor, tools: tools}
	config.Config.Runner.WorkDir = work
	config.Config.Runner.StateDir = state
	t.Cleanup(func() { runnerIntegration = originalIntegration; config.Config.Runner = originalRunnerConfig })
	var digest string
	if digest, err = executor.DigestAllowedSource(source); err != nil {
		t.Fatal(err)
	}
	var project *db.Project = createResourceAPIProject(t, "Runner Project")
	var blueprint *db.Blueprint
	if blueprint, err = db.CreateBlueprint(project.ID, "Runner", "runner", ""); err != nil {
		t.Fatal(err)
	}
	var version *db.BlueprintVersion
	var reference string = source + "?ref=" + digest
	if version, err = db.PublishBlueprintVersion(blueprint.ID, db.BlueprintDocument{FormatVersion: 1, OpenTofuModule: reference, AnsibleProject: reference, NamePattern: "{{deployment}}-{{resource}}", Networks: []db.BlueprintNetworkSpec{{Key: "lan", Kind: "isolated"}}, Resources: []db.BlueprintResourceSpec{{Key: "vm", Kind: "vm", Template: "test-v1", VCPU: 1, MemoryMB: 512, DiskGB: 8, Networks: []string{"lan"}}}}, nil); err != nil {
		t.Fatal(err)
	}
	var group *db.CloudGroup = &db.CloudGroup{UUID: "runner-group", Name: "Runner Group", Slug: "runner-group", GroupType: db.GroupTypeStudentGroup, OwnerScopeType: db.RoleBindingScopeProject, OwnerScopeID: &project.ID, SyncSource: db.CloudGroupSyncSourceLocal, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = db.CloudGroups.Insert(group); err != nil {
		t.Fatal(err)
	}
	var deployment *db.Deployment = &db.Deployment{UUID: "runner-deployment", ProjectID: project.ID, BlueprintVersionID: version.ID, GroupID: group.ID, Name: "runner-g01", Status: "planned", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = db.Deployments.Insert(deployment); err != nil {
		t.Fatal(err)
	}
	var user *db.User = ensureResourceAPIUser(t, "runner-admin")
	user.IsSystemAdmin = true
	if err = db.Users.Update(user); err != nil {
		t.Fatal(err)
	}
	var job *db.Job
	if job, err = db.CreateJob(db.JobCreateInput{JobType: db.JobTypeTerraform, RequestedByUserID: &user.ID, ProjectID: &project.ID, Operation: "tofu.plan", OperationKey: "runner-test", Input: map[string]any{"deployment_id": deployment.ID}}); err != nil {
		t.Fatal(err)
	}
	var service *jobscheduler.Service
	if service, err = jobscheduler.Init(filepath.Join(root, "tasks.db")); err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if err = service.RegisterConsumer(jobscheduler.TaskTypeRunnerAction, consumeRunnerAction); err != nil {
		t.Fatal(err)
	}
	if _, err = service.EnqueueJobTask(job, jobscheduler.TaskTypeRunnerAction, jobscheduler.JobPayload{Operation: "tofu.plan", Metadata: map[string]string{"deploymentID": strconv.Itoa(deployment.ID)}}); err != nil {
		t.Fatal(err)
	}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	var done chan error = make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		if job, _, err = db.GetJobByID(job.ID); err == nil && job.Status == db.JobStatusSucceeded {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.Status != db.JobStatusSucceeded {
		t.Fatalf("runner job did not succeed: %#v", job)
	}
	var runs []*db.RunnerRun
	if runs, err = db.RunnerRunsForDeployment(deployment.ID); err != nil || len(runs) != 1 || runs[0].StateRef == "" || runs[0].SourceDigest != digest {
		if len(runs) == 1 {
			t.Fatalf("runner artifacts were not persisted: run=%+v expected_digest=%q err=%v", *runs[0], digest, err)
		}
		t.Fatalf("runner artifacts were not persisted: count=%d err=%v", len(runs), err)
	}
	if !strings.Contains(runs[0].SummaryJSON, `"add":1`) {
		t.Fatalf("plan summary was not persisted: %s", runs[0].SummaryJSON)
	}
	var fiberApp *fiber.App = newAuthenticatedFiberApp()
	fiberApp.Post("/api/v1/deployments/:id/runs", postDeploymentRun)
	var token string = authenticateTestUser(t, user.Username, false)
	var response *http.Response = runnerAPIRequest(t, fiberApp, token, deployment.ID, `{"action":"tofu.apply","confirm":false}`, "apply-unconfirmed")
	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("unconfirmed apply status=%d", response.StatusCode)
	}
	var applyJob *db.Job
	response = runnerAPIRequest(t, fiberApp, token, deployment.ID, `{"action":"tofu.apply","confirm":true}`, "apply-confirmed")
	if response.StatusCode != fiber.StatusAccepted {
		t.Fatalf("confirmed apply status=%d", response.StatusCode)
	}
	if err = json.NewDecoder(response.Body).Decode(&applyJob); err != nil {
		t.Fatal(err)
	}
	if applyJob = waitForRunnerJob(t, applyJob.ID); applyJob.Status != db.JobStatusSucceeded {
		t.Fatalf("apply job did not succeed: %#v", applyJob)
	}
	var ansibleJob *db.Job
	response = runnerAPIRequest(t, fiberApp, token, deployment.ID, `{"action":"ansible.check"}`, "ansible-check")
	if response.StatusCode != fiber.StatusAccepted {
		t.Fatalf("Ansible check status=%d", response.StatusCode)
	}
	if err = json.NewDecoder(response.Body).Decode(&ansibleJob); err != nil {
		t.Fatal(err)
	}
	if ansibleJob = waitForRunnerJob(t, ansibleJob.ID); ansibleJob.Status != db.JobStatusSucceeded {
		t.Fatalf("Ansible job did not succeed: %#v", ansibleJob)
	}
	response = runnerAPIRequest(t, fiberApp, token, deployment.ID, `{"action":"tofu.destroy","confirm":false}`, "destroy-unconfirmed")
	if response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("unconfirmed destroy status=%d", response.StatusCode)
	}
	var destroyJob *db.Job
	response = runnerAPIRequest(t, fiberApp, token, deployment.ID, `{"action":"tofu.destroy","confirm":true}`, "destroy-confirmed")
	if response.StatusCode != fiber.StatusAccepted {
		t.Fatalf("confirmed destroy status=%d", response.StatusCode)
	}
	if err = json.NewDecoder(response.Body).Decode(&destroyJob); err != nil {
		t.Fatal(err)
	}
	if destroyJob = waitForRunnerJob(t, destroyJob.ID); destroyJob.Status != db.JobStatusSucceeded {
		t.Fatalf("destroy job did not succeed: %#v", destroyJob)
	}
	cancel()
	if err = <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if runs, err = db.RunnerRunsForDeployment(deployment.ID); err != nil || len(runs) != 4 {
		t.Fatalf("expected plan, apply, Ansible, and destroy runs: %#v err=%v", runs, err)
	}
	for _, completedJob := range []*db.Job{job, applyJob, ansibleJob, destroyJob} {
		var logs []*db.JobLog
		if logs, err = db.JobLogsForJob(completedJob.ID); err != nil || len(logs) == 0 {
			t.Fatalf("runner logs missing for job %d: %#v err=%v", completedJob.ID, logs, err)
		}
		for _, log := range logs {
			if strings.Contains(log.Message, "runner-secret") {
				t.Fatalf("secret reached stored logs for job %d", completedJob.ID)
			}
		}
	}
}

func runnerAPIRequest(t *testing.T, fiberApp *fiber.App, token string, deploymentID int, body, idempotencyKey string) (responseResult *http.Response) {
	t.Helper()
	var request *http.Request = httptest.NewRequest(http.MethodPost, "/api/v1/deployments/"+strconv.Itoa(deploymentID)+"/runs", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	var err error
	if responseResult, err = testFiberRequest(fiberApp, request); err != nil {
		t.Fatal(err)
	}
	return
}

func waitForRunnerJob(t *testing.T, jobID int) (jobResult *db.Job) {
	t.Helper()
	var found bool
	var err error
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		if jobResult, found, err = db.GetJobByID(jobID); err == nil && found && (jobResult.Status == db.JobStatusSucceeded || jobResult.Status == db.JobStatusFailed || jobResult.Status == db.JobStatusCancelled) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("runner job %d did not finish: %#v err=%v", jobID, jobResult, err)
	return
}

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/db"
	"github.com/UNHCSC/organesson/proxmox"
	jobscheduler "github.com/UNHCSC/organesson/scheduler"
	"github.com/gofiber/fiber/v2"
)

func TestAssignedPowerAndConsoleStayTagAndPermissionScoped(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var project *db.Project = createResourceAPIProject(t, "VM Operations")
	var assigned *db.User = ensureResourceAPIUser(t, "assigned-student")
	var outsider *db.User = ensureResourceAPIUser(t, "other-student")
	grantProjectRole(t, project.ID, assigned.ID, db.ProjectRoleOperator)
	var resource *db.Resource
	var err error
	if resource, err = db.CreateResource(db.ResourceCreateInput{ProjectID: project.ID, Name: "Student VM", ResourceType: db.ResourceTypeVM}); err != nil {
		t.Fatalf("create resource: %v", err)
	}
	var now time.Time = time.Now().UTC()
	var cluster *db.ProxmoxCluster = &db.ProxmoxCluster{UUID: "test-cluster", Name: "test-cluster", APIURL: "https://pve.test", VerifyTLS: true, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err = db.ProxmoxClusters.Insert(cluster); err != nil {
		t.Fatalf("insert cluster: %v", err)
	}
	var node *db.ProxmoxNode = &db.ProxmoxNode{ClusterID: cluster.ID, Name: "pve-a", CreatedAt: now, UpdatedAt: now}
	if err = db.ProxmoxNodes.Insert(node); err != nil {
		t.Fatalf("insert node: %v", err)
	}
	if err = db.VirtualMachines.Insert(&db.VirtualMachine{ResourceID: resource.ID, ClusterID: cluster.ID, NodeID: &node.ID, ProxmoxVMID: 101, Name: resource.Name, CPUCores: 1, MemoryMB: 512, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("insert vm: %v", err)
	}
	var fake *proxmox.FakeService = &proxmox.FakeService{Guests: []proxmox.Guest{{VMID: 101, Node: "pve-a", Name: resource.Name, Kind: "qemu", Tags: []string{proxmox.DefaultManagedTag}}}, Tasks: map[string]proxmox.Task{"UPID:test": {ID: "UPID:test", Status: "stopped", ExitStatus: "OK"}}, ConsoleTickets: map[int]proxmox.ConsoleTicket{101: {Ticket: "short-lived-secret", Port: 5900, ExpiresAt: now.Add(time.Minute)}}}
	var original proxmoxIntegrationState = proxmoxIntegration
	proxmoxIntegration = proxmoxIntegrationState{service: fake, clusterIdentity: "test-cluster", managedTag: proxmox.DefaultManagedTag, enabled: true}
	t.Cleanup(func() { proxmoxIntegration = original })
	var schedulerService *jobscheduler.Service
	if schedulerService, err = jobscheduler.Init(filepath.Join(t.TempDir(), "tasks.db")); err != nil {
		t.Fatalf("scheduler init: %v", err)
	}
	defer schedulerService.Close()
	if err = schedulerService.RegisterConsumer(jobscheduler.TaskTypeProxmoxAction, consumeProxmoxAction); err != nil {
		t.Fatalf("register consumer: %v", err)
	}
	var app *fiber.App = newAuthenticatedFiberApp()
	app.Post("/api/v1/resources/:id/actions/:action", postResourcePowerAction)
	app.Post("/api/v1/resources/:id/console-sessions", postResourceConsoleSession)
	var path string = "/api/v1/resources/" + strconv.Itoa(resource.ID) + "/actions/start"
	var response *http.Response = resourceAPIRequest(t, app, authenticateTestUser(t, outsider.Username, false), http.MethodPost, path, "")
	if response.StatusCode != 403 {
		t.Fatalf("expected outsider 403, got %d", response.StatusCode)
	}
	var request *http.Request = httptest.NewRequest(http.MethodPost, path, nil)
	request.Header.Set("Authorization", "Bearer "+authenticateTestUser(t, assigned.Username, false))
	request.Header.Set("Idempotency-Key", "start-once")
	if response, err = testFiberRequest(app, request); err != nil || response.StatusCode != 202 {
		t.Fatalf("power status=%d err=%v", response.StatusCode, err)
	}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	var done chan error = make(chan error, 1)
	go func() { done <- schedulerService.Run(ctx) }()
	var jobs []*db.Job
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		jobs, _ = db.ListJobs()
		if len(jobs) == 1 && jobs[0].Status == db.JobStatusSucceeded {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if err = <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("scheduler: %v", err)
	}
	if len(fake.Actions) != 1 || len(jobs) != 1 || jobs[0].Status != db.JobStatusSucceeded {
		t.Fatalf("unexpected action=%#v jobs=%#v", fake.Actions, jobs)
	}
	request = httptest.NewRequest(http.MethodPost, path, nil)
	request.Header.Set("Authorization", "Bearer "+authenticateTestUser(t, assigned.Username, false))
	request.Header.Set("Idempotency-Key", "start-once")
	if response, err = testFiberRequest(app, request); err != nil || response.StatusCode != 200 || len(fake.Actions) != 1 {
		t.Fatalf("duplicate request created work: status=%d actions=%#v err=%v", response.StatusCode, fake.Actions, err)
	}
	response = resourceAPIRequest(t, app, authenticateTestUser(t, assigned.Username, false), http.MethodPost, "/api/v1/resources/"+strconv.Itoa(resource.ID)+"/console-sessions", "")
	var body map[string]any
	if err = json.NewDecoder(response.Body).Decode(&body); err != nil || strings.Contains(fmt.Sprint(body), "short-lived-secret") {
		t.Fatalf("console response leaked ticket: %#v err=%v", body, err)
	}
	fake.Guests[0].Tags = nil
	if _, err = managedMachineGuest(context.Background(), &db.VirtualMachine{ClusterID: cluster.ID, NodeID: &node.ID, ProxmoxVMID: 101}); err == nil {
		t.Fatal("expected lost managed tag to block operation")
	}
	fake.Guests[0].Tags = []string{proxmox.DefaultManagedTag}
	fake.Tasks["UPID:test"] = proxmox.Task{ID: "UPID:test", Status: "running"}
	var timeoutJob *db.Job
	var found bool
	if timeoutJob, err = db.CreateJob(db.JobCreateInput{JobType: db.JobTypeProxmox, RequestedByUserID: &assigned.ID, ProjectID: &project.ID, ResourceID: &resource.ID, Operation: "vm.start", OperationKey: "timeout-test"}); err != nil {
		t.Fatal(err)
	}
	var payload []byte
	if payload, err = json.Marshal(jobscheduler.JobPayload{JobID: timeoutJob.ID, Operation: "start"}); err != nil {
		t.Fatal(err)
	}
	var oldPollLimit int = proxmoxTaskPollLimit
	var oldPollInterval time.Duration = proxmoxTaskPollInterval
	proxmoxTaskPollLimit = 1
	proxmoxTaskPollInterval = 0
	consumeProxmoxAction(0, payload)
	proxmoxTaskPollLimit = oldPollLimit
	proxmoxTaskPollInterval = oldPollInterval
	if timeoutJob, found, err = db.GetJobByID(timeoutJob.ID); err != nil || !found || timeoutJob.Status != db.JobStatusFailed || timeoutJob.ErrorCode != "provider_timeout" {
		t.Fatalf("timeout job=%#v found=%t err=%v", timeoutJob, found, err)
	}
	var lostJob *db.Job
	if lostJob, err = db.CreateJob(db.JobCreateInput{JobType: db.JobTypeProxmox, RequestedByUserID: &assigned.ID, ProjectID: &project.ID, ResourceID: &resource.ID, Operation: "vm.start", OperationKey: "lost-test"}); err != nil {
		t.Fatal(err)
	}
	if err = db.ArchiveResource(resource); err != nil {
		t.Fatal(err)
	}
	if payload, err = json.Marshal(jobscheduler.JobPayload{JobID: lostJob.ID, Operation: "start"}); err != nil {
		t.Fatal(err)
	}
	consumeProxmoxAction(0, payload)
	if lostJob, found, err = db.GetJobByID(lostJob.ID); err != nil || !found || lostJob.Status != db.JobStatusFailed || lostJob.ErrorCode != "resource_missing" {
		t.Fatalf("lost-resource job=%#v found=%t err=%v", lostJob, found, err)
	}
	var audits []*db.AuditEvent
	if audits, err = db.AuditEventsForProject(&project.ID); err != nil || len(audits) < 3 {
		t.Fatalf("expected action job audits, count=%d err=%v", len(audits), err)
	}
}

func TestConsoleSessionRejectsWrongUserAndExpiry(t *testing.T) {
	var now time.Time = time.Now().UTC()
	var session consoleSession = consoleSession{ID: "opaque", UserID: 10, ExpiresAt: now.Add(time.Minute)}
	if !consoleSessionValid(session, 10, now) {
		t.Fatal("expected active owner session")
	}
	if consoleSessionValid(session, 11, now) {
		t.Fatal("expected wrong user rejection")
	}
	if consoleSessionValid(session, 10, now.Add(2*time.Minute)) {
		t.Fatal("expected expired session rejection")
	}
}

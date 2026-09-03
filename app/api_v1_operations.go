package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/UNHCSC/organesson/db"
	"github.com/UNHCSC/organesson/proxmox"
	jobscheduler "github.com/UNHCSC/organesson/scheduler"
	"github.com/fasthttp/websocket"
	fiberwebsocket "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/z46-dev/gasket"
)

type consoleSession struct {
	ID         string
	UserID     int
	ResourceID int
	Ticket     string
	Port       int
	Node       string
	VMID       int
	ExpiresAt  time.Time
}

var operationRate = struct {
	sync.Mutex
	last map[string]time.Time
}{last: make(map[string]time.Time)}
var consoleSessions = struct {
	sync.Mutex
	items map[string]consoleSession
}{items: make(map[string]consoleSession)}

var proxmoxTaskPollInterval time.Duration = 500 * time.Millisecond
var proxmoxTaskPollLimit int = 120

func powerPermission(action string) (permissionResult db.PermissionKey, okResult bool) {
	switch action {
	case "start":
		return db.PermissionVMStart, true
	case "stop", "shutdown":
		return db.PermissionVMStop, true
	case "reboot":
		return db.PermissionVMReboot, true
	}
	return
}

func operationResource(c *fiber.Ctx, permission db.PermissionKey) (resourceResult *db.Resource, machineResult *db.VirtualMachine, errResult error) {
	var id int
	var found, allowed bool
	if id, errResult = strconv.Atoi(c.Params("id")); errResult != nil {
		return nil, nil, fiber.ErrBadRequest
	}
	if resourceResult, found, errResult = db.GetResourceByID(id); errResult != nil || !found || resourceResult.ResourceType != db.ResourceTypeVM {
		return nil, nil, fiber.ErrNotFound
	}
	if allowed, errResult = currentUserCan(c, permission, db.RoleBindingScopeResource, &id); errResult != nil {
		return
	}
	if !allowed {
		if allowed, errResult = currentUserCan(c, permission, db.RoleBindingScopeProject, &resourceResult.ProjectID); errResult != nil {
			return
		}
	}
	if !allowed {
		return nil, nil, fiber.ErrForbidden
	}
	if machineResult, found, errResult = db.VirtualMachineForResource(id); errResult != nil || !found {
		return nil, nil, fiber.ErrNotFound
	}
	return
}

func enforceOperationRate(userID, resourceID int, kind string) (errResult error) {
	var key string = fmt.Sprintf("%d:%d:%s", userID, resourceID, kind)
	operationRate.Lock()
	defer operationRate.Unlock()
	var now time.Time = time.Now().UTC()
	var last time.Time = operationRate.last[key]
	if !last.IsZero() && now.Sub(last) < time.Second {
		return fmt.Errorf("rate limit exceeded")
	}
	operationRate.last[key] = now
	return
}

func postResourcePowerAction(c *fiber.Ctx) (errResult error) {
	var action string = strings.ToLower(c.Params("action"))
	var permission db.PermissionKey
	var ok bool
	if permission, ok = powerPermission(action); !ok {
		return c.Status(404).JSON(fiber.Map{"error": "unknown power action"})
	}
	var resource *db.Resource
	if resource, _, errResult = operationResource(c, permission); errResult != nil {
		return c.Status(fiberStatus(errResult)).JSON(fiber.Map{"error": "resource is not available"})
	}
	var user *db.User = currentDBUser(c)
	var key string = strings.TrimSpace(c.Get("Idempotency-Key"))
	if key == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Idempotency-Key header is required"})
	}
	key = fmt.Sprintf("power:%d:%d:%s:%s", user.ID, resource.ID, action, key)
	var job *db.Job
	var found bool
	if job, found, errResult = db.FindJobByIdempotencyKey(key); errResult != nil {
		return c.SendStatus(500)
	}
	if found {
		return c.JSON(job)
	}
	if errResult = enforceOperationRate(user.ID, resource.ID, "power"); errResult != nil {
		return c.Status(429).JSON(fiber.Map{"error": errResult.Error()})
	}
	if job, errResult = db.CreateJob(db.JobCreateInput{JobType: db.JobTypeProxmox, RequestedByUserID: &user.ID, ProjectID: &resource.ProjectID, ResourceID: &resource.ID, Operation: "vm." + action, IdempotencyKey: key, OperationKey: key, MaxAttempts: 1}); errResult != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create job"})
	}
	if _, errResult = jobscheduler.EnqueueJobTask(job, jobscheduler.TaskTypeProxmoxAction, jobscheduler.JobPayload{Operation: action}); errResult != nil {
		_ = db.FailJob(job.ID, "enqueue_failed", "Power action could not be queued.", "transient")
		return c.SendStatus(503)
	}
	return c.Status(202).JSON(job)
}

func postResourceConsoleSession(c *fiber.Ctx) (errResult error) {
	var resource *db.Resource
	var machine *db.VirtualMachine
	if resource, machine, errResult = operationResource(c, db.PermissionVMConsole); errResult != nil {
		return c.Status(fiberStatus(errResult)).JSON(fiber.Map{"error": "resource is not available"})
	}
	var user *db.User = currentDBUser(c)
	if errResult = enforceOperationRate(user.ID, resource.ID, "console"); errResult != nil {
		return c.Status(429).JSON(fiber.Map{"error": errResult.Error()})
	}
	var job *db.Job
	if job, errResult = db.CreateJob(db.JobCreateInput{JobType: db.JobTypeProxmox, RequestedByUserID: &user.ID, ProjectID: &resource.ProjectID, ResourceID: &resource.ID, Operation: "vm.console", OperationKey: "console-session", MaxAttempts: 1}); errResult != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to record console operation"})
	}
	var guest proxmox.Guest
	if guest, errResult = managedMachineGuest(c.UserContext(), machine); errResult != nil {
		_ = db.FailJob(job.ID, "managed_identity_missing", "The managed guest identity could not be verified.", "permanent")
		return c.Status(409).JSON(fiber.Map{"error": "managed guest identity could not be verified"})
	}
	var ticket proxmox.ConsoleTicket
	if ticket, errResult = proxmoxIntegration.service.CreateConsoleTicket(c.UserContext(), guest.Node, guest.VMID); errResult != nil {
		_ = db.FailJob(job.ID, "console_ticket_failed", "The console ticket could not be created.", "transient")
		return c.Status(502).JSON(fiber.Map{"error": "console ticket could not be created"})
	}
	_ = db.MarkJobRunning(job.ID)
	_ = db.MarkJobFinished(job.ID, db.JobStatusSucceeded)
	var bytes [16]byte
	_, _ = rand.Read(bytes[:])
	var session consoleSession = consoleSession{ID: hex.EncodeToString(bytes[:]), UserID: user.ID, ResourceID: resource.ID, Ticket: ticket.Ticket, Port: ticket.Port, Node: guest.Node, VMID: guest.VMID, ExpiresAt: ticket.ExpiresAt}
	consoleSessions.Lock()
	for id, existing := range consoleSessions.items {
		if !existing.ExpiresAt.After(time.Now().UTC()) {
			delete(consoleSessions.items, id)
		}
	}
	consoleSessions.items[session.ID] = session
	consoleSessions.Unlock()
	return c.Status(201).JSON(fiber.Map{"id": session.ID, "job_id": job.ID, "expires_at": session.ExpiresAt, "websocket_path": "/api/v1/console-sessions/" + session.ID + "/websocket"})
}

func validateConsoleUpgrade(c *fiber.Ctx) (errResult error) {
	if !fiberwebsocket.IsWebSocketUpgrade(c) {
		return c.SendStatus(fiber.StatusUpgradeRequired)
	}
	var origin *url.URL
	if origin, errResult = url.Parse(c.Get("Origin")); errResult != nil || origin.Host != c.Get("Host") || origin.Scheme != c.Protocol() {
		return c.Status(403).JSON(fiber.Map{"error": "console origin is not allowed"})
	}
	consoleSessions.Lock()
	var session consoleSession
	var found bool
	session, found = consoleSessions.items[c.Params("id")]
	consoleSessions.Unlock()
	var user *db.User = currentDBUser(c)
	if !found || user == nil || !consoleSessionValid(session, user.ID, time.Now().UTC()) {
		return c.Status(403).JSON(fiber.Map{"error": "console session is invalid or expired"})
	}
	var resource *db.Resource
	if resource, found, errResult = db.GetResourceByID(session.ResourceID); errResult != nil || !found {
		return c.Status(403).JSON(fiber.Map{"error": "console target is unavailable"})
	}
	var allowed bool
	if allowed, errResult = currentUserCan(c, db.PermissionVMConsole, db.RoleBindingScopeResource, &resource.ID); errResult != nil {
		return c.SendStatus(500)
	}
	if !allowed {
		if allowed, errResult = currentUserCan(c, db.PermissionVMConsole, db.RoleBindingScopeProject, &resource.ProjectID); errResult != nil {
			return c.SendStatus(500)
		}
	}
	if !allowed {
		return c.Status(403).JSON(fiber.Map{"error": "console permission was revoked"})
	}
	var machine *db.VirtualMachine
	if machine, found, errResult = db.VirtualMachineForResource(resource.ID); errResult != nil || !found {
		return c.Status(403).JSON(fiber.Map{"error": "console target is unavailable"})
	}
	if _, errResult = managedMachineGuest(c.UserContext(), machine); errResult != nil {
		return c.Status(403).JSON(fiber.Map{"error": "managed console target could not be verified"})
	}
	c.Locals("console-session", session)
	return c.Next()
}

func consoleSessionValid(session consoleSession, userID int, now time.Time) (okResult bool) {
	return session.ID != "" && session.UserID == userID && session.ExpiresAt.After(now)
}

func proxyConsoleSession(browser *fiberwebsocket.Conn) {
	var session, ok = browser.Locals("console-session").(consoleSession)
	if !ok {
		_ = browser.Close()
		return
	}
	consoleSessions.Lock()
	delete(consoleSessions.items, session.ID)
	consoleSessions.Unlock()
	var dialer, supported = proxmoxIntegration.service.(proxmox.ConsoleDialer)
	if !supported {
		_ = browser.Close()
		return
	}
	var upstream *websocket.Conn
	var err error
	if upstream, err = dialer.DialConsole(context.Background(), session.Node, session.VMID, session.Port, session.Ticket); err != nil {
		_ = browser.Close()
		return
	}
	defer upstream.Close()
	var done chan struct{} = make(chan struct{}, 2)
	var copyMessages func(destination, source *websocket.Conn)
	copyMessages = func(destination, source *websocket.Conn) {
		for {
			var messageType int
			var data []byte
			if messageType, data, err = source.ReadMessage(); err != nil {
				break
			}
			if err = destination.WriteMessage(messageType, data); err != nil {
				break
			}
		}
		done <- struct{}{}
	}
	go copyMessages(upstream, browser.Conn)
	go copyMessages(browser.Conn, upstream)
	<-done
}

func machineNode(machine *db.VirtualMachine) string {
	if machine.NodeID == nil {
		return ""
	}
	var node *db.ProxmoxNode
	node, _ = db.ProxmoxNodes.Select(*machine.NodeID)
	if node == nil {
		return ""
	}
	return node.Name
}

func managedMachineGuest(ctx context.Context, machine *db.VirtualMachine) (guestResult proxmox.Guest, errResult error) {
	if machine == nil || proxmoxIntegration.service == nil {
		return guestResult, fmt.Errorf("Proxmox integration is unavailable")
	}
	var cluster *db.ProxmoxCluster
	if cluster, errResult = db.ProxmoxClusters.Select(machine.ClusterID); errResult != nil || cluster == nil {
		return guestResult, fmt.Errorf("Proxmox cluster identity is missing")
	}
	if cluster.UUID != proxmoxIntegration.clusterIdentity && cluster.Name != proxmoxIntegration.clusterIdentity {
		return guestResult, fmt.Errorf("Proxmox cluster identity does not match")
	}
	if guestResult, errResult = proxmoxIntegration.service.GetGuest(ctx, machineNode(machine), machine.ProxmoxVMID); errResult != nil {
		return
	}
	if !proxmox.HasTag(guestResult, proxmoxIntegration.managedTag) {
		return guestResult, fmt.Errorf("required managed tag is missing")
	}
	return
}

func consumeProxmoxAction(_ int, payload []byte) (result gasket.TaskConsumerResult) {
	var jobPayload jobscheduler.JobPayload
	var err error
	if err = json.Unmarshal(payload, &jobPayload); err != nil {
		result.Error = err
		return
	}
	var job *db.Job
	var resource *db.Resource
	var machine *db.VirtualMachine
	var user *db.User
	var found, allowed bool
	if job, found, err = db.GetJobByID(jobPayload.JobID); err != nil || !found {
		result.Error = err
		return
	}
	if resource, found, err = db.GetResourceByID(*job.ResourceID); err != nil || !found {
		_ = db.FailJob(job.ID, "resource_missing", "The resource no longer exists.", "permanent")
		result.Success = true
		return
	}
	if user, err = db.Users.Select(*job.RequestedByUserID); err != nil || user == nil {
		result.Error = err
		return
	}
	var permission db.PermissionKey
	if permission, allowed = powerPermission(jobPayload.Operation); !allowed {
		_ = db.FailJob(job.ID, "invalid_operation", "The requested operation is invalid.", "permanent")
		result.Success = true
		return
	}
	if !user.IsSystemAdmin {
		var groups []int
		if groups, err = db.CloudGroupIDsForUser(user.ID); err != nil {
			result.Error = err
			return
		}
		if allowed, err = db.HasPermission(db.PermissionCheck{UserID: user.ID, GroupIDs: groups, Permission: permission, ScopeType: db.RoleBindingScopeResource, ScopeID: &resource.ID}); err != nil {
			result.Error = err
			return
		}
		if !allowed {
			if allowed, err = db.HasPermission(db.PermissionCheck{UserID: user.ID, GroupIDs: groups, Permission: permission, ScopeType: db.RoleBindingScopeProject, ScopeID: &resource.ProjectID}); err != nil {
				result.Error = err
				return
			}
		}
	}
	if !allowed && !user.IsSystemAdmin {
		_ = db.FailJob(job.ID, "authorization_revoked", "Permission was revoked before execution.", "permanent")
		result.Success = true
		return
	}
	if machine, found, err = db.VirtualMachineForResource(resource.ID); err != nil || !found {
		_ = db.FailJob(job.ID, "identity_missing", "The Proxmox identity is missing.", "permanent")
		result.Success = true
		return
	}
	var guest proxmox.Guest
	if guest, err = managedMachineGuest(context.Background(), machine); err != nil {
		_ = db.FailJob(job.ID, "managed_tag_missing", "The required managed tag could not be verified.", "permanent")
		result.Success = true
		return
	}
	if err = db.MarkJobRunning(job.ID); err != nil {
		result.Error = err
		return
	}
	var taskID string
	switch jobPayload.Operation {
	case "start":
		taskID, err = proxmoxIntegration.service.StartGuest(context.Background(), guest.Node, guest.VMID)
	case "stop":
		taskID, err = proxmoxIntegration.service.StopGuest(context.Background(), guest.Node, guest.VMID)
	case "shutdown":
		taskID, err = proxmoxIntegration.service.ShutdownGuest(context.Background(), guest.Node, guest.VMID)
	case "reboot":
		taskID, err = proxmoxIntegration.service.RebootGuest(context.Background(), guest.Node, guest.VMID)
	}
	if err != nil {
		_ = db.FailJob(job.ID, "provider_error", "Proxmox rejected the power action.", "transient")
		result.Success = true
		return
	}
	for poll := 0; poll < proxmoxTaskPollLimit; poll++ {
		var task proxmox.Task
		if task, err = proxmoxIntegration.service.GetTask(context.Background(), guest.Node, taskID); err != nil {
			break
		}
		_ = db.UpdateJobProgress(job.ID, min(95, 10+poll), 2*time.Minute)
		if task.Status == "stopped" {
			if task.ExitStatus == "OK" || task.ExitStatus == "" {
				var observed proxmox.Guest
				if observed, err = managedMachineGuest(context.Background(), machine); err == nil {
					var state db.PowerState = db.PowerStateUnknown
					if observed.Status == "running" {
						state = db.PowerStateRunning
					} else if observed.Status == "stopped" {
						state = db.PowerStateStopped
					}
					_ = db.UpdateVirtualMachinePower(machine, state)
				}
				_ = db.MarkJobFinished(job.ID, db.JobStatusSucceeded)
			} else {
				_ = db.FailJob(job.ID, "provider_task_failed", "Proxmox could not complete the power action.", "permanent")
			}
			result.Success = true
			return
		}
		time.Sleep(proxmoxTaskPollInterval)
	}
	_ = db.FailJob(job.ID, "provider_timeout", "Timed out waiting for Proxmox task completion.", "transient")
	result.Success = true
	return
}

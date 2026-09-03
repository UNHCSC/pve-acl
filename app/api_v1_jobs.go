package app

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/UNHCSC/organesson/db"
	jobscheduler "github.com/UNHCSC/organesson/scheduler"
	"github.com/gofiber/fiber/v2"
)

func currentUserCanViewJob(c *fiber.Ctx, job *db.Job) (allowedResult bool, errResult error) {
	var user *db.User = currentDBUser(c)
	if user == nil || job == nil {
		return
	}
	if currentUserIsSiteAdmin(c) || (job.RequestedByUserID != nil && *job.RequestedByUserID == user.ID) {
		return true, nil
	}
	if job.ResourceID != nil {
		return currentUserCan(c, db.PermissionAuditRead, db.RoleBindingScopeResource, job.ResourceID)
	}
	if job.ProjectID != nil {
		return currentUserCan(c, db.PermissionAuditRead, db.RoleBindingScopeProject, job.ProjectID)
	}
	return
}

func getJobs(c *fiber.Ctx) (errResult error) {
	var jobs []*db.Job
	if jobs, errResult = db.ListJobs(); errResult != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to load jobs"})
	}
	var visible []*db.Job = make([]*db.Job, 0, len(jobs))
	for _, job := range jobs {
		var allowed bool
		if allowed, errResult = currentUserCanViewJob(c, job); errResult != nil {
			return c.Status(500).JSON(fiber.Map{"error": "permission check failed"})
		}
		if allowed {
			visible = append(visible, job)
		}
	}
	return c.JSON(visible)
}

func jobFromParam(c *fiber.Ctx) (jobResult *db.Job, errResult error) {
	var id int
	var found bool
	if id, errResult = strconv.Atoi(c.Params("id")); errResult != nil {
		return nil, fiber.ErrBadRequest
	}
	if jobResult, found, errResult = db.GetJobByID(id); errResult != nil {
		return
	}
	if !found {
		return nil, fiber.ErrNotFound
	}
	var allowed bool
	if allowed, errResult = currentUserCanViewJob(c, jobResult); errResult != nil {
		return
	}
	if !allowed {
		return nil, fiber.ErrForbidden
	}
	return
}

func getJob(c *fiber.Ctx) (errResult error) {
	var job *db.Job
	if job, errResult = jobFromParam(c); errResult != nil {
		return c.Status(fiberStatus(errResult)).JSON(fiber.Map{"error": "job was not found or is not visible"})
	}
	return c.JSON(job)
}

func getJobLogs(c *fiber.Ctx) (errResult error) {
	var job *db.Job
	if job, errResult = jobFromParam(c); errResult != nil {
		return c.Status(fiberStatus(errResult)).JSON(fiber.Map{"error": "job was not found or is not visible"})
	}
	var logs []*db.JobLog
	if logs, errResult = db.JobLogsForJob(job.ID); errResult != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to load job logs"})
	}
	return c.JSON(logs)
}

func postDemoJob(c *fiber.Ctx) (errResult error) {
	if !currentUserIsSiteAdmin(c) {
		return c.Status(403).JSON(fiber.Map{"error": "site administrator access required"})
	}
	var key string = strings.TrimSpace(c.Get("Idempotency-Key"))
	if key == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Idempotency-Key header is required"})
	}
	var user *db.User = currentDBUser(c)
	var digest [sha256.Size]byte = sha256.Sum256([]byte(strconv.Itoa(user.ID) + ":" + key))
	key = hex.EncodeToString(digest[:])
	var job *db.Job
	var found bool
	if job, found, errResult = db.FindJobByIdempotencyKey(key); errResult != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to check idempotency key"})
	}
	if found {
		return c.JSON(job)
	}
	if job, errResult = db.CreateJob(db.JobCreateInput{JobType: db.JobTypeCleanup, RequestedByUserID: &user.ID, Operation: "system.demo", IdempotencyKey: key, OperationKey: "system.demo:" + key, Input: map[string]any{"steps": 4}, MaxAttempts: 3}); errResult != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create job"})
	}
	if _, errResult = jobscheduler.EnqueueJobTask(job, jobscheduler.TaskTypeSystemDemo, jobscheduler.JobPayload{Operation: "system.demo"}, jobscheduler.NewDefaultRetryPolicy()); errResult != nil {
		_ = db.FailJob(job.ID, "enqueue_failed", "The operation could not be queued.", "transient")
		return c.Status(503).JSON(fiber.Map{"error": "failed to enqueue job"})
	}
	return c.Status(202).JSON(job)
}

func postCancelJob(c *fiber.Ctx) (errResult error) {
	var request struct {
		Confirm bool `json:"confirm"`
	}
	if errResult = c.BodyParser(&request); errResult != nil || !request.Confirm {
		return c.Status(400).JSON(fiber.Map{"error": "explicit cancellation confirmation is required"})
	}
	var job *db.Job
	if job, errResult = jobFromParam(c); errResult != nil {
		return c.Status(fiberStatus(errResult)).JSON(fiber.Map{"error": "job was not found or is not visible"})
	}
	var user *db.User = currentDBUser(c)
	if !currentUserIsSiteAdmin(c) && (job.RequestedByUserID == nil || *job.RequestedByUserID != user.ID) {
		return c.Status(403).JSON(fiber.Map{"error": "only the requester or an administrator may cancel this job"})
	}
	if job, errResult = db.RequestJobCancellation(job.ID); errResult != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to request cancellation"})
	}
	return c.JSON(job)
}

func fiberStatus(err error) (statusResult int) {
	if err == fiber.ErrBadRequest {
		return 400
	}
	if err == fiber.ErrForbidden {
		return 403
	}
	if err == fiber.ErrNotFound {
		return 404
	}
	return 500
}

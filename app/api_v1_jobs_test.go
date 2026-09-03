package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/UNHCSC/organesson/db"
	jobscheduler "github.com/UNHCSC/organesson/scheduler"
	"github.com/gofiber/fiber/v2"
)

func TestJobListDoesNotExposeAnotherUsersJob(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var owner *db.User = ensureResourceAPIUser(t, "job-owner")
	var other *db.User = ensureResourceAPIUser(t, "job-other")
	var err error
	if _, err = db.CreateJob(db.JobCreateInput{JobType: db.JobTypeCleanup, RequestedByUserID: &owner.ID, Operation: "private.operation"}); err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	var app *fiber.App = newAuthenticatedFiberApp()
	app.Get("/api/v1/jobs", getJobs)
	var response *http.Response = resourceAPIRequest(t, app, authenticateTestUser(t, other.Username, false), http.MethodGet, "/api/v1/jobs", "")
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	var body []byte
	if body, err = io.ReadAll(response.Body); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(body) != "[]" {
		t.Fatalf("expected no visible jobs, got %s", body)
	}
}

func TestDemoJobIdempotencyKeyDoesNotDuplicateWork(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var service *jobscheduler.Service
	var err error
	if service, err = jobscheduler.Init(filepath.Join(t.TempDir(), "tasks.db")); err != nil {
		t.Fatalf("scheduler init: %v", err)
	}
	defer service.Close()
	var app *fiber.App = newAuthenticatedFiberApp()
	app.Post("/api/v1/jobs/demo", postDemoJob)
	var token string = authenticateTestUser(t, "job-admin", true)
	for attempt := 0; attempt < 2; attempt++ {
		var request *http.Request = httptest.NewRequest(http.MethodPost, "/api/v1/jobs/demo", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Idempotency-Key", "same-request")
		var response *http.Response
		if response, err = testFiberRequest(app, request); err != nil || (response.StatusCode != 200 && response.StatusCode != 202) {
			t.Fatalf("attempt %d status=%v err=%v", attempt, response.StatusCode, err)
		}
	}
	var count int64
	if count, err = db.Jobs.Count(); err != nil || count != 1 {
		t.Fatalf("expected one job, count=%d err=%v", count, err)
	}
}

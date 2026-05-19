package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func TestGetSystemSummary(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		token    string     = authenticateTestUser(t, "summary-user", true)
		fiberApp *fiber.App = newAuthenticatedFiberApp()
		req      *http.Request
		resp     *http.Response
		body     map[string]any
		counts   map[string]any
		ok       bool
		err      error
	)

	fiberApp.Get("/api/v1/system/summary", getSystemSummary)

	req = httptest.NewRequest("GET", "/api/v1/system/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if counts, ok = body["counts"].(map[string]any); !ok {
		t.Fatalf("expected counts object, got %#v", body["counts"])
	}
	if counts["permissions"].(float64) < float64(len(db.CorePermissionNames)) {
		t.Fatalf("expected core permissions to be counted, got %#v", counts["permissions"])
	}
}

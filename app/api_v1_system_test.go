package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func TestGetSystemSummary(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	token := authenticateTestUser(t, "summary-user", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/system/summary", getSystemSummary)

	req := httptest.NewRequest("GET", "/api/v1/system/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	counts, ok := body["counts"].(map[string]any)
	if !ok {
		t.Fatalf("expected counts object, got %#v", body["counts"])
	}
	if counts["permissions"].(float64) < float64(len(db.CorePermissionNames)) {
		t.Fatalf("expected core permissions to be counted, got %#v", counts["permissions"])
	}
}

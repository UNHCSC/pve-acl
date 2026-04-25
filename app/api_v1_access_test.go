package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/UNHCSC/proxman/db"
	"github.com/gofiber/fiber/v2"
)

func TestGetAccessData(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	token := authenticateTestUser(t, "access-user", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/system/access", getAccessData)

	req := httptest.NewRequest("GET", "/api/v1/system/access", nil)
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
	if len(body["roles"].([]any)) == 0 {
		t.Fatal("expected at least one role")
	}
	if len(body["permissions"].([]any)) == 0 {
		t.Fatal("expected permissions")
	}
}

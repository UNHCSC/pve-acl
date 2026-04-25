package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestEnumRoutes(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v1/enums/asset-types", getAssetTypes)
	app.Get("/api/v1/enums/asset-permissions", getAssetPermissions)
	app.Get("/api/v1/enums/management-permissions", getManagementPermissions)

	tests := []string{
		"/api/v1/enums/asset-types",
		"/api/v1/enums/asset-permissions",
		"/api/v1/enums/management-permissions",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", path, nil))
			if err != nil {
				t.Fatalf("app.Test returned error: %v", err)
			}
			if resp.StatusCode != fiber.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}

			var body map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(body) == 0 {
				t.Fatal("expected non-empty enum response")
			}
		})
	}
}

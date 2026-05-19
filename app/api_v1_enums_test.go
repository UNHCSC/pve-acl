package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestEnumRoutes(t *testing.T) {
	var (
		fiberApp *fiber.App = fiber.New()
		tests    []string   = []string{
			"/api/v1/enums/asset-types",
			"/api/v1/enums/asset-permissions",
			"/api/v1/enums/management-permissions",
		}
	)

	fiberApp.Get("/api/v1/enums/asset-types", getAssetTypes)
	fiberApp.Get("/api/v1/enums/asset-permissions", getAssetPermissions)
	fiberApp.Get("/api/v1/enums/management-permissions", getManagementPermissions)

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			var (
				resp *http.Response
				body map[string]string
				err  error
			)

			if resp, err = fiberApp.Test(httptest.NewRequest("GET", path, nil)); err != nil {
				t.Fatalf("app.Test returned error: %v", err)
			}

			if resp.StatusCode != fiber.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}

			if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if len(body) == 0 {
				t.Fatal("expected non-empty enum response")
			}
		})
	}
}

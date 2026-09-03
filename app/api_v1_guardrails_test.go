package app

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func TestProjectSecretAPINeverReturnsPlaintext(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var err error

	if err = db.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))); err != nil {
		t.Fatalf("ConfigureSecretEncryption returned error: %v", err)
	}
	var project *db.Project = createResourceAPIProject(t, "Encrypted Secrets")
	var manager *db.User = ensureResourceAPIUser(t, "secret-manager")
	grantProjectRole(t, project.ID, manager.ID, db.ProjectRoleManager)
	var token string = authenticateTestUser(t, manager.Username, false)
	var fiberApp *fiber.App = newAuthenticatedFiberApp()
	fiberApp.Get("/api/v1/projects/:id/secrets", getProjectSecrets)
	fiberApp.Post("/api/v1/projects/:id/secrets", postProjectSecret)
	fiberApp.Patch("/api/v1/projects/:id/secrets/:secretID", patchProjectSecret)
	var resp *http.Response

	resp = resourceAPIRequest(t, fiberApp, token, "POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/secrets", `{"name":"admin password","secretType":"password","value":"never-return-this"}`)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected secret create 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode secret response: %v", err)
	}
	var found bool

	if _, found = created["encrypted_value"]; found {
		t.Fatal("secret API returned encrypted value")
	}
	var secretID int = int(created["id"].(float64))
	resp = resourceAPIRequest(t, fiberApp, token, "PATCH", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/secrets/"+strconv.Itoa(secretID), `{"value":"rotated-never-return"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected secret rotate 200, got %d", resp.StatusCode)
	}
	var stored *db.Secret
	stored, err = db.Secrets.Select(secretID)
	if err != nil || stored == nil || strings.Contains(string(stored.EncryptedValue), "rotated-never-return") {
		t.Fatalf("expected encrypted secret at rest, secret=%#v err=%v", stored, err)
	}
}

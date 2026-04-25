package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/UNHCSC/proxman/auth"
	"github.com/UNHCSC/proxman/db"
	"github.com/gofiber/fiber/v2"
)

func TestGetCurrentUser(t *testing.T) {
	initACLTestDB(t)

	auth.AddUserInjection("me-user", "secret", auth.AuthPermsAdministrator)
	t.Cleanup(func() {
		auth.DeleteUserInjection("me-user")
		auth.Logout("me-user")
	})

	user, err := auth.Authenticate("me-user", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}

	token, err := user.Token.SignedString(jwtSigningKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/users/me", getCurrentUser)

	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
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

	if body["username"] != "me-user" {
		t.Fatalf("expected username me-user, got %#v", body["username"])
	}
	if body["permissions"] != "administrator" {
		t.Fatalf("expected administrator permissions, got %#v", body["permissions"])
	}
	if body["authSource"] != "local" {
		t.Fatalf("expected local auth source, got %#v", body["authSource"])
	}
}

func TestGetCurrentUserAccessIncludesGroupsAndRoles(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	dbUser, _, err := db.EnsureUser("identity-user", "Identity User", "identity@example.test", "local", "identity-user")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}

	groups, err := db.ListCloudGroups()
	if err != nil {
		t.Fatalf("ListCloudGroups returned error: %v", err)
	}

	var adminGroup *db.CloudGroup
	for _, group := range groups {
		if group.Slug == db.DefaultAdminGroupSlug {
			adminGroup = group
			break
		}
	}
	if adminGroup == nil {
		t.Fatal("expected default admin group")
	}

	if _, err := db.EnsureCloudGroupMembership(dbUser.ID, adminGroup.ID, db.MembershipRoleMember); err != nil {
		t.Fatalf("EnsureCloudGroupMembership returned error: %v", err)
	}

	token := authenticateTestUser(t, "identity-user", false)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/users/me/access", getCurrentUserAccess)

	req := httptest.NewRequest("GET", "/api/v1/users/me/access", nil)
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
	if len(body["groups"].([]any)) == 0 {
		t.Fatal("expected current groups")
	}
	if len(body["roles"].([]any)) == 0 {
		t.Fatal("expected current roles")
	}
	if len(body["roleBindings"].([]any)) == 0 {
		t.Fatal("expected current role bindings")
	}
}

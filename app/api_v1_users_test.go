package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func TestGetCurrentUser(t *testing.T) {
	initACLTestDB(t)

	auth.AddUserInjection("me-user", "secret", auth.AuthPermsAdministrator)
	t.Cleanup(func() {
		auth.DeleteUserInjection("me-user")
		auth.Logout("me-user")
	})

	var (
		user     *auth.AuthUser
		token    string
		fiberApp *fiber.App
		req      *http.Request
		resp     *http.Response
		body     map[string]any
		err      error
	)

	if user, err = auth.Authenticate("me-user", "secret"); err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if token, err = user.Token.SignedString(jwtSigningKey); err != nil {
		t.Fatalf("sign token: %v", err)
	}

	fiberApp = fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/users/me", getCurrentUser)

	req = httptest.NewRequest("GET", "/api/v1/users/me", nil)
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
	ensureInitialSetupForTest(t)

	var (
		dbUser     *db.User
		groups     []*db.CloudGroup
		adminGroup *db.CloudGroup
		token      string
		fiberApp   *fiber.App
		req        *http.Request
		resp       *http.Response
		body       map[string]any
		err        error
	)

	if dbUser, _, err = db.EnsureUser("identity-user", "Identity User", "identity@example.test", "local", "identity-user"); err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if groups, err = db.ListCloudGroups(); err != nil {
		t.Fatalf("ListCloudGroups returned error: %v", err)
	}
	for _, group := range groups {
		if group.Slug == db.DefaultAdminGroupSlug {
			adminGroup = group
			break
		}
	}
	if adminGroup == nil {
		t.Fatal("expected default admin group")
	}

	if _, err = db.EnsureCloudGroupMembership(dbUser.ID, adminGroup.ID, db.MembershipRoleMember); err != nil {
		t.Fatalf("EnsureCloudGroupMembership returned error: %v", err)
	}

	token = authenticateTestUser(t, "identity-user", false)

	fiberApp = fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/users/me/access", getCurrentUserAccess)

	req = httptest.NewRequest("GET", "/api/v1/users/me/access", nil)
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

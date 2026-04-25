package app

import (
	"net/http/httptest"
	"testing"

	"github.com/UNHCSC/proxman/auth"
	"github.com/UNHCSC/proxman/config"
	"github.com/UNHCSC/proxman/db"
	"github.com/gofiber/fiber/v2"
)

func TestRequireAPIAuthRejectsMissingToken(t *testing.T) {
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/protected", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := fiberApp.Test(httptest.NewRequest("GET", "/api/v1/protected", nil))
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLDAPSyncOnlyImportsConfiguredAndOptInGroups(t *testing.T) {
	initACLTestDB(t)
	config.Config.LDAP.AdminGroups = []string{"Domain Admins"}
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	dbUser, _, err := db.EnsureUser("ldap-sync-user", "LDAP Sync User", "sync@example.test", "ldap", "ldap-sync-user")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}

	syncedGroup, _, err := db.EnsureCloudGroup("Teaching Staff", "teaching-staff", db.GroupTypeCustom)
	if err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	syncedGroup.SyncSource = db.CloudGroupSyncSourceLDAP
	syncedGroup.ExternalID = "Teaching Staff"
	syncedGroup.SyncMembership = true
	if err := db.UpdateCloudGroup(syncedGroup); err != nil {
		t.Fatalf("UpdateCloudGroup returned error: %v", err)
	}

	if err := syncLDAPCloudGroupMemberships(dbUser, []string{"Domain Admins", "Teaching Staff", "ipausers"}); err != nil {
		t.Fatalf("syncLDAPCloudGroupMemberships returned error: %v", err)
	}

	if _, found, err := db.CloudGroupMembershipForUserAndGroup(dbUser.ID, syncedGroup.ID); err != nil || !found {
		t.Fatalf("expected membership in opt-in synced group, found=%v err=%v", found, err)
	}
	domainAdmins := findTestGroupBySlug(t, "domain-admins")
	if _, found, err := db.CloudGroupMembershipForUserAndGroup(dbUser.ID, domainAdmins.ID); err != nil || !found {
		t.Fatalf("expected membership in configured admin group, found=%v err=%v", found, err)
	}

	groups, err := db.ListCloudGroups()
	if err != nil {
		t.Fatalf("ListCloudGroups returned error: %v", err)
	}
	for _, group := range groups {
		if group.Slug == "ipausers" {
			t.Fatalf("did not expect arbitrary LDAP group to be imported: %#v", group)
		}
	}
}

func TestRequireAPIAuthSetsCurrentUser(t *testing.T) {
	initACLTestDB(t)

	auth.AddUserInjection("middleware-user", "secret", auth.AuthPermsUser)
	t.Cleanup(func() {
		auth.DeleteUserInjection("middleware-user")
		auth.Logout("middleware-user")
	})

	user, err := auth.Authenticate("middleware-user", "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}

	token, err := user.Token.SignedString(jwtSigningKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/protected", func(c *fiber.Ctx) error {
		if currentUser(c) == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if currentDBUser(c) == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

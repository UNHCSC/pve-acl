package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
	"github.com/z46-dev/golog"
)

func initACLTestDB(t *testing.T) {
	t.Helper()
	var err error

	if db.Driver != nil {
		if err = db.Driver.Close(); err != nil {
			t.Fatalf("close previous database: %v", err)
		}
	}

	config.Config = config.Configuration{}
	config.Config.Database.File = filepath.Join(t.TempDir(), "acl-test.db")

	if err = db.Init(golog.New()); err != nil {
		t.Fatalf("db.Init returned error: %v", err)
	}

	t.Cleanup(func() {
		if db.Driver != nil {
			_ = db.Driver.Close()
		}
	})
}

func TestACLGroupAndUserLookupRoutes(t *testing.T) {
	initACLTestDB(t)

	var (
		err  error
		now  time.Time     = time.Now().UTC()
		user *db.LocalUser = &db.LocalUser{
			Username:  "alice",
			Name:      "Alice Example",
			Email:     "alice@example.test",
			FirstSeen: now,
			LastSeen:  now,
		}
		group *db.LocalGroup = &db.LocalGroup{
			Groupname:   "teaching-staff",
			DisplayName: "Teaching Staff",
		}
	)

	if err = db.LocalUsers.Insert(user); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if err = db.LocalGroups.Insert(group); err != nil {
		t.Fatalf("insert group: %v", err)
	}

	if err = db.LocalGroupMembershipsByUser.Insert(&db.LocalGroupMembership{
		Username:  user.Username,
		Groupname: group.Groupname,
	}); err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	var (
		token    string     = authenticateTestUser(t, "acl-admin", true)
		fiberApp *fiber.App = fiber.New()
	)

	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/acl/groupsForUser/:username", getGroupsForUser)
	fiberApp.Get("/api/v1/acl/usersForGroup/:groupname", getUsersForGroup)

	var (
		groupReq  *http.Request = httptest.NewRequest("GET", "/api/v1/acl/groupsForUser/alice", nil)
		groupResp *http.Response
	)

	groupReq.Header.Set("Authorization", "Bearer "+token)

	if groupResp, err = fiberApp.Test(groupReq); err != nil {
		t.Fatalf("groups route returned error: %v", err)
	}
	if groupResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected groups status 200, got %d", groupResp.StatusCode)
	}

	var groups []string
	if err = json.NewDecoder(groupResp.Body).Decode(&groups); err != nil {
		t.Fatalf("decode groups response: %v", err)
	}

	if !slices.Contains(groups, group.Groupname) {
		t.Fatalf("expected groups to contain %q, got %#v", group.Groupname, groups)
	}

	var (
		userReq  *http.Request = httptest.NewRequest("GET", "/api/v1/acl/usersForGroup/teaching-staff", nil)
		userResp *http.Response
	)

	userReq.Header.Set("Authorization", "Bearer "+token)

	if userResp, err = fiberApp.Test(userReq); err != nil {
		t.Fatalf("users route returned error: %v", err)
	}

	if userResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected users status 200, got %d", userResp.StatusCode)
	}

	var users []string
	if err = json.NewDecoder(userResp.Body).Decode(&users); err != nil {
		t.Fatalf("decode users response: %v", err)
	}

	if !slices.Contains(users, user.Username) {
		t.Fatalf("expected users to contain %q, got %#v", user.Username, users)
	}
}

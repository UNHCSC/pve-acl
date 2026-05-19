package db

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/z46-dev/golog"
)

func initTestDB(t *testing.T) {
	t.Helper()

	if Driver != nil {
		{
			var err error

			if err = Driver.Close(); err != nil {
				t.Fatalf("close previous database: %v", err)
			}
		}
	}

	config.Config = config.Configuration{}
	config.Config.Database.File = filepath.Join(t.TempDir(), "test.db")
	{
		var err error

		if err = Init(golog.New()); err != nil {
			t.Fatalf("Init returned error: %v", err)
		}
	}

	t.Cleanup(func() {
		if Driver != nil {
			_ = Driver.Close()
		}
	})
}

func TestGroupLookupHelpers(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var user *LocalUser

	user = &LocalUser{
		Username:  "alice",
		Name:      "Alice Example",
		Email:     "alice@example.test",
		FirstSeen: now,
		LastSeen:  now,
	}
	var group *LocalGroup

	group = &LocalGroup{
		Groupname:   "it666-fall2026-group-03",
		DisplayName: "Group 03",
	}
	var membership *LocalGroupMembership

	membership = &LocalGroupMembership{
		Username:  user.Username,
		Groupname: group.Groupname,
	}
	{
		var err error

		if err = LocalUsers.Insert(user); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	{
		var err error

		if err = LocalGroups.Insert(group); err != nil {
			t.Fatalf("insert group: %v", err)
		}
	}
	{
		var err error

		if err = LocalGroupMembershipsByUser.Insert(membership); err != nil {
			t.Fatalf("insert membership: %v", err)
		}
	}
	var (
		groups []string
		err    error
	)

	groups, err = GroupsForUser(user.Username)
	if err != nil {
		t.Fatalf("GroupsForUser returned error: %v", err)
	}
	if !slices.Contains(groups, group.Groupname) {
		t.Fatalf("expected groups to contain %q, got %#v", group.Groupname, groups)
	}
	var users []string

	users, err = UsersForGroup(group.Groupname)
	if err != nil {
		t.Fatalf("UsersForGroup returned error: %v", err)
	}
	if !slices.Contains(users, user.Username) {
		t.Fatalf("expected users to contain %q, got %#v", user.Username, users)
	}
}

func TestAssetLookupHelpers(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var user *LocalUser

	user = &LocalUser{
		Username:  "bob",
		Name:      "Bob Example",
		Email:     "bob@example.test",
		FirstSeen: now,
		LastSeen:  now,
	}
	var group *LocalGroup

	group = &LocalGroup{
		Groupname:   "club-officers",
		DisplayName: "Club Officers",
	}
	var asset *ProxmoxAsset

	asset = &ProxmoxAsset{
		ID:   "vm-1201",
		Name: "student-vm-1201",
		Type: ProxmoxAssetTypeVM,
	}
	{
		var err error

		if err = LocalUsers.Insert(user); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	{
		var err error

		if err = LocalGroups.Insert(group); err != nil {
			t.Fatalf("insert group: %v", err)
		}
	}
	{
		var err error

		if err = ProxmoxAssets.Insert(asset); err != nil {
			t.Fatalf("insert asset: %v", err)
		}
	}
	{
		var err error

		if err = ProxmoxAssetAssignmentsByUser.Insert(&ProxmoxAssetAssignmentByUser{
			AssetID:     asset.ID,
			Username:    user.Username,
			Permissions: AssetPermissionsView | AssetPermissionsPowerControl,
		}); err != nil {
			t.Fatalf("insert user assignment: %v", err)
		}
	}
	{
		var err error

		if err = ProxmoxAssetAssignmentsByGroup.Insert(&ProxmoxAssetAssignmentByGroup{
			AssetID:     asset.ID,
			Groupname:   group.Groupname,
			Permissions: AssetPermissionsView,
		}); err != nil {
			t.Fatalf("insert group assignment: %v", err)
		}
	}
	var (
		userAssets []string
		err        error
	)

	userAssets, err = AssetIDsForUser(user.Username)
	if err != nil {
		t.Fatalf("AssetIDsForUser returned error: %v", err)
	}
	if !slices.Contains(userAssets, asset.ID) {
		t.Fatalf("expected user assets to contain %q, got %#v", asset.ID, userAssets)
	}
	var groupAssets []string

	groupAssets, err = AssetIDsForGroup(group.Groupname)
	if err != nil {
		t.Fatalf("AssetIDsForGroup returned error: %v", err)
	}
	if !slices.Contains(groupAssets, asset.ID) {
		t.Fatalf("expected group assets to contain %q, got %#v", asset.ID, groupAssets)
	}
}

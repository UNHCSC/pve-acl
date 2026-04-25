package db

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/UNHCSC/proxman/config"
	"github.com/z46-dev/golog"
	"github.com/z46-dev/gomysql"
)

func initTestDB(t *testing.T) {
	t.Helper()

	if gomysql.DB != nil {
		if err := gomysql.Close(); err != nil {
			t.Fatalf("close previous database: %v", err)
		}
	}

	config.Config = config.Configuration{}
	config.Config.Database.File = filepath.Join(t.TempDir(), "test.db")

	if err := Init(golog.New()); err != nil {
		t.Fatalf("db.Init returned error: %v", err)
	}

	t.Cleanup(func() {
		if gomysql.DB != nil {
			_ = gomysql.Close()
		}
	})
}

func TestGroupLookupHelpers(t *testing.T) {
	initTestDB(t)

	now := time.Now().UTC()
	user := &LocalUser{
		Username:  "alice",
		Name:      "Alice Example",
		Email:     "alice@example.test",
		FirstSeen: now,
		LastSeen:  now,
	}
	group := &LocalGroup{
		Groupname:   "it666-fall2026-group-03",
		DisplayName: "Group 03",
	}
	membership := &LocalGroupMembership{
		Username:  user.Username,
		Groupname: group.Groupname,
	}

	if err := LocalUsers.Insert(user); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := LocalGroups.Insert(group); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	if err := LocalGroupMembershipsByUser.Insert(membership); err != nil {
		t.Fatalf("insert membership: %v", err)
	}

	groups, err := GroupsForUser(user.Username)
	if err != nil {
		t.Fatalf("GroupsForUser returned error: %v", err)
	}
	if !slices.Contains(groups, group.Groupname) {
		t.Fatalf("expected groups to contain %q, got %#v", group.Groupname, groups)
	}

	users, err := UsersForGroup(group.Groupname)
	if err != nil {
		t.Fatalf("UsersForGroup returned error: %v", err)
	}
	if !slices.Contains(users, user.Username) {
		t.Fatalf("expected users to contain %q, got %#v", user.Username, users)
	}
}

func TestAssetLookupHelpers(t *testing.T) {
	initTestDB(t)

	now := time.Now().UTC()
	user := &LocalUser{
		Username:  "bob",
		Name:      "Bob Example",
		Email:     "bob@example.test",
		FirstSeen: now,
		LastSeen:  now,
	}
	group := &LocalGroup{
		Groupname:   "club-officers",
		DisplayName: "Club Officers",
	}
	asset := &ProxmoxAsset{
		ID:   "vm-1201",
		Name: "student-vm-1201",
		Type: ProxmoxAssetTypeVM,
	}

	if err := LocalUsers.Insert(user); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := LocalGroups.Insert(group); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	if err := ProxmoxAssets.Insert(asset); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if err := ProxmoxAssetAssignmentsByUser.Insert(&ProxmoxAssetAssignmentByUser{
		AssetID:     asset.ID,
		Username:    user.Username,
		Permissions: AssetPermissionsView | AssetPermissionsPowerControl,
	}); err != nil {
		t.Fatalf("insert user assignment: %v", err)
	}
	if err := ProxmoxAssetAssignmentsByGroup.Insert(&ProxmoxAssetAssignmentByGroup{
		AssetID:     asset.ID,
		Groupname:   group.Groupname,
		Permissions: AssetPermissionsView,
	}); err != nil {
		t.Fatalf("insert group assignment: %v", err)
	}

	userAssets, err := AssetIDsForUser(user.Username)
	if err != nil {
		t.Fatalf("AssetIDsForUser returned error: %v", err)
	}
	if !slices.Contains(userAssets, asset.ID) {
		t.Fatalf("expected user assets to contain %q, got %#v", asset.ID, userAssets)
	}

	groupAssets, err := AssetIDsForGroup(group.Groupname)
	if err != nil {
		t.Fatalf("AssetIDsForGroup returned error: %v", err)
	}
	if !slices.Contains(groupAssets, asset.ID) {
		t.Fatalf("expected group assets to contain %q, got %#v", asset.ID, groupAssets)
	}
}

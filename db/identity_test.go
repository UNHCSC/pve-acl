package db

import (
	"slices"
	"testing"
	"time"
)

func TestEnsureUserIsIdempotent(t *testing.T) {
	initTestDB(t)
	var (
		user    *User
		created bool
		err     error
	)

	user, created, err = EnsureUser("alice", "Alice Example", "alice@example.test", "local", "alice")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if !created {
		t.Fatal("expected first EnsureUser call to create user")
	}
	if user.ID == 0 {
		t.Fatal("expected user ID to be set")
	}
	var existing *User

	existing, created, err = EnsureUser("alice", "Ignored", "ignored@example.test", "local", "alice")
	if err != nil {
		t.Fatalf("second EnsureUser returned error: %v", err)
	}
	if created {
		t.Fatal("expected second EnsureUser call to reuse existing user")
	}
	if existing.ID != user.ID {
		t.Fatalf("expected existing user ID %d, got %d", user.ID, existing.ID)
	}
}

func TestCloudGroupIDsForUser(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var (
		user *User
		err  error
	)

	user, _, err = EnsureUser("alice", "Alice Example", "alice@example.test", "local", "alice")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	var group *CloudGroup

	group = insertTestCloudGroup(t, "Teaching Staff", "teaching-staff", now)
	{
		var err error

		if err = CloudGroupMemberships.Insert(&CloudGroupMembership{
			UserID:         user.ID,
			GroupID:        group.ID,
			MembershipRole: MembershipRoleMember,
			CreatedAt:      now,
		}); err != nil {
			t.Fatalf("insert cloud group membership: %v", err)
		}
	}
	var groupIDs []int

	groupIDs, err = CloudGroupIDsForUser(user.ID)
	if err != nil {
		t.Fatalf("CloudGroupIDsForUser returned error: %v", err)
	}
	if !slices.Contains(groupIDs, group.ID) {
		t.Fatalf("expected group IDs to contain %d, got %#v", group.ID, groupIDs)
	}
	{
		var err error

		if err = ArchiveCloudGroup(group); err != nil {
			t.Fatalf("ArchiveCloudGroup returned error: %v", err)
		}
	}
	groupIDs, err = CloudGroupIDsForUser(user.ID)
	if err != nil {
		t.Fatalf("CloudGroupIDsForUser after archive returned error: %v", err)
	}
	if slices.Contains(groupIDs, group.ID) {
		t.Fatalf("expected archived group ID to be omitted, got %#v", groupIDs)
	}
}

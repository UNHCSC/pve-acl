package db

import (
	"testing"

	"github.com/UNHCSC/proxman/config"
)

func TestEnsureInitialSetupSeedsCoreRows(t *testing.T) {
	initTestDB(t)
	config.Config.LDAP.AdminGroups = []string{"admins", "Domain Admins"}

	if err := EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	if _, found, err := findOrganizationBySlug(DefaultRootOrganizationSlug); err != nil {
		t.Fatalf("find organization: %v", err)
	} else if !found {
		t.Fatalf("expected root organization %q", DefaultRootOrganizationSlug)
	}

	if _, found, err := findCloudGroupBySlug(DefaultAdminGroupSlug); err != nil {
		t.Fatalf("find admin group: %v", err)
	} else if !found {
		t.Fatalf("expected admin group %q", DefaultAdminGroupSlug)
	}

	configuredAdminGroup, found, err := findCloudGroupBySlug("domain-admins")
	if err != nil {
		t.Fatalf("find configured admin group: %v", err)
	} else if !found {
		t.Fatal("expected configured LDAP admin group to be mirrored as a cloud group")
	}
	if configuredAdminGroup.SyncSource != CloudGroupSyncSourceLDAP || !configuredAdminGroup.SyncMembership || configuredAdminGroup.ExternalID != "Domain Admins" {
		t.Fatalf("expected configured admin group to be LDAP synced, got %#v", configuredAdminGroup)
	}

	role, found, err := findRoleByName(DefaultLabAdminRoleName)
	if err != nil {
		t.Fatalf("find role: %v", err)
	}
	if !found {
		t.Fatalf("expected role %q", DefaultLabAdminRoleName)
	}

	permissionCount, err := Permissions.Count()
	if err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if permissionCount != int64(len(CorePermissionNames)) {
		t.Fatalf("expected %d permissions, got %d", len(CorePermissionNames), permissionCount)
	}

	rolePermissionCount, err := RolePermissions.Count()
	if err != nil {
		t.Fatalf("count role permissions: %v", err)
	}
	if rolePermissionCount != int64(len(CorePermissionNames)) {
		t.Fatalf("expected %d role permissions, got %d", len(CorePermissionNames), rolePermissionCount)
	}

	roleBindingCount, err := RoleBindings.Count()
	if err != nil {
		t.Fatalf("count role bindings: %v", err)
	}
	if roleBindingCount != 2 {
		t.Fatalf("expected two global admin group role bindings, got %d", roleBindingCount)
	}

	if role.ID == 0 {
		t.Fatal("expected LabAdmin role ID to be set")
	}
}

func TestEnsureInitialSetupIsIdempotent(t *testing.T) {
	initTestDB(t)
	config.Config.LDAP.AdminGroups = []string{"admins"}

	if err := EnsureInitialSetup(); err != nil {
		t.Fatalf("first EnsureInitialSetup returned error: %v", err)
	}

	counts := map[string]int64{}
	for name, countFn := range map[string]func() (int64, error){
		"organizations":    Organizations.Count,
		"groups":           CloudGroups.Count,
		"roles":            Roles.Count,
		"permissions":      Permissions.Count,
		"role_permissions": RolePermissions.Count,
		"role_bindings":    RoleBindings.Count,
		"audit_events":     AuditEvents.Count,
	} {
		count, err := countFn()
		if err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		counts[name] = count
	}

	if err := EnsureInitialSetup(); err != nil {
		t.Fatalf("second EnsureInitialSetup returned error: %v", err)
	}

	for name, countFn := range map[string]func() (int64, error){
		"organizations":    Organizations.Count,
		"groups":           CloudGroups.Count,
		"roles":            Roles.Count,
		"permissions":      Permissions.Count,
		"role_permissions": RolePermissions.Count,
		"role_bindings":    RoleBindings.Count,
		"audit_events":     AuditEvents.Count,
	} {
		count, err := countFn()
		if err != nil {
			t.Fatalf("count %s after second setup: %v", name, err)
		}
		if count != counts[name] {
			t.Fatalf("expected %s count to remain %d, got %d", name, counts[name], count)
		}
	}
}

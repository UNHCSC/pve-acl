package db

import (
	"testing"
	"time"
)

func TestHasPermissionAllowsDirectUserRoleBinding(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	permission, role := insertTestRoleWithPermission(t, "VMOperator", "vm.start", now)
	projectScopeID := 42

	if created, err := ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeProject, &projectScopeID, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	} else if !created {
		t.Fatal("expected role binding to be created")
	}

	allowed, err := HasPermission(PermissionCheck{
		UserID:         1001,
		PermissionName: permission.Name,
		ScopeType:      RoleBindingScopeProject,
		ScopeID:        &projectScopeID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected direct user role binding to allow permission")
	}
}

func TestHasPermissionAllowsGroupRoleBinding(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	permission, role := insertTestRoleWithPermission(t, "NetworkManager", "network.update", now)
	group := insertTestCloudGroup(t, "Teaching Staff", "teaching-staff", now)
	groupScopeID := 77

	if created, err := ensureRoleBinding(role.ID, RoleBindingSubjectGroup, group.ID, RoleBindingScopeGroup, &groupScopeID, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	} else if !created {
		t.Fatal("expected role binding to be created")
	}

	allowed, err := HasPermission(PermissionCheck{
		UserID:         1001,
		GroupIDs:       []int{group.ID},
		PermissionName: permission.Name,
		ScopeType:      RoleBindingScopeGroup,
		ScopeID:        &groupScopeID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected group role binding to allow permission")
	}
}

func TestHasPermissionAllowsGlobalAdminAcrossScopes(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	configuredGroup := insertTestCloudGroup(t, "Admins", "admins", now)
	permission, role := insertTestRoleWithPermission(t, DefaultLabAdminRoleName, "vm.delete", now)

	if created, err := ensureRoleBinding(role.ID, RoleBindingSubjectGroup, configuredGroup.ID, RoleBindingScopeGlobal, nil, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	} else if !created {
		t.Fatal("expected role binding to be created")
	}

	resourceScopeID := 1201
	allowed, err := HasPermission(PermissionCheck{
		UserID:         1001,
		GroupIDs:       []int{configuredGroup.ID},
		PermissionName: permission.Name,
		ScopeType:      RoleBindingScopeResource,
		ScopeID:        &resourceScopeID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected global admin binding to allow resource-scoped permission")
	}
}

func TestHasPermissionAllowsOrgBindingOnDescendantProject(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	permission, role := insertTestRoleWithPermission(t, "OrgProjectManager", "project.manage", now)
	root := insertTestOrganization(t, "Lab", "lab", nil, now)
	child := insertTestOrganization(t, "Teaching", "teaching", &root.ID, now)
	project := insertTestProject(t, "Training Lab", "training-lab", child.ID, now)

	if _, err := ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeOrg, &root.ID, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	}

	allowed, err := HasPermission(PermissionCheck{
		UserID:         1001,
		PermissionName: permission.Name,
		ScopeType:      RoleBindingScopeProject,
		ScopeID:        &project.ID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected parent org binding to allow descendant project permission")
	}
}

func TestHasPermissionDeniesWrongScopeOrMissingPermission(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	permission, role := insertTestRoleWithPermission(t, "ProjectViewer", "vm.read", now)
	allowedScopeID := 1
	deniedScopeID := 2

	if _, err := ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeProject, &allowedScopeID, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	}

	allowed, err := HasPermission(PermissionCheck{
		UserID:         1001,
		PermissionName: permission.Name,
		ScopeType:      RoleBindingScopeProject,
		ScopeID:        &deniedScopeID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if allowed {
		t.Fatal("expected wrong scope to be denied")
	}

	allowed, err = HasPermission(PermissionCheck{
		UserID:         1001,
		PermissionName: "vm.delete",
		ScopeType:      RoleBindingScopeProject,
		ScopeID:        &allowedScopeID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error for missing permission: %v", err)
	}
	if allowed {
		t.Fatal("expected missing permission to be denied")
	}
}

func insertTestRoleWithPermission(t *testing.T, roleName, permissionName string, now time.Time) (*Permission, *Role) {
	t.Helper()

	role, _, err := ensureRole(roleName, roleName+" test role", false, now)
	if err != nil {
		t.Fatalf("ensure role: %v", err)
	}

	permission, _, err := ensurePermission(permissionName)
	if err != nil {
		t.Fatalf("ensure permission: %v", err)
	}

	if _, err := ensureRolePermission(role.ID, permission.ID); err != nil {
		t.Fatalf("ensure role permission: %v", err)
	}

	return permission, role
}

func insertTestCloudGroup(t *testing.T, name, slug string, now time.Time) *CloudGroup {
	t.Helper()

	group, _, err := ensureCloudGroup(name, slug, GroupTypeCustom, now)
	if err != nil {
		t.Fatalf("ensure cloud group: %v", err)
	}

	return group
}

func insertTestOrganization(t *testing.T, name, slug string, parentID *int, now time.Time) *Organization {
	t.Helper()

	org := &Organization{
		UUID:        slug + "-uuid",
		Name:        name,
		Slug:        slug,
		ParentOrgID: parentID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := Organizations.Insert(org); err != nil {
		t.Fatalf("insert organization: %v", err)
	}
	return org
}

func insertTestProject(t *testing.T, name, slug string, orgID int, now time.Time) *Project {
	t.Helper()

	project := &Project{
		UUID:           slug + "-uuid",
		OrganizationID: orgID,
		Name:           name,
		Slug:           slug,
		ProjectType:    ProjectTypeLab,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := Projects.Insert(project); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	return project
}

package db

import (
	"testing"
	"time"
)

func TestHasPermissionAllowsDirectUserRoleBinding(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	role := insertTestRoleWithPermission(t, "VMOperator", PermissionVMStart, now)
	projectScopeID := 42

	if created, err := ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeProject, &projectScopeID, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	} else if !created {
		t.Fatal("expected role binding to be created")
	}

	allowed, err := HasPermission(PermissionCheck{
		UserID:     1001,
		Permission: PermissionVMStart,
		ScopeType:  RoleBindingScopeProject,
		ScopeID:    &projectScopeID,
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

	role := insertTestRoleWithPermission(t, "NetworkManager", PermissionNetworkUpdate, now)
	group := insertTestCloudGroup(t, "Teaching Staff", "teaching-staff", now)
	groupScopeID := 77

	if created, err := ensureRoleBinding(role.ID, RoleBindingSubjectGroup, group.ID, RoleBindingScopeGroup, &groupScopeID, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	} else if !created {
		t.Fatal("expected role binding to be created")
	}

	allowed, err := HasPermission(PermissionCheck{
		UserID:     1001,
		GroupIDs:   []int{group.ID},
		Permission: PermissionNetworkUpdate,
		ScopeType:  RoleBindingScopeGroup,
		ScopeID:    &groupScopeID,
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
	role := insertTestRoleWithPermission(t, DefaultLabAdminRoleName, PermissionVMDelete, now)

	if created, err := ensureRoleBinding(role.ID, RoleBindingSubjectGroup, configuredGroup.ID, RoleBindingScopeGlobal, nil, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	} else if !created {
		t.Fatal("expected role binding to be created")
	}

	resourceScopeID := 1201
	allowed, err := HasPermission(PermissionCheck{
		UserID:     1001,
		GroupIDs:   []int{configuredGroup.ID},
		Permission: PermissionVMDelete,
		ScopeType:  RoleBindingScopeResource,
		ScopeID:    &resourceScopeID,
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

	role := insertTestRoleWithPermission(t, "OrgProjectManager", PermissionProjectManage, now)
	root := insertTestOrganization(t, "Lab", "lab", nil, now)
	child := insertTestOrganization(t, "Teaching", "teaching", &root.ID, now)
	project := insertTestProject(t, "Training Lab", "training-lab", child.ID, now)

	if _, err := ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeOrg, &root.ID, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	}

	allowed, err := HasPermission(PermissionCheck{
		UserID:     1001,
		Permission: PermissionProjectManage,
		ScopeType:  RoleBindingScopeProject,
		ScopeID:    &project.ID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected parent org binding to allow descendant project permission")
	}
}

func TestOrganizationMembershipInheritsIntoDescendantProject(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	root := insertTestOrganization(t, "Lab", "lab", nil, now)
	child := insertTestOrganization(t, "Courses", "courses", &root.ID, now)
	project := insertTestProject(t, "IT666", "it666", child.ID, now)

	if _, err := EnsureOrganizationMembership(root.ID, ProjectMemberSubjectUser, 1001, MembershipRoleMember); err != nil {
		t.Fatalf("EnsureOrganizationMembership returned error: %v", err)
	}
	member, err := SubjectInProjectOrAncestor(project.ID, ProjectMemberSubjectUser, 1001)
	if err != nil {
		t.Fatalf("SubjectInProjectOrAncestor returned error: %v", err)
	}
	if !member {
		t.Fatal("expected Lab membership to inherit into descendant project")
	}
}

func TestResourceScopedAccessCombinesPrivateAndHigherPowerGrants(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	root := insertTestOrganization(t, "Lab", "lab", nil, now)
	project := insertTestProject(t, "IT666", "it666", root.ID, now)
	resource := insertTestResource(t, project.ID, "student-vm-1", now)
	resourceRole := insertTestRoleWithPermission(t, DefaultResourceUserRoleName, PermissionVMConsole, now)
	projectRole := insertTestRoleWithPermission(t, "ProjectConsoleAdmin", PermissionVMConsole, now)

	if _, err := EnsureResourceOwner(resource.ID, OwnerSubjectUser, 1001); err != nil {
		t.Fatalf("EnsureResourceOwner returned error: %v", err)
	}
	if err := EnsureResourceUserAccess(resource.ID, RoleBindingSubjectUser, 1001); err != nil {
		t.Fatalf("EnsureResourceUserAccess returned error: %v", err)
	}
	if _, err := ensureRoleBinding(projectRole.ID, RoleBindingSubjectUser, 2002, RoleBindingScopeProject, &project.ID, now); err != nil {
		t.Fatalf("ensure project role binding: %v", err)
	}

	studentAllowed, err := HasPermission(PermissionCheck{
		UserID:     1001,
		Permission: PermissionVMConsole,
		ScopeType:  RoleBindingScopeResource,
		ScopeID:    &resource.ID,
	})
	if err != nil {
		t.Fatalf("HasPermission student returned error: %v", err)
	}
	if !studentAllowed {
		t.Fatal("expected direct resource user grant to allow console")
	}

	projectAdminAllowed, err := HasPermission(PermissionCheck{
		UserID:     2002,
		Permission: PermissionVMConsole,
		ScopeType:  RoleBindingScopeResource,
		ScopeID:    &resource.ID,
	})
	if err != nil {
		t.Fatalf("HasPermission project admin returned error: %v", err)
	}
	if !projectAdminAllowed {
		t.Fatal("expected project-scoped grant to flow down to resource")
	}

	outsiderAllowed, err := HasPermission(PermissionCheck{
		UserID:     3003,
		Permission: PermissionVMConsole,
		ScopeType:  RoleBindingScopeResource,
		ScopeID:    &resource.ID,
	})
	if err != nil {
		t.Fatalf("HasPermission outsider returned error: %v", err)
	}
	if outsiderAllowed {
		t.Fatal("expected unrelated user to be denied")
	}

	if resourceRole.ID == 0 {
		t.Fatal("expected resource role to be set")
	}
}

func TestHasPermissionDeniesWrongScopeOrMissingPermission(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	role := insertTestRoleWithPermission(t, "ProjectViewer", PermissionVMRead, now)
	allowedScopeID := 1
	deniedScopeID := 2

	if _, err := ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeProject, &allowedScopeID, now); err != nil {
		t.Fatalf("ensure role binding: %v", err)
	}

	allowed, err := HasPermission(PermissionCheck{
		UserID:     1001,
		Permission: PermissionVMRead,
		ScopeType:  RoleBindingScopeProject,
		ScopeID:    &deniedScopeID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if allowed {
		t.Fatal("expected wrong scope to be denied")
	}

	allowed, err = HasPermission(PermissionCheck{
		UserID:     1001,
		Permission: PermissionVMDelete,
		ScopeType:  RoleBindingScopeProject,
		ScopeID:    &allowedScopeID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error for missing permission: %v", err)
	}
	if allowed {
		t.Fatal("expected missing permission to be denied")
	}
}

func insertTestResource(t *testing.T, projectID int, slug string, now time.Time) *Resource {
	t.Helper()

	resource := &Resource{
		UUID:         slug + "-uuid",
		ProjectID:    projectID,
		OwnerType:    OwnerTypeProject,
		OwnerID:      projectID,
		ResourceType: ResourceTypeVM,
		Name:         slug,
		Slug:         slug,
		Status:       ResourceStatusReady,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := Resources.Insert(resource); err != nil {
		t.Fatalf("insert resource: %v", err)
	}
	return resource
}

func insertTestRoleWithPermission(t *testing.T, roleName string, permissionKey PermissionKey, now time.Time) *Role {
	t.Helper()

	role, _, err := ensureRole(roleName, roleName+" test role", false, now)
	if err != nil {
		t.Fatalf("ensure role: %v", err)
	}

	permission, _, err := ensurePermission(permissionKey.String())
	if err != nil {
		t.Fatalf("ensure permission: %v", err)
	}

	if _, err := ensureRolePermission(role.ID, permission.ID); err != nil {
		t.Fatalf("ensure role permission: %v", err)
	}

	return role
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

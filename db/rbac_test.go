package db

import (
	"testing"
	"time"
)

func TestHasPermissionAllowsDirectUserRoleBinding(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var role *Role

	role = insertTestRoleWithPermission(t, "VMOperator", PermissionVMStart, now)
	var projectScopeID int

	projectScopeID = 42
	{
		var (
			created bool
			err     error
		)

		if created, err = ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeProject, &projectScopeID, now); err != nil {
			t.Fatalf("ensure role binding: %v", err)
		} else if !created {
			t.Fatal("expected role binding to be created")
		}
	}
	var (
		allowed bool
		err     error
	)

	allowed, err = HasPermission(PermissionCheck{
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
	var now time.Time

	now = time.Now().UTC()
	var role *Role

	role = insertTestRoleWithPermission(t, "NetworkManager", PermissionNetworkUpdate, now)
	var group *CloudGroup

	group = insertTestCloudGroup(t, "Teaching Staff", "teaching-staff", now)
	var groupScopeID int

	groupScopeID = 77
	{
		var (
			created bool
			err     error
		)

		if created, err = ensureRoleBinding(role.ID, RoleBindingSubjectGroup, group.ID, RoleBindingScopeGroup, &groupScopeID, now); err != nil {
			t.Fatalf("ensure role binding: %v", err)
		} else if !created {
			t.Fatal("expected role binding to be created")
		}
	}
	var (
		allowed bool
		err     error
	)

	allowed, err = HasPermission(PermissionCheck{
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
	var now time.Time

	now = time.Now().UTC()
	var configuredGroup *CloudGroup

	configuredGroup = insertTestCloudGroup(t, "Admins", "admins", now)
	var role *Role

	role = insertTestRoleWithPermission(t, DefaultLabAdminRoleName, PermissionVMDelete, now)
	{
		var (
			created bool
			err     error
		)

		if created, err = ensureRoleBinding(role.ID, RoleBindingSubjectGroup, configuredGroup.ID, RoleBindingScopeGlobal, nil, now); err != nil {
			t.Fatalf("ensure role binding: %v", err)
		} else if !created {
			t.Fatal("expected role binding to be created")
		}
	}
	var resourceScopeID int

	resourceScopeID = 1201
	var (
		allowed bool
		err     error
	)

	allowed, err = HasPermission(PermissionCheck{
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
	var now time.Time

	now = time.Now().UTC()
	var role *Role

	role = insertTestRoleWithPermission(t, "OrgProjectManager", PermissionProjectManage, now)
	var root *Organization

	root = insertTestOrganization(t, "Lab", "lab", nil, now)
	var child *Organization

	child = insertTestOrganization(t, "Teaching", "teaching", &root.ID, now)
	var project *Project

	project = insertTestProject(t, "Training Lab", "training-lab", child.ID, now)
	{
		var err error

		if _, err = ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeOrg, &root.ID, now); err != nil {
			t.Fatalf("ensure role binding: %v", err)
		}
	}
	var (
		allowed bool
		err     error
	)

	allowed, err = HasPermission(PermissionCheck{
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
	var now time.Time

	now = time.Now().UTC()
	var root *Organization

	root = insertTestOrganization(t, "Lab", "lab", nil, now)
	var child *Organization

	child = insertTestOrganization(t, "Courses", "courses", &root.ID, now)
	var project *Project

	project = insertTestProject(t, "IT666", "it666", child.ID, now)
	{
		var err error

		if _, err = EnsureOrganizationMembership(root.ID, ProjectMemberSubjectUser, 1001, MembershipRoleMember); err != nil {
			t.Fatalf("EnsureOrganizationMembership returned error: %v", err)
		}
	}
	var (
		member bool
		err    error
	)

	member, err = SubjectInProjectOrAncestor(project.ID, ProjectMemberSubjectUser, 1001)
	if err != nil {
		t.Fatalf("SubjectInProjectOrAncestor returned error: %v", err)
	}
	if !member {
		t.Fatal("expected Lab membership to inherit into descendant project")
	}
}

func TestResourceScopedAccessCombinesPrivateAndHigherPowerGrants(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var root *Organization

	root = insertTestOrganization(t, "Lab", "lab", nil, now)
	var project *Project

	project = insertTestProject(t, "IT666", "it666", root.ID, now)
	var resource *Resource

	resource = insertTestResource(t, project.ID, "student-vm-1", now)
	var resourceRole *Role

	resourceRole = insertTestRoleWithPermission(t, DefaultResourceUserRoleName, PermissionVMConsole, now)
	var projectRole *Role

	projectRole = insertTestRoleWithPermission(t, "ProjectConsoleAdmin", PermissionVMConsole, now)
	{
		var err error

		if _, err = EnsureResourceOwner(resource.ID, OwnerSubjectUser, 1001); err != nil {
			t.Fatalf("EnsureResourceOwner returned error: %v", err)
		}
	}
	{
		var err error

		if err = EnsureResourceUserAccess(resource.ID, RoleBindingSubjectUser, 1001); err != nil {
			t.Fatalf("EnsureResourceUserAccess returned error: %v", err)
		}
	}
	{
		var err error

		if _, err = ensureRoleBinding(projectRole.ID, RoleBindingSubjectUser, 2002, RoleBindingScopeProject, &project.ID, now); err != nil {
			t.Fatalf("ensure project role binding: %v", err)
		}
	}
	var (
		studentAllowed bool
		err            error
	)

	studentAllowed, err = HasPermission(PermissionCheck{
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
	var projectAdminAllowed bool

	projectAdminAllowed, err = HasPermission(PermissionCheck{
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
	var outsiderAllowed bool

	outsiderAllowed, err = HasPermission(PermissionCheck{
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
	var now time.Time

	now = time.Now().UTC()
	var role *Role

	role = insertTestRoleWithPermission(t, "ProjectViewer", PermissionVMRead, now)
	var allowedScopeID int

	allowedScopeID = 1
	var deniedScopeID int

	deniedScopeID = 2
	{
		var err error

		if _, err = ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeProject, &allowedScopeID, now); err != nil {
			t.Fatalf("ensure role binding: %v", err)
		}
	}
	var (
		allowed bool
		err     error
	)

	allowed, err = HasPermission(PermissionCheck{
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

func insertTestResource(t *testing.T, projectID int, slug string, now time.Time) (resourceResult *Resource) {
	t.Helper()
	var resource *Resource

	resource = &Resource{
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
	{
		var err error

		if err = Resources.Insert(resource); err != nil {
			t.Fatalf("insert resource: %v", err)
		}
	}
	return resource
}

func insertTestRoleWithPermission(t *testing.T, roleName string, permissionKey PermissionKey, now time.Time) (roleResult *Role) {
	t.Helper()
	var (
		role *Role
		err  error
	)

	role, _, err = ensureRole(roleName, roleName+" test role", false, now)
	if err != nil {
		t.Fatalf("ensure role: %v", err)
	}
	var permission *Permission

	permission, _, err = ensurePermission(permissionKey.String())
	if err != nil {
		t.Fatalf("ensure permission: %v", err)
	}
	{
		var err error

		if _, err = ensureRolePermission(role.ID, permission.ID); err != nil {
			t.Fatalf("ensure role permission: %v", err)
		}
	}

	return role
}

func insertTestCloudGroup(t *testing.T, name, slug string, now time.Time) (cloudGroupResult *CloudGroup) {
	t.Helper()
	var (
		group *CloudGroup
		err   error
	)

	group, _, err = ensureCloudGroup(name, slug, GroupTypeCustom, now)
	if err != nil {
		t.Fatalf("ensure cloud group: %v", err)
	}

	return group
}

func insertTestOrganization(t *testing.T, name, slug string, parentID *int, now time.Time) (organizationResult *Organization) {
	t.Helper()
	var org *Organization

	org = &Organization{
		UUID:        slug + "-uuid",
		Name:        name,
		Slug:        slug,
		ParentOrgID: parentID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	{
		var err error

		if err = Organizations.Insert(org); err != nil {
			t.Fatalf("insert organization: %v", err)
		}
	}
	return org
}

func insertTestProject(t *testing.T, name, slug string, orgID int, now time.Time) (projectResult *Project) {
	t.Helper()
	var project *Project

	project = &Project{
		UUID:           slug + "-uuid",
		OrganizationID: orgID,
		Name:           name,
		Slug:           slug,
		ProjectType:    ProjectTypeLab,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	{
		var err error

		if err = Projects.Insert(project); err != nil {
			t.Fatalf("insert project: %v", err)
		}
	}
	return project
}

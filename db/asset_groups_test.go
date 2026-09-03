package db

import (
	"testing"
	"time"
)

func TestAssetGroupResourceAndAssignmentStayProjectScoped(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var org *Organization

	org = insertTestOrganization(t, "Lab", "lab", nil, now)
	var project *Project

	project = insertTestProject(t, "IT666", "it666", org.ID, now)
	var otherProject *Project

	otherProject = insertTestProject(t, "CS527", "cs527", org.ID, now)
	var resource *Resource

	resource = insertTestResource(t, project.ID, "charlie-vm", now)
	var otherResource *Resource

	otherResource = insertTestResource(t, otherProject.ID, "other-vm", now)
	var role *Role

	role = insertTestRoleWithPermission(t, "IT666 VM User", PermissionVMConsole, now)
	var (
		group *AssetGroup
		err   error
	)

	group, err = CreateAssetGroup(AssetGroupCreateInput{
		ProjectID: project.ID,
		Name:      "Student VMs",
	})
	if err != nil {
		t.Fatalf("CreateAssetGroup returned error: %v", err)
	}
	{
		var (
			created bool
			err     error
		)

		if created, err = EnsureAssetGroupResource(group.ID, resource.ID); err != nil || !created {
			t.Fatalf("expected asset group resource to be created, created=%v err=%v", created, err)
		}
	}
	{
		var err error

		if _, err = EnsureAssetGroupResource(group.ID, otherResource.ID); err == nil {
			t.Fatal("expected cross-project asset group resource to be rejected")
		}
	}
	var (
		assignment *AssetAssignment
		created    bool
	)

	assignment, created, err = EnsureAssetAssignment(AssetAssignmentInput{
		ProjectID:   project.ID,
		ResourceID:  &resource.ID,
		SubjectType: RoleBindingSubjectUser,
		SubjectID:   1001,
		RoleID:      role.ID,
	})
	if err != nil || !created {
		t.Fatalf("expected resource assignment to be created, created=%v err=%v", created, err)
	}
	if assignment.ProjectID != project.ID {
		t.Fatalf("expected assignment to remain project-owned, got %d", assignment.ProjectID)
	}
	var allowed bool

	allowed, err = HasPermission(PermissionCheck{
		UserID:     1001,
		Permission: PermissionVMConsole,
		ScopeType:  RoleBindingScopeResource,
		ScopeID:    &resource.ID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected direct asset assignment to grant resource permission")
	}
	var bindings []*RoleBinding

	bindings, err = RoleBindingsForSubject(RoleBindingSubjectUser, 1001)
	if err != nil {
		t.Fatalf("RoleBindingsForSubject returned error: %v", err)
	}
	var foundSourceBinding bool

	for _, binding := range bindings {
		if binding.SourceType == RoleBindingSourceAssetAssignment && binding.SourceID != nil && *binding.SourceID == assignment.ID {
			foundSourceBinding = true
		}
	}
	if !foundSourceBinding {
		t.Fatalf("expected assignment-derived binding to be source tagged, got %#v", bindings)
	}
}

func TestCreateResourceValidatesProjectAndDuplicateSlug(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var org *Organization

	org = insertTestOrganization(t, "Lab", "lab", nil, now)
	var project *Project

	project = insertTestProject(t, "IT666", "it666", org.ID, now)
	var resource *Resource
	var err error

	resource, err = CreateResource(ResourceCreateInput{
		ProjectID:    project.ID,
		Name:         "Student VM",
		Slug:         "student-vm",
		ResourceType: ResourceTypeVM,
	})
	if err != nil {
		t.Fatalf("CreateResource returned error: %v", err)
	}
	if resource.Status != ResourceStatusReady || resource.OwnerType != OwnerTypeProject || resource.OwnerID != project.ID {
		t.Fatalf("expected ready project-owned resource, got %#v", resource)
	}
	if _, err = CreateResource(ResourceCreateInput{
		ProjectID:    project.ID,
		Name:         "Duplicate Student VM",
		Slug:         "student-vm",
		ResourceType: ResourceTypeVM,
	}); err == nil {
		t.Fatal("expected duplicate resource slug to be rejected")
	}
	var inactiveProject *Project

	inactiveProject = insertTestProject(t, "Archived", "archived", org.ID, now)
	inactiveProject.IsActive = false
	if err = Projects.Update(inactiveProject); err != nil {
		t.Fatalf("Projects.Update returned error: %v", err)
	}
	if _, err = CreateResource(ResourceCreateInput{
		ProjectID:    inactiveProject.ID,
		Name:         "Archived VM",
		ResourceType: ResourceTypeVM,
	}); err == nil {
		t.Fatal("expected inactive project resource creation to be rejected")
	}
	if _, err = CreateResource(ResourceCreateInput{
		ProjectID:    99999,
		Name:         "Missing VM",
		ResourceType: ResourceTypeVM,
	}); err == nil {
		t.Fatal("expected missing project resource creation to be rejected")
	}
}

func TestAssetGroupAssignmentMaintainsSourceTaggedDerivedBindings(t *testing.T) {
	initTestDB(t)
	var now time.Time

	now = time.Now().UTC()
	var org *Organization

	org = insertTestOrganization(t, "Lab", "lab", nil, now)
	var project *Project

	project = insertTestProject(t, "IT666", "it666", org.ID, now)
	var firstResource *Resource

	firstResource = insertTestResource(t, project.ID, "student-vm-1", now)
	var secondResource *Resource

	secondResource = insertTestResource(t, project.ID, "student-vm-2", now)
	var role *Role

	role = insertTestRoleWithPermission(t, "IT666 VM User", PermissionVMConsole, now)
	var (
		group *AssetGroup
		err   error
	)

	group, err = CreateAssetGroup(AssetGroupCreateInput{
		ProjectID: project.ID,
		Name:      "Student VMs",
	})
	if err != nil {
		t.Fatalf("CreateAssetGroup returned error: %v", err)
	}
	if _, err = EnsureAssetGroupResource(group.ID, firstResource.ID); err != nil {
		t.Fatalf("EnsureAssetGroupResource first returned error: %v", err)
	}
	var assignment *AssetAssignment

	assignment, _, err = EnsureAssetAssignment(AssetAssignmentInput{
		ProjectID:    project.ID,
		AssetGroupID: &group.ID,
		SubjectType:  RoleBindingSubjectUser,
		SubjectID:    1001,
		RoleID:       role.ID,
	})
	if err != nil {
		t.Fatalf("EnsureAssetAssignment returned error: %v", err)
	}
	assertHasResourcePermission(t, 1001, PermissionVMConsole, firstResource.ID, true)
	assertSourceTaggedBinding(t, RoleBindingSubjectUser, 1001, assignment.ID, firstResource.ID, true)

	if _, err = EnsureAssetGroupResource(group.ID, secondResource.ID); err != nil {
		t.Fatalf("EnsureAssetGroupResource second returned error: %v", err)
	}
	assertHasResourcePermission(t, 1001, PermissionVMConsole, secondResource.ID, true)
	assertSourceTaggedBinding(t, RoleBindingSubjectUser, 1001, assignment.ID, secondResource.ID, true)

	if _, err = ensureRoleBinding(role.ID, RoleBindingSubjectUser, 1001, RoleBindingScopeResource, &firstResource.ID, now); err != nil {
		t.Fatalf("ensure manual role binding returned error: %v", err)
	}
	if err = ArchiveAssetAssignment(assignment.ID); err != nil {
		t.Fatalf("ArchiveAssetAssignment returned error: %v", err)
	}
	assertSourceTaggedBinding(t, RoleBindingSubjectUser, 1001, assignment.ID, firstResource.ID, false)
	assertSourceTaggedBinding(t, RoleBindingSubjectUser, 1001, assignment.ID, secondResource.ID, false)
	assertHasResourcePermission(t, 1001, PermissionVMConsole, firstResource.ID, true)
	assertHasResourcePermission(t, 1001, PermissionVMConsole, secondResource.ID, false)
}

func assertHasResourcePermission(t *testing.T, userID int, permission PermissionKey, resourceID int, expected bool) {
	t.Helper()
	var (
		allowed bool
		err     error
	)

	allowed, err = HasPermission(PermissionCheck{
		UserID:     userID,
		Permission: permission,
		ScopeType:  RoleBindingScopeResource,
		ScopeID:    &resourceID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if allowed != expected {
		t.Fatalf("expected allowed=%v for resource %d, got %v", expected, resourceID, allowed)
	}
}

func assertSourceTaggedBinding(t *testing.T, subjectType RoleBindingSubject, subjectID int, assignmentID int, resourceID int, expected bool) {
	t.Helper()
	var (
		bindings []*RoleBinding
		err      error
	)

	bindings, err = RoleBindingsForSubject(subjectType, subjectID)
	if err != nil {
		t.Fatalf("RoleBindingsForSubject returned error: %v", err)
	}
	for _, binding := range bindings {
		if binding.ScopeType == RoleBindingScopeResource &&
			binding.ScopeID != nil &&
			*binding.ScopeID == resourceID &&
			binding.SourceType == RoleBindingSourceAssetAssignment &&
			binding.SourceID != nil &&
			*binding.SourceID == assignmentID {
			if !expected {
				t.Fatalf("did not expect source tagged binding for assignment %d resource %d", assignmentID, resourceID)
			}
			return
		}
	}
	if expected {
		t.Fatalf("expected source tagged binding for assignment %d resource %d, got %#v", assignmentID, resourceID, bindings)
	}
}

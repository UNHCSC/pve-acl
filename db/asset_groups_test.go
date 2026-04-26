package db

import (
	"testing"
	"time"
)

func TestAssetGroupResourceAndAssignmentStayProjectScoped(t *testing.T) {
	initTestDB(t)
	now := time.Now().UTC()

	org := insertTestOrganization(t, "Lab", "lab", nil, now)
	project := insertTestProject(t, "IT666", "it666", org.ID, now)
	otherProject := insertTestProject(t, "CS527", "cs527", org.ID, now)
	resource := insertTestResource(t, project.ID, "charlie-vm", now)
	otherResource := insertTestResource(t, otherProject.ID, "other-vm", now)
	role := insertTestRoleWithPermission(t, "IT666 VM User", PermissionVMConsole, now)

	group, err := CreateAssetGroup(AssetGroupCreateInput{
		ProjectID: project.ID,
		Name:      "Student VMs",
	})
	if err != nil {
		t.Fatalf("CreateAssetGroup returned error: %v", err)
	}

	if created, err := EnsureAssetGroupResource(group.ID, resource.ID); err != nil || !created {
		t.Fatalf("expected asset group resource to be created, created=%v err=%v", created, err)
	}
	if _, err := EnsureAssetGroupResource(group.ID, otherResource.ID); err == nil {
		t.Fatal("expected cross-project asset group resource to be rejected")
	}

	assignment, created, err := EnsureAssetAssignment(AssetAssignmentInput{
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

	allowed, err := HasPermission(PermissionCheck{
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
}

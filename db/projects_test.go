package db

import "testing"

func TestCreateProjectUsesDefaultOrganization(t *testing.T) {
	initTestDB(t)
	if err := EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := CreateProject(ProjectCreateInput{
		Name:        "Training Lab",
		Description: "Local-only test project",
		ProjectType: ProjectTypeLab,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	if project.ID == 0 {
		t.Fatal("expected project ID to be set")
	}
	if project.Slug != "training-lab" {
		t.Fatalf("expected slug training-lab, got %q", project.Slug)
	}
	if !project.IsActive {
		t.Fatal("expected project to be active")
	}

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected one project, got %d", len(projects))
	}
}

func TestCreateProjectRejectsDuplicateSlug(t *testing.T) {
	initTestDB(t)
	if err := EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	if _, err := CreateProject(ProjectCreateInput{Name: "Training Lab"}); err != nil {
		t.Fatalf("first CreateProject returned error: %v", err)
	}
	if _, err := CreateProject(ProjectCreateInput{Name: "Training Lab"}); err == nil {
		t.Fatal("expected duplicate project slug to be rejected")
	}
}

func TestCreateProjectRejectsArchivedOrganization(t *testing.T) {
	initTestDB(t)
	if err := EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}
	root, found, err := GetOrganizationBySlug(DefaultRootOrganizationSlug)
	if err != nil || !found {
		t.Fatalf("expected root organization, found=%v err=%v", found, err)
	}

	org, err := CreateOrganization(OrganizationCreateInput{
		Name:        "Archived Teaching",
		Slug:        "archived-teaching",
		ParentOrgID: &root.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}
	if err := ArchiveOrganization(org); err != nil {
		t.Fatalf("ArchiveOrganization returned error: %v", err)
	}

	if _, err := CreateProject(ProjectCreateInput{
		Name:           "Should Not Attach",
		OrganizationID: org.ID,
		ProjectType:    ProjectTypeLab,
	}); err == nil {
		t.Fatal("expected archived organization to be rejected")
	}
}

func TestCreateOrganizationRejectsSecondRoot(t *testing.T) {
	initTestDB(t)
	if err := EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	if _, err := CreateOrganization(OrganizationCreateInput{
		Name: "Another Root",
		Slug: "another-root",
	}); err == nil {
		t.Fatal("expected second root organization to be rejected")
	}
}

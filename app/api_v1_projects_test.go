package app

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func TestProjectAPIListAndCreate(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	token := authenticateTestUser(t, "project-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/projects", getProjects)
	fiberApp.Post("/api/v1/projects", postCreateProject)

	createBody := bytes.NewBufferString(`{"name":"Blue Team Practice","description":"Local project shell"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/projects", createBody)
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := fiberApp.Test(createReq)
	if err != nil {
		t.Fatalf("create route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	listReq := httptest.NewRequest("GET", "/api/v1/projects", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := fiberApp.Test(listReq)
	if err != nil {
		t.Fatalf("list route returned error: %v", err)
	}
	if listResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var projects []db.Project
	if err := json.NewDecoder(listResp.Body).Decode(&projects); err != nil {
		t.Fatalf("decode projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected one project, got %d", len(projects))
	}
	if projects[0].Slug != "blue-team-practice" {
		t.Fatalf("expected slug blue-team-practice, got %q", projects[0].Slug)
	}

	memberships, err := db.ProjectMembershipsForProject(projects[0].ID)
	if err != nil {
		t.Fatalf("ProjectMembershipsForProject returned error: %v", err)
	}
	if len(memberships) != 1 {
		t.Fatalf("expected creator owner membership, got %d memberships", len(memberships))
	}
	if memberships[0].SubjectType != db.ProjectMemberSubjectUser {
		t.Fatalf("expected creator owner membership, got %#v", memberships[0])
	}
	projectRole, found, err := db.ProjectMemberAccessRole(projects[0].ID, db.ProjectMemberSubjectUser, memberships[0].SubjectID)
	if err != nil || !found || projectRole != db.ProjectRoleOwner {
		t.Fatalf("expected creator owner role binding, role=%v found=%v err=%v", projectRole, found, err)
	}
}

func TestProjectAPIDeniesNonAdminCreate(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	token := authenticateTestUser(t, "project-viewer", false)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/projects", postCreateProject)

	createBody := bytes.NewBufferString(`{"name":"Denied Project"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/projects", createBody)
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	resp, err := fiberApp.Test(createReq)
	if err != nil {
		t.Fatalf("create route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestProjectAPISiteAdminCanViewAnyProject(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        "CSC Team",
		Description: "Created by site admin",
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	token := authenticateTestUser(t, "site-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/projects/:slug", getProjectBySlug)

	req := httptest.NewRequest("GET", "/api/v1/projects/"+project.Slug, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("project detail route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProjectAPINotFoundIsNotUnauthorized(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	token := authenticateTestUser(t, "site-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/projects/:slug", getProjectBySlug)

	req := httptest.NewRequest("GET", "/api/v1/projects/missing-project", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("project detail route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestProjectAPICreatorCanViewCreatedProject(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	token := authenticateTestUser(t, "project-creator", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/projects", postCreateProject)
	fiberApp.Get("/api/v1/projects/:slug", getProjectBySlug)

	createBody := bytes.NewBufferString(`{"name":"CSC Team","slug":"csc-team"}`)
	createReq := httptest.NewRequest("POST", "/api/v1/projects", createBody)
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := fiberApp.Test(createReq)
	if err != nil {
		t.Fatalf("create route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	viewReq := httptest.NewRequest("GET", "/api/v1/projects/csc-team", nil)
	viewReq.Header.Set("Authorization", "Bearer "+token)

	viewResp, err := fiberApp.Test(viewReq)
	if err != nil {
		t.Fatalf("view route returned error: %v", err)
	}
	if viewResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", viewResp.StatusCode)
	}
}

func TestProjectAPIOwnerMembershipCanViewProject(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        "Owner Visible",
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	dbUser, _, err := db.EnsureUser("project-owner", "Project Owner", "owner@example.test", "local", "project-owner")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, dbUser.ID); err != nil {
		t.Fatalf("EnsureProjectMembership returned error: %v", err)
	}

	token := authenticateTestUser(t, "project-owner", false)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/projects/:slug", getProjectBySlug)

	req := httptest.NewRequest("GET", "/api/v1/projects/"+project.Slug, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("view route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProjectMembershipsIncludeHumanReadableSubjects(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        "Readable Members",
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	dbUser, _, err := db.EnsureUser("readable-user", "Readable User", "readable@example.test", "local", "readable-user")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, dbUser.ID); err != nil {
		t.Fatalf("EnsureProjectMembership returned error: %v", err)
	}

	token := authenticateTestUser(t, "readable-user", false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/projects/:id/memberships", getProjectMemberships)

	req := httptest.NewRequest("GET", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("membership route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode memberships: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected one membership, got %d", len(body))
	}
	subject, ok := body[0]["subject"].(map[string]any)
	if !ok {
		t.Fatalf("expected subject payload, got %#v", body[0])
	}
	if subject["label"] != "Readable User" {
		t.Fatalf("expected readable label, got %#v", subject["label"])
	}
}

func TestProjectManagerCanAddProjectMembershipByRef(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        "Scoped Access",
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	manager, _, err := db.EnsureUser("project-manager", "Project Manager", "manager@example.test", "local", "project-manager")
	if err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	target, _, err := db.EnsureUser("project-target", "Project Target", "target@example.test", "local", "project-target")
	if err != nil {
		t.Fatalf("EnsureUser target returned error: %v", err)
	}
	if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, manager.ID); err != nil {
		t.Fatalf("EnsureProjectMembership manager returned error: %v", err)
	}
	if err := db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, manager.ID, db.ProjectRoleManager); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole manager returned error: %v", err)
	}

	token := authenticateTestUser(t, "project-manager", false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/projects/:id/memberships", postCreateProjectMembership)

	req := httptest.NewRequest("POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships", bytes.NewBufferString(`{
		"subjectType": "user",
		"subjectRef": "project-target",
		"projectRole": "developer"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("membership route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created membership: %v", err)
	}
	if created["subject_type"] == nil || created["project_role"] == nil {
		t.Fatalf("expected created membership payload, got %#v", created)
	}

	assertProjectMemberRole(t, project.ID, db.ProjectMemberSubjectUser, target.ID, db.ProjectRoleDeveloper)
}

func TestProjectManagerCanAddProjectGroupByExternalRef(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        "Group Scoped Access",
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	manager, _, err := db.EnsureUser("project-group-manager", "Project Group Manager", "manager@example.test", "local", "project-group-manager")
	if err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	group, _, err := db.EnsureCloudGroup("Teaching Staff", "teaching-staff", db.GroupTypeProject)
	if err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	group.SyncSource = db.CloudGroupSyncSourceLDAP
	group.ExternalID = "ipa-teaching-staff"
	group.SyncMembership = true
	if err := db.UpdateCloudGroup(group); err != nil {
		t.Fatalf("UpdateCloudGroup returned error: %v", err)
	}
	if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, manager.ID); err != nil {
		t.Fatalf("EnsureProjectMembership manager returned error: %v", err)
	}
	if err := db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, manager.ID, db.ProjectRoleManager); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole manager returned error: %v", err)
	}

	token := authenticateTestUser(t, "project-group-manager", false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/projects/:id/memberships", postCreateProjectMembership)

	req := httptest.NewRequest("POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships", bytes.NewBufferString(`{
		"subjectType": "group",
		"subjectRef": "ipa-teaching-staff",
		"projectRole": "developer"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("membership route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created membership: %v", err)
	}
	subject, ok := created["subject"].(map[string]any)
	if !ok {
		t.Fatalf("expected subject payload, got %#v", created)
	}
	if subject["slug"] != "teaching-staff" {
		t.Fatalf("expected group subject payload, got %#v", subject)
	}

	assertProjectMemberRole(t, project.ID, db.ProjectMemberSubjectGroup, group.ID, db.ProjectRoleDeveloper)
}

func TestProjectManagerCanPatchProjectMembershipRole(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        "Patch Membership",
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	manager, _, err := db.EnsureUser("project-role-manager", "Project Role Manager", "manager@example.test", "local", "project-role-manager")
	if err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	target, _, err := db.EnsureUser("project-role-target", "Project Role Target", "target@example.test", "local", "project-role-target")
	if err != nil {
		t.Fatalf("EnsureUser target returned error: %v", err)
	}
	if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, manager.ID); err != nil {
		t.Fatalf("EnsureProjectMembership manager returned error: %v", err)
	}
	if err := db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, manager.ID, db.ProjectRoleManager); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole manager returned error: %v", err)
	}
	if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, target.ID); err != nil {
		t.Fatalf("EnsureProjectMembership target returned error: %v", err)
	}
	if err := db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, target.ID, db.ProjectRoleViewer); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole target returned error: %v", err)
	}

	var targetMembership *db.ProjectMembership
	memberships, err := db.ProjectMembershipsForProject(project.ID)
	if err != nil {
		t.Fatalf("ProjectMembershipsForProject returned error: %v", err)
	}
	for _, membership := range memberships {
		if membership.SubjectID == target.ID {
			targetMembership = membership
			break
		}
	}
	if targetMembership == nil {
		t.Fatal("expected target membership")
	}

	token := authenticateTestUser(t, "project-role-manager", false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Patch("/api/v1/projects/:id/memberships/:membershipID", patchProjectMembership)

	req := httptest.NewRequest("PATCH", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships/"+strconv.Itoa(targetMembership.ID), bytes.NewBufferString(`{
		"projectRole": "owner"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("patch membership route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	assertProjectMemberRole(t, project.ID, db.ProjectMemberSubjectUser, target.ID, db.ProjectRoleOwner)
}

func TestProjectAPIDeleteRemovesProject(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        "Delete Me",
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	token := authenticateTestUser(t, "delete-admin", true)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Delete("/api/v1/projects/:slug", deleteProjectBySlug)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/"+project.Slug, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("delete route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	if _, found, err := db.GetProjectBySlug(project.Slug); err != nil || found {
		t.Fatalf("expected project to be deleted, found=%v err=%v", found, err)
	}
}

func TestProjectAPIDeleteAllowsOrgScopedProjectManager(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	root, found, err := db.GetOrganizationBySlug(db.DefaultRootOrganizationSlug)
	if err != nil || !found {
		t.Fatalf("expected default root organization, found=%v err=%v", found, err)
	}
	child, err := db.CreateOrganization(db.OrganizationCreateInput{
		Name:        "Scoped Delete",
		Slug:        "scoped-delete",
		ParentOrgID: &root.ID,
	})
	if err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}
	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:           "Org Scoped Delete",
		ProjectType:    db.ProjectTypeCustom,
		OrganizationID: child.ID,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	user, _, err := db.EnsureUser("org-project-manager", "Org Project Manager", "org-project-manager@example.test", "local", "org-project-manager")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	role, _, err := db.EnsureRole("Scoped Project Manager", "Can manage projects below an org", false)
	if err != nil {
		t.Fatalf("EnsureRole returned error: %v", err)
	}
	permission, found, err := db.GetPermissionByName(db.PermissionProjectManage.String())
	if err != nil || !found {
		t.Fatalf("expected project.manage permission, found=%v err=%v", found, err)
	}
	if _, err := db.EnsureRolePermission(role.ID, permission.ID); err != nil {
		t.Fatalf("EnsureRolePermission returned error: %v", err)
	}
	if _, err := db.EnsureRoleBinding(role.ID, db.RoleBindingSubjectUser, user.ID, db.RoleBindingScopeOrg, &root.ID); err != nil {
		t.Fatalf("EnsureRoleBinding returned error: %v", err)
	}

	token := authenticateTestUser(t, user.Username, false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Delete("/api/v1/projects/:slug", deleteProjectBySlug)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/"+project.Slug, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("delete route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func assertProjectMemberRole(t *testing.T, projectID int, subjectType db.ProjectMemberSubject, subjectID int, expected db.ProjectRole) {
	t.Helper()
	role, found, err := db.ProjectMemberAccessRole(projectID, subjectType, subjectID)
	if err != nil || !found || role != expected {
		t.Fatalf("expected project member role %v, got role=%v found=%v err=%v", expected, role, found, err)
	}
}

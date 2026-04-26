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

	ownerReq := httptest.NewRequest("PATCH", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships/"+strconv.Itoa(targetMembership.ID), bytes.NewBufferString(`{
		"projectRole": "owner"
	}`))
	ownerReq.Header.Set("Authorization", "Bearer "+token)
	ownerReq.Header.Set("Content-Type", "application/json")

	ownerResp, err := fiberApp.Test(ownerReq)
	if err != nil {
		t.Fatalf("patch owner membership route returned error: %v", err)
	}
	if ownerResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected owner promotion to be forbidden, got %d", ownerResp.StatusCode)
	}

	req := httptest.NewRequest("PATCH", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships/"+strconv.Itoa(targetMembership.ID), bytes.NewBufferString(`{
		"projectRole": "developer"
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

	assertProjectMemberRole(t, project.ID, db.ProjectMemberSubjectUser, target.ID, db.ProjectRoleDeveloper)
}

func TestProjectManagerCanCreateScopedRoleAndAssignOnlyContainedPermissions(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        "Scoped Role Project",
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	manager, _, err := db.EnsureUser("scoped-project-manager", "Scoped Project Manager", "manager@example.test", "local", "scoped-project-manager")
	if err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	target, _, err := db.EnsureUser("scoped-project-target", "Scoped Project Target", "target@example.test", "local", "scoped-project-target")
	if err != nil {
		t.Fatalf("EnsureUser target returned error: %v", err)
	}
	if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, manager.ID); err != nil {
		t.Fatalf("EnsureProjectMembership manager returned error: %v", err)
	}
	if err := db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, manager.ID, db.ProjectRoleManager); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole manager returned error: %v", err)
	}

	token := authenticateTestUser(t, manager.Username, false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/roles", postCreateRole)
	fiberApp.Post("/api/v1/roles/:id/permissions", postCreateRolePermission)
	fiberApp.Get("/api/v1/projects/:id/roles", getProjectAssignableRoles)
	fiberApp.Post("/api/v1/projects/:id/memberships", postCreateProjectMembership)

	createRoleBody := `{"name":"Scoped VM Reader","description":"Read only inside project","scopeType":"project","scopeID":` + strconv.Itoa(project.ID) + `}`
	createRoleReq := httptest.NewRequest("POST", "/api/v1/roles", bytes.NewBufferString(createRoleBody))
	createRoleReq.Header.Set("Authorization", "Bearer "+token)
	createRoleReq.Header.Set("Content-Type", "application/json")
	createRoleResp, err := fiberApp.Test(createRoleReq)
	if err != nil {
		t.Fatalf("create scoped role route returned error: %v", err)
	}
	if createRoleResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createRoleResp.StatusCode)
	}
	var role map[string]any
	if err := json.NewDecoder(createRoleResp.Body).Decode(&role); err != nil {
		t.Fatalf("decode scoped role: %v", err)
	}
	roleID := int(role["id"].(float64))

	vmRead, found, err := db.GetPermissionByName(db.PermissionVMRead.String())
	if err != nil || !found {
		t.Fatalf("expected vm.read permission, found=%v err=%v", found, err)
	}
	grantReq := httptest.NewRequest("POST", "/api/v1/roles/"+strconv.Itoa(roleID)+"/permissions", bytes.NewBufferString(`{"permissionID":`+strconv.Itoa(vmRead.ID)+`}`))
	grantReq.Header.Set("Authorization", "Bearer "+token)
	grantReq.Header.Set("Content-Type", "application/json")
	grantResp, err := fiberApp.Test(grantReq)
	if err != nil {
		t.Fatalf("grant scoped permission route returned error: %v", err)
	}
	if grantResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected vm.read grant to succeed, got %d", grantResp.StatusCode)
	}

	userManage, found, err := db.GetPermissionByName(db.PermissionUserManage.String())
	if err != nil || !found {
		t.Fatalf("expected user.manage permission, found=%v err=%v", found, err)
	}
	deniedGrantReq := httptest.NewRequest("POST", "/api/v1/roles/"+strconv.Itoa(roleID)+"/permissions", bytes.NewBufferString(`{"permissionID":`+strconv.Itoa(userManage.ID)+`}`))
	deniedGrantReq.Header.Set("Authorization", "Bearer "+token)
	deniedGrantReq.Header.Set("Content-Type", "application/json")
	deniedGrantResp, err := fiberApp.Test(deniedGrantReq)
	if err != nil {
		t.Fatalf("grant denied scoped permission route returned error: %v", err)
	}
	if deniedGrantResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected user.manage grant to be forbidden, got %d", deniedGrantResp.StatusCode)
	}

	rolesReq := httptest.NewRequest("GET", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/roles", nil)
	rolesReq.Header.Set("Authorization", "Bearer "+token)
	rolesResp, err := fiberApp.Test(rolesReq)
	if err != nil {
		t.Fatalf("project roles route returned error: %v", err)
	}
	if rolesResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", rolesResp.StatusCode)
	}
	var roles []map[string]any
	if err := json.NewDecoder(rolesResp.Body).Decode(&roles); err != nil {
		t.Fatalf("decode assignable roles: %v", err)
	}
	foundScopedRole := false
	for _, item := range roles {
		if int(item["id"].(float64)) == roleID {
			foundScopedRole = true
		}
		if item["name"] == db.DefaultProjectOwnerRoleName {
			t.Fatalf("project manager should not be offered owner role: %#v", roles)
		}
	}
	if !foundScopedRole {
		t.Fatalf("expected scoped role to be assignable, got %#v", roles)
	}

	memberBody := `{"subjectType":"user","subjectRef":"` + target.Username + `","roleID":` + strconv.Itoa(roleID) + `}`
	memberReq := httptest.NewRequest("POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships", bytes.NewBufferString(memberBody))
	memberReq.Header.Set("Authorization", "Bearer "+token)
	memberReq.Header.Set("Content-Type", "application/json")
	memberResp, err := fiberApp.Test(memberReq)
	if err != nil {
		t.Fatalf("project member route returned error: %v", err)
	}
	if memberResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", memberResp.StatusCode)
	}

	bindings, err := db.RoleBindingsForSubject(db.RoleBindingSubjectUser, target.ID)
	if err != nil {
		t.Fatalf("RoleBindingsForSubject returned error: %v", err)
	}
	for _, binding := range bindings {
		if binding.RoleID == roleID && binding.ScopeType == db.RoleBindingScopeProject && binding.ScopeID != nil && *binding.ScopeID == project.ID {
			return
		}
	}
	t.Fatalf("expected target to receive scoped role binding, got %#v", bindings)
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

func TestOrganizationAPIDeleteArchivesEmptyOrganization(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	org, err := db.CreateOrganization(db.OrganizationCreateInput{
		Name:        "Archive Org",
		Slug:        "archive-org",
		ParentOrgID: rootOrgIDPtr(t),
	})
	if err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}

	token := authenticateTestUser(t, "org-archive-admin", true)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Delete("/api/v1/organizations/:id", deleteOrganization)
	fiberApp.Get("/api/v1/projects/tree", getProjectTree)

	req := httptest.NewRequest("DELETE", "/api/v1/organizations/"+strconv.Itoa(org.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("delete organization route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	archived, found, err := db.GetOrganizationByID(org.ID)
	if err != nil || !found {
		t.Fatalf("expected archived organization to remain, found=%v err=%v", found, err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("expected organization to be archived")
	}

	treeReq := httptest.NewRequest("GET", "/api/v1/projects/tree", nil)
	treeReq.Header.Set("Authorization", "Bearer "+token)
	treeResp, err := fiberApp.Test(treeReq)
	if err != nil {
		t.Fatalf("project tree route returned error: %v", err)
	}
	if treeResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", treeResp.StatusCode)
	}
	var tree map[string][]map[string]any
	if err := json.NewDecoder(treeResp.Body).Decode(&tree); err != nil {
		t.Fatalf("decode project tree: %v", err)
	}
	for _, item := range tree["organizations"] {
		if item["slug"] == "archive-org" {
			t.Fatalf("archived organization should be omitted from tree: %#v", tree["organizations"])
		}
	}
}

func TestOrganizationAPIDeniesSecondRootCreateAndRootMove(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	child, err := db.CreateOrganization(db.OrganizationCreateInput{
		Name:        "Child Org",
		Slug:        "child-org",
		ParentOrgID: rootOrgIDPtr(t),
	})
	if err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}

	token := authenticateTestUser(t, "root-guard-admin", true)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/organizations", postCreateOrganization)
	fiberApp.Patch("/api/v1/organizations/:id", patchOrganization)

	createReq := httptest.NewRequest("POST", "/api/v1/organizations", bytes.NewBufferString(`{"name":"Second Root","slug":"second-root"}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := fiberApp.Test(createReq)
	if err != nil {
		t.Fatalf("create organization route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected second root create to return 400, got %d", createResp.StatusCode)
	}

	moveReq := httptest.NewRequest("PATCH", "/api/v1/organizations/"+strconv.Itoa(child.ID), bytes.NewBufferString(`{"parentOrgID":null}`))
	moveReq.Header.Set("Authorization", "Bearer "+token)
	moveReq.Header.Set("Content-Type", "application/json")
	moveResp, err := fiberApp.Test(moveReq)
	if err != nil {
		t.Fatalf("patch organization route returned error: %v", err)
	}
	if moveResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected move to root to return 400, got %d", moveResp.StatusCode)
	}
}

func TestOrganizationAPIDeleteStillRejectsOrganizationsWithProjects(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	org, err := db.CreateOrganization(db.OrganizationCreateInput{
		Name:        "Busy Org",
		Slug:        "busy-org",
		ParentOrgID: rootOrgIDPtr(t),
	})
	if err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}
	if _, err := db.CreateProject(db.ProjectCreateInput{
		Name:           "Busy Project",
		OrganizationID: org.ID,
		ProjectType:    db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	token := authenticateTestUser(t, "busy-org-admin", true)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Delete("/api/v1/organizations/:id", deleteOrganization)

	req := httptest.NewRequest("DELETE", "/api/v1/organizations/"+strconv.Itoa(org.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("delete busy organization route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	stillActive, found, err := db.GetOrganizationByID(org.ID)
	if err != nil || !found {
		t.Fatalf("expected organization to remain, found=%v err=%v", found, err)
	}
	if stillActive.ArchivedAt != nil {
		t.Fatal("expected organization with projects not to be archived")
	}
}

func assertProjectMemberRole(t *testing.T, projectID int, subjectType db.ProjectMemberSubject, subjectID int, expected db.ProjectRole) {
	t.Helper()
	role, found, err := db.ProjectMemberAccessRole(projectID, subjectType, subjectID)
	if err != nil || !found || role != expected {
		t.Fatalf("expected project member role %v, got role=%v found=%v err=%v", expected, role, found, err)
	}
}

func rootOrgID(t *testing.T) int {
	t.Helper()
	root, found, err := db.GetOrganizationBySlug(db.DefaultRootOrganizationSlug)
	if err != nil || !found {
		t.Fatalf("expected default root organization, found=%v err=%v", found, err)
	}
	return root.ID
}

func rootOrgIDPtr(t *testing.T) *int {
	t.Helper()
	id := rootOrgID(t)
	return &id
}

package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func TestProjectAPIListAndCreate(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		token       string     = authenticateTestUser(t, "project-admin", true)
		fiberApp    *fiber.App = newAuthenticatedFiberApp()
		createReq   *http.Request
		createResp  *http.Response
		listReq     *http.Request
		listResp    *http.Response
		projects    []db.Project
		memberships []*db.ProjectMembership
		projectRole db.ProjectRole
		found       bool
		err         error
	)

	fiberApp.Get("/api/v1/projects", getProjects)
	fiberApp.Post("/api/v1/projects", postCreateProject)

	createReq = httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBufferString(`{"name":"Blue Team Practice","description":"Local project shell"}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	if createResp, err = fiberApp.Test(createReq); err != nil {
		t.Fatalf("create route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	listReq = httptest.NewRequest("GET", "/api/v1/projects", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	if listResp, err = fiberApp.Test(listReq); err != nil {
		t.Fatalf("list route returned error: %v", err)
	}
	if listResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	if err = json.NewDecoder(listResp.Body).Decode(&projects); err != nil {
		t.Fatalf("decode projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected one project, got %d", len(projects))
	}
	if projects[0].Slug != "blue-team-practice" {
		t.Fatalf("expected slug blue-team-practice, got %q", projects[0].Slug)
	}

	if memberships, err = db.ProjectMembershipsForProject(projects[0].ID); err != nil {
		t.Fatalf("ProjectMembershipsForProject returned error: %v", err)
	}
	if len(memberships) != 1 {
		t.Fatalf("expected creator owner membership, got %d memberships", len(memberships))
	}
	if memberships[0].SubjectType != db.ProjectMemberSubjectUser {
		t.Fatalf("expected creator owner membership, got %#v", memberships[0])
	}
	if projectRole, found, err = db.ProjectMemberAccessRole(projects[0].ID, db.ProjectMemberSubjectUser, memberships[0].SubjectID); err != nil || !found || projectRole != db.ProjectRoleOwner {
		t.Fatalf("expected creator owner role binding, role=%v found=%v err=%v", projectRole, found, err)
	}
}

func TestProjectAPIDeniesNonAdminCreate(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		token     string     = authenticateTestUser(t, "project-viewer", false)
		fiberApp  *fiber.App = newAuthenticatedFiberApp()
		createReq *http.Request
		resp      *http.Response
		err       error
	)

	fiberApp.Post("/api/v1/projects", postCreateProject)

	createReq = httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBufferString(`{"name":"Denied Project"}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	if resp, err = fiberApp.Test(createReq); err != nil {
		t.Fatalf("create route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestProjectAPISiteAdminCanViewAnyProject(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		project  *db.Project
		token    string
		fiberApp *fiber.App
		req      *http.Request
		resp     *http.Response
		err      error
	)

	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        "CSC Team",
		Description: "Created by site admin",
		ProjectType: db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	token = authenticateTestUser(t, "site-admin", true)

	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Get("/api/v1/projects/:slug", getProjectBySlug)

	req = httptest.NewRequest("GET", "/api/v1/projects/"+project.Slug, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("project detail route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProjectAPINotFoundIsNotUnauthorized(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		token    string     = authenticateTestUser(t, "site-admin", true)
		fiberApp *fiber.App = newAuthenticatedFiberApp()
		req      *http.Request
		resp     *http.Response
		err      error
	)

	fiberApp.Get("/api/v1/projects/:slug", getProjectBySlug)

	req = httptest.NewRequest("GET", "/api/v1/projects/missing-project", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("project detail route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestProjectAPICreatorCanViewCreatedProject(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		token      string     = authenticateTestUser(t, "project-creator", true)
		fiberApp   *fiber.App = newAuthenticatedFiberApp()
		createReq  *http.Request
		createResp *http.Response
		viewReq    *http.Request
		viewResp   *http.Response
		err        error
	)

	fiberApp.Post("/api/v1/projects", postCreateProject)
	fiberApp.Get("/api/v1/projects/:slug", getProjectBySlug)

	createReq = httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBufferString(`{"name":"CSC Team","slug":"csc-team"}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	if createResp, err = fiberApp.Test(createReq); err != nil {
		t.Fatalf("create route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	viewReq = httptest.NewRequest("GET", "/api/v1/projects/csc-team", nil)
	viewReq.Header.Set("Authorization", "Bearer "+token)

	if viewResp, err = fiberApp.Test(viewReq); err != nil {
		t.Fatalf("view route returned error: %v", err)
	}
	if viewResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", viewResp.StatusCode)
	}
}

func TestProjectAPIOwnerMembershipCanViewProject(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		project  *db.Project
		dbUser   *db.User
		token    string
		fiberApp *fiber.App
		req      *http.Request
		resp     *http.Response
		err      error
	)

	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        "Owner Visible",
		ProjectType: db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	if dbUser, _, err = db.EnsureUser("project-owner", "Project Owner", "owner@example.test", "local", "project-owner"); err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if _, err = db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, dbUser.ID); err != nil {
		t.Fatalf("EnsureProjectMembership returned error: %v", err)
	}

	token = authenticateTestUser(t, "project-owner", false)

	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Get("/api/v1/projects/:slug", getProjectBySlug)

	req = httptest.NewRequest("GET", "/api/v1/projects/"+project.Slug, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("view route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProjectMembershipsIncludeHumanReadableSubjects(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		project  *db.Project
		dbUser   *db.User
		token    string
		fiberApp *fiber.App
		req      *http.Request
		resp     *http.Response
		body     []map[string]any
		subject  map[string]any
		ok       bool
		err      error
	)

	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        "Readable Members",
		ProjectType: db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if dbUser, _, err = db.EnsureUser("readable-user", "Readable User", "readable@example.test", "local", "readable-user"); err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if _, err = db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, dbUser.ID); err != nil {
		t.Fatalf("EnsureProjectMembership returned error: %v", err)
	}

	token = authenticateTestUser(t, "readable-user", false)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Get("/api/v1/projects/:id/memberships", getProjectMemberships)

	req = httptest.NewRequest("GET", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("membership route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode memberships: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected one membership, got %d", len(body))
	}
	if subject, ok = body[0]["subject"].(map[string]any); !ok {
		t.Fatalf("expected subject payload, got %#v", body[0])
	}
	if subject["label"] != "Readable User" {
		t.Fatalf("expected readable label, got %#v", subject["label"])
	}
}

func TestProjectManagerCanAddProjectMembershipByRef(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		project  *db.Project
		manager  *db.User
		target   *db.User
		token    string
		fiberApp *fiber.App
		req      *http.Request
		resp     *http.Response
		created  map[string]any
		err      error
	)

	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        "Scoped Access",
		ProjectType: db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if manager, _, err = db.EnsureUser("project-manager", "Project Manager", "manager@example.test", "local", "project-manager"); err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	if target, _, err = db.EnsureUser("project-target", "Project Target", "target@example.test", "local", "project-target"); err != nil {
		t.Fatalf("EnsureUser target returned error: %v", err)
	}
	if _, err = db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, manager.ID); err != nil {
		t.Fatalf("EnsureProjectMembership manager returned error: %v", err)
	}
	if err = db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, manager.ID, db.ProjectRoleManager); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole manager returned error: %v", err)
	}

	token = authenticateTestUser(t, "project-manager", false)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Post("/api/v1/projects/:id/memberships", postCreateProjectMembership)

	req = httptest.NewRequest("POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships", bytes.NewBufferString(`{
		"subjectType": "user",
		"subjectRef": "project-target",
		"projectRole": "developer"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("membership route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if err = json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created membership: %v", err)
	}
	if created["subject_type"] == nil || created["project_role"] == nil {
		t.Fatalf("expected created membership payload, got %#v", created)
	}

	assertProjectMemberRole(t, project.ID, db.ProjectMemberSubjectUser, target.ID, db.ProjectRoleDeveloper)
}

func TestProjectManagerCanAddProjectGroupByExternalRef(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		project  *db.Project
		manager  *db.User
		group    *db.CloudGroup
		token    string
		fiberApp *fiber.App
		req      *http.Request
		resp     *http.Response
		created  map[string]any
		subject  map[string]any
		ok       bool
		err      error
	)

	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        "Group Scoped Access",
		ProjectType: db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if manager, _, err = db.EnsureUser("project-group-manager", "Project Group Manager", "manager@example.test", "local", "project-group-manager"); err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	if group, _, err = db.EnsureCloudGroup("Teaching Staff", "teaching-staff", db.GroupTypeProject); err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	group.SyncSource = db.CloudGroupSyncSourceLDAP
	group.ExternalID = "ipa-teaching-staff"
	group.SyncMembership = true
	if err = db.UpdateCloudGroup(group); err != nil {
		t.Fatalf("UpdateCloudGroup returned error: %v", err)
	}
	if _, err = db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, manager.ID); err != nil {
		t.Fatalf("EnsureProjectMembership manager returned error: %v", err)
	}
	if err = db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, manager.ID, db.ProjectRoleManager); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole manager returned error: %v", err)
	}

	token = authenticateTestUser(t, "project-group-manager", false)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Post("/api/v1/projects/:id/memberships", postCreateProjectMembership)

	req = httptest.NewRequest("POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships", bytes.NewBufferString(`{
		"subjectType": "group",
		"subjectRef": "ipa-teaching-staff",
		"projectRole": "developer"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("membership route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if err = json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created membership: %v", err)
	}
	if subject, ok = created["subject"].(map[string]any); !ok {
		t.Fatalf("expected subject payload, got %#v", created)
	}
	if subject["slug"] != "teaching-staff" {
		t.Fatalf("expected group subject payload, got %#v", subject)
	}

	assertProjectMemberRole(t, project.ID, db.ProjectMemberSubjectGroup, group.ID, db.ProjectRoleDeveloper)
}

func TestProjectManagerCanPatchProjectMembershipRole(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		project          *db.Project
		manager          *db.User
		target           *db.User
		targetMembership *db.ProjectMembership
		memberships      []*db.ProjectMembership
		token            string
		fiberApp         *fiber.App
		ownerReq         *http.Request
		ownerResp        *http.Response
		req              *http.Request
		resp             *http.Response
		err              error
	)

	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        "Patch Membership",
		ProjectType: db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if manager, _, err = db.EnsureUser("project-role-manager", "Project Role Manager", "manager@example.test", "local", "project-role-manager"); err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	if target, _, err = db.EnsureUser("project-role-target", "Project Role Target", "target@example.test", "local", "project-role-target"); err != nil {
		t.Fatalf("EnsureUser target returned error: %v", err)
	}
	if _, err = db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, manager.ID); err != nil {
		t.Fatalf("EnsureProjectMembership manager returned error: %v", err)
	}
	if err = db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, manager.ID, db.ProjectRoleManager); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole manager returned error: %v", err)
	}
	if _, err = db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, target.ID); err != nil {
		t.Fatalf("EnsureProjectMembership target returned error: %v", err)
	}
	if err = db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, target.ID, db.ProjectRoleViewer); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole target returned error: %v", err)
	}

	if memberships, err = db.ProjectMembershipsForProject(project.ID); err != nil {
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

	token = authenticateTestUser(t, "project-role-manager", false)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Patch("/api/v1/projects/:id/memberships/:membershipID", patchProjectMembership)

	ownerReq = httptest.NewRequest("PATCH", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships/"+strconv.Itoa(targetMembership.ID), bytes.NewBufferString(`{
		"projectRole": "owner"
	}`))
	ownerReq.Header.Set("Authorization", "Bearer "+token)
	ownerReq.Header.Set("Content-Type", "application/json")

	if ownerResp, err = fiberApp.Test(ownerReq); err != nil {
		t.Fatalf("patch owner membership route returned error: %v", err)
	}
	if ownerResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected owner promotion to be forbidden, got %d", ownerResp.StatusCode)
	}

	req = httptest.NewRequest("PATCH", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships/"+strconv.Itoa(targetMembership.ID), bytes.NewBufferString(`{
		"projectRole": "developer"
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("patch membership route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	assertProjectMemberRole(t, project.ID, db.ProjectMemberSubjectUser, target.ID, db.ProjectRoleDeveloper)
}

func TestProjectManagerCanCreateScopedRoleAndAssignOnlyContainedPermissions(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		project         *db.Project
		manager         *db.User
		target          *db.User
		token           string
		fiberApp        *fiber.App
		createRoleBody  string
		createRoleReq   *http.Request
		createRoleResp  *http.Response
		role            map[string]any
		roleID          int
		vmRead          *db.Permission
		userManage      *db.Permission
		grantReq        *http.Request
		grantResp       *http.Response
		deniedGrantReq  *http.Request
		deniedGrantResp *http.Response
		rolesReq        *http.Request
		rolesResp       *http.Response
		roles           []map[string]any
		foundScopedRole bool
		memberBody      string
		memberReq       *http.Request
		memberResp      *http.Response
		bindings        []*db.RoleBinding
		found           bool
		err             error
	)

	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        "Scoped Role Project",
		ProjectType: db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if manager, _, err = db.EnsureUser("scoped-project-manager", "Scoped Project Manager", "manager@example.test", "local", "scoped-project-manager"); err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	if target, _, err = db.EnsureUser("scoped-project-target", "Scoped Project Target", "target@example.test", "local", "scoped-project-target"); err != nil {
		t.Fatalf("EnsureUser target returned error: %v", err)
	}
	if _, err = db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, manager.ID); err != nil {
		t.Fatalf("EnsureProjectMembership manager returned error: %v", err)
	}
	if err = db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, manager.ID, db.ProjectRoleManager); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole manager returned error: %v", err)
	}

	token = authenticateTestUser(t, manager.Username, false)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Post("/api/v1/roles", postCreateRole)
	fiberApp.Post("/api/v1/roles/:id/permissions", postCreateRolePermission)
	fiberApp.Get("/api/v1/projects/:id/roles", getProjectAssignableRoles)
	fiberApp.Post("/api/v1/projects/:id/memberships", postCreateProjectMembership)

	createRoleBody = `{"name":"Scoped VM Reader","description":"Read only inside project","scopeType":"project","scopeID":` + strconv.Itoa(project.ID) + `}`
	createRoleReq = httptest.NewRequest("POST", "/api/v1/roles", bytes.NewBufferString(createRoleBody))
	createRoleReq.Header.Set("Authorization", "Bearer "+token)
	createRoleReq.Header.Set("Content-Type", "application/json")
	if createRoleResp, err = fiberApp.Test(createRoleReq); err != nil {
		t.Fatalf("create scoped role route returned error: %v", err)
	}
	if createRoleResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createRoleResp.StatusCode)
	}
	if err = json.NewDecoder(createRoleResp.Body).Decode(&role); err != nil {
		t.Fatalf("decode scoped role: %v", err)
	}
	roleID = int(role["id"].(float64))

	if vmRead, found, err = db.GetPermissionByName(db.PermissionVMRead.String()); err != nil || !found {
		t.Fatalf("expected vm.read permission, found=%v err=%v", found, err)
	}
	grantReq = httptest.NewRequest("POST", "/api/v1/roles/"+strconv.Itoa(roleID)+"/permissions", bytes.NewBufferString(`{"permissionID":`+strconv.Itoa(vmRead.ID)+`}`))
	grantReq.Header.Set("Authorization", "Bearer "+token)
	grantReq.Header.Set("Content-Type", "application/json")
	if grantResp, err = fiberApp.Test(grantReq); err != nil {
		t.Fatalf("grant scoped permission route returned error: %v", err)
	}
	if grantResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected vm.read grant to succeed, got %d", grantResp.StatusCode)
	}

	if userManage, found, err = db.GetPermissionByName(db.PermissionUserManage.String()); err != nil || !found {
		t.Fatalf("expected user.manage permission, found=%v err=%v", found, err)
	}
	deniedGrantReq = httptest.NewRequest("POST", "/api/v1/roles/"+strconv.Itoa(roleID)+"/permissions", bytes.NewBufferString(`{"permissionID":`+strconv.Itoa(userManage.ID)+`}`))
	deniedGrantReq.Header.Set("Authorization", "Bearer "+token)
	deniedGrantReq.Header.Set("Content-Type", "application/json")
	if deniedGrantResp, err = fiberApp.Test(deniedGrantReq); err != nil {
		t.Fatalf("grant denied scoped permission route returned error: %v", err)
	}
	if deniedGrantResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected user.manage grant to be forbidden, got %d", deniedGrantResp.StatusCode)
	}

	rolesReq = httptest.NewRequest("GET", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/roles", nil)
	rolesReq.Header.Set("Authorization", "Bearer "+token)
	if rolesResp, err = fiberApp.Test(rolesReq); err != nil {
		t.Fatalf("project roles route returned error: %v", err)
	}
	if rolesResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", rolesResp.StatusCode)
	}
	if err = json.NewDecoder(rolesResp.Body).Decode(&roles); err != nil {
		t.Fatalf("decode assignable roles: %v", err)
	}
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

	memberBody = `{"subjectType":"user","subjectRef":"` + target.Username + `","roleID":` + strconv.Itoa(roleID) + `}`
	memberReq = httptest.NewRequest("POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/memberships", bytes.NewBufferString(memberBody))
	memberReq.Header.Set("Authorization", "Bearer "+token)
	memberReq.Header.Set("Content-Type", "application/json")
	if memberResp, err = fiberApp.Test(memberReq); err != nil {
		t.Fatalf("project member route returned error: %v", err)
	}
	if memberResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", memberResp.StatusCode)
	}

	if bindings, err = db.RoleBindingsForSubject(db.RoleBindingSubjectUser, target.ID); err != nil {
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
	ensureInitialSetupForTest(t)

	var (
		project  *db.Project
		token    string
		fiberApp *fiber.App
		req      *http.Request
		resp     *http.Response
		found    bool
		err      error
	)

	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        "Delete Me",
		ProjectType: db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	token = authenticateTestUser(t, "delete-admin", true)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Delete("/api/v1/projects/:slug", deleteProjectBySlug)

	req = httptest.NewRequest("DELETE", "/api/v1/projects/"+project.Slug, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("delete route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	if _, found, err = db.GetProjectBySlug(project.Slug); err != nil || found {
		t.Fatalf("expected project to be deleted, found=%v err=%v", found, err)
	}
}

func TestProjectAPIDeleteAllowsOrgScopedProjectManager(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		root       *db.Organization
		child      *db.Organization
		project    *db.Project
		user       *db.User
		role       *db.Role
		permission *db.Permission
		token      string
		fiberApp   *fiber.App
		req        *http.Request
		resp       *http.Response
		found      bool
		err        error
	)

	if root, found, err = db.GetOrganizationBySlug(db.DefaultRootOrganizationSlug); err != nil || !found {
		t.Fatalf("expected default root organization, found=%v err=%v", found, err)
	}
	if child, err = db.CreateOrganization(db.OrganizationCreateInput{
		Name:        "Scoped Delete",
		Slug:        "scoped-delete",
		ParentOrgID: &root.ID,
	}); err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}
	if project, err = db.CreateProject(db.ProjectCreateInput{
		Name:           "Org Scoped Delete",
		ProjectType:    db.ProjectTypeCustom,
		OrganizationID: child.ID,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if user, _, err = db.EnsureUser("org-project-manager", "Org Project Manager", "org-project-manager@example.test", "local", "org-project-manager"); err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if role, _, err = db.EnsureRole("Scoped Project Manager", "Can manage projects below an org", false); err != nil {
		t.Fatalf("EnsureRole returned error: %v", err)
	}
	if permission, found, err = db.GetPermissionByName(db.PermissionProjectManage.String()); err != nil || !found {
		t.Fatalf("expected project.manage permission, found=%v err=%v", found, err)
	}
	if _, err = db.EnsureRolePermission(role.ID, permission.ID); err != nil {
		t.Fatalf("EnsureRolePermission returned error: %v", err)
	}
	if _, err = db.EnsureRoleBinding(role.ID, db.RoleBindingSubjectUser, user.ID, db.RoleBindingScopeOrg, &root.ID); err != nil {
		t.Fatalf("EnsureRoleBinding returned error: %v", err)
	}

	token = authenticateTestUser(t, user.Username, false)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Delete("/api/v1/projects/:slug", deleteProjectBySlug)

	req = httptest.NewRequest("DELETE", "/api/v1/projects/"+project.Slug, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("delete route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestOrganizationAPIDeleteArchivesEmptyOrganization(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		org      *db.Organization
		archived *db.Organization
		token    string
		fiberApp *fiber.App
		req      *http.Request
		resp     *http.Response
		treeReq  *http.Request
		treeResp *http.Response
		tree     map[string][]map[string]any
		found    bool
		err      error
	)

	if org, err = db.CreateOrganization(db.OrganizationCreateInput{
		Name:        "Archive Org",
		Slug:        "archive-org",
		ParentOrgID: rootOrgIDPtr(t),
	}); err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}

	token = authenticateTestUser(t, "org-archive-admin", true)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Delete("/api/v1/organizations/:id", deleteOrganization)
	fiberApp.Get("/api/v1/projects/tree", getProjectTree)

	req = httptest.NewRequest("DELETE", "/api/v1/organizations/"+strconv.Itoa(org.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("delete organization route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	if archived, found, err = db.GetOrganizationByID(org.ID); err != nil || !found {
		t.Fatalf("expected archived organization to remain, found=%v err=%v", found, err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("expected organization to be archived")
	}

	treeReq = httptest.NewRequest("GET", "/api/v1/projects/tree", nil)
	treeReq.Header.Set("Authorization", "Bearer "+token)
	if treeResp, err = fiberApp.Test(treeReq); err != nil {
		t.Fatalf("project tree route returned error: %v", err)
	}
	if treeResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", treeResp.StatusCode)
	}
	if err = json.NewDecoder(treeResp.Body).Decode(&tree); err != nil {
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
	ensureInitialSetupForTest(t)

	var (
		child      *db.Organization
		token      string
		fiberApp   *fiber.App
		createReq  *http.Request
		createResp *http.Response
		moveReq    *http.Request
		moveResp   *http.Response
		err        error
	)

	if child, err = db.CreateOrganization(db.OrganizationCreateInput{
		Name:        "Child Org",
		Slug:        "child-org",
		ParentOrgID: rootOrgIDPtr(t),
	}); err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}

	token = authenticateTestUser(t, "root-guard-admin", true)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Post("/api/v1/organizations", postCreateOrganization)
	fiberApp.Patch("/api/v1/organizations/:id", patchOrganization)

	createReq = httptest.NewRequest("POST", "/api/v1/organizations", bytes.NewBufferString(`{"name":"Second Root","slug":"second-root"}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	if createResp, err = fiberApp.Test(createReq); err != nil {
		t.Fatalf("create organization route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected second root create to return 400, got %d", createResp.StatusCode)
	}

	moveReq = httptest.NewRequest("PATCH", "/api/v1/organizations/"+strconv.Itoa(child.ID), bytes.NewBufferString(`{"parentOrgID":null}`))
	moveReq.Header.Set("Authorization", "Bearer "+token)
	moveReq.Header.Set("Content-Type", "application/json")
	if moveResp, err = fiberApp.Test(moveReq); err != nil {
		t.Fatalf("patch organization route returned error: %v", err)
	}
	if moveResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected move to root to return 400, got %d", moveResp.StatusCode)
	}
}

func TestOrganizationAPIDeleteStillRejectsOrganizationsWithProjects(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)

	var (
		org         *db.Organization
		stillActive *db.Organization
		token       string
		fiberApp    *fiber.App
		req         *http.Request
		resp        *http.Response
		found       bool
		err         error
	)

	if org, err = db.CreateOrganization(db.OrganizationCreateInput{
		Name:        "Busy Org",
		Slug:        "busy-org",
		ParentOrgID: rootOrgIDPtr(t),
	}); err != nil {
		t.Fatalf("CreateOrganization returned error: %v", err)
	}
	if _, err = db.CreateProject(db.ProjectCreateInput{
		Name:           "Busy Project",
		OrganizationID: org.ID,
		ProjectType:    db.ProjectTypeCustom,
	}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}

	token = authenticateTestUser(t, "busy-org-admin", true)
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Delete("/api/v1/organizations/:id", deleteOrganization)

	req = httptest.NewRequest("DELETE", "/api/v1/organizations/"+strconv.Itoa(org.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if resp, err = fiberApp.Test(req); err != nil {
		t.Fatalf("delete busy organization route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if stillActive, found, err = db.GetOrganizationByID(org.ID); err != nil || !found {
		t.Fatalf("expected organization to remain, found=%v err=%v", found, err)
	}
	if stillActive.ArchivedAt != nil {
		t.Fatal("expected organization with projects not to be archived")
	}
}

func assertProjectMemberRole(t *testing.T, projectID int, subjectType db.ProjectMemberSubject, subjectID int, expected db.ProjectRole) {
	t.Helper()

	var (
		role  db.ProjectRole
		found bool
		err   error
	)

	if role, found, err = db.ProjectMemberAccessRole(projectID, subjectType, subjectID); err != nil || !found || role != expected {
		t.Fatalf("expected project member role %v, got role=%v found=%v err=%v", expected, role, found, err)
	}
}

func rootOrgID(t *testing.T) (countResult int) {
	t.Helper()

	var (
		root  *db.Organization
		found bool
		err   error
	)

	if root, found, err = db.GetOrganizationBySlug(db.DefaultRootOrganizationSlug); err != nil || !found {
		t.Fatalf("expected default root organization, found=%v err=%v", found, err)
	}
	return root.ID
}

func rootOrgIDPtr(t *testing.T) (intResult *int) {
	t.Helper()

	var id int = rootOrgID(t)
	return &id
}

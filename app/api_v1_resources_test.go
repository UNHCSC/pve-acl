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

func TestProjectManagerCanManageResourceWorkflow(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var (
		project  *db.Project
		manager  *db.User
		token    string
		fiberApp *fiber.App
		err      error
	)

	project = createResourceAPIProject(t, "Managed Resources")
	manager = ensureResourceAPIUser(t, "resource-manager")
	grantProjectRole(t, project.ID, manager.ID, db.ProjectRoleManager)
	token = authenticateTestUser(t, manager.Username, false)
	fiberApp = newResourceAPITestApp()

	var resource map[string]any

	resource = createResourceAPIResource(t, fiberApp, token, project.ID, `{"name":"Student VM","slug":"student-vm","resourceType":"vm"}`)
	if resource["status_label"] != "ready" || resource["resource_type_label"] != "vm" {
		t.Fatalf("expected ready vm resource, got %#v", resource)
	}
	var resourceID int

	resourceID = int(resource["id"].(float64))
	var resp *http.Response

	resp = resourceAPIRequest(t, fiberApp, token, "GET", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/resources", "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected list 200, got %d", resp.StatusCode)
	}
	var resources []map[string]any

	if err = json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		t.Fatalf("decode resources: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected one resource, got %#v", resources)
	}
	resp = resourceAPIRequest(t, fiberApp, token, "PATCH", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/resources/"+strconv.Itoa(resourceID), `{"name":"Student VM Updated","slug":"student-vm-updated","status":"unknown"}`)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected update 200, got %d", resp.StatusCode)
	}
	resp = resourceAPIRequest(t, fiberApp, token, "DELETE", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/resources/"+strconv.Itoa(resourceID), "")
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected delete 204, got %d", resp.StatusCode)
	}
	var found bool

	_, found, err = db.GetResourceByID(resourceID)
	if err != nil || found {
		t.Fatalf("expected resource to be archived, found=%v err=%v", found, err)
	}
}

func TestProjectViewerCannotMutateResourceWorkflow(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var project *db.Project

	project = createResourceAPIProject(t, "Viewer Resources")
	var viewer *db.User

	viewer = ensureResourceAPIUser(t, "resource-viewer")
	grantProjectRole(t, project.ID, viewer.ID, db.ProjectRoleViewer)
	var token string

	token = authenticateTestUser(t, viewer.Username, false)
	var fiberApp *fiber.App

	fiberApp = newResourceAPITestApp()
	var resp *http.Response

	resp = resourceAPIRequest(t, fiberApp, token, "POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/resources", `{"name":"Denied VM","resourceType":"vm"}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected viewer resource create 403, got %d", resp.StatusCode)
	}
	resp = resourceAPIRequest(t, fiberApp, token, "POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/asset-assignments", `{"targetType":"resource","resourceID":1,"subjectType":"user","subjectRef":"nobody","roleID":1}`)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected viewer assignment create 403, got %d", resp.StatusCode)
	}
}

func TestProjectDeveloperCanCreateVMButCannotAssignRoles(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var project *db.Project

	project = createResourceAPIProject(t, "Developer Resources")
	var developer *db.User

	developer = ensureResourceAPIUser(t, "resource-developer")
	grantProjectRole(t, project.ID, developer.ID, db.ProjectRoleDeveloper)
	var token string

	token = authenticateTestUser(t, developer.Username, false)
	var fiberApp *fiber.App

	fiberApp = newResourceAPITestApp()
	var resource map[string]any

	resource = createResourceAPIResource(t, fiberApp, token, project.ID, `{"name":"Dev VM","resourceType":"vm"}`)
	if resource["resource_type_label"] != "vm" {
		t.Fatalf("expected developer-created vm, got %#v", resource)
	}
	var role *db.Role

	role = createResourceAPIRole(t, "Resource Console", project.ID, db.PermissionVMRead, db.PermissionVMConsole)
	var target *db.User

	target = ensureResourceAPIUser(t, "assigned-developer-target")
	var body string

	body = `{"targetType":"resource","resourceID":` + strconv.Itoa(int(resource["id"].(float64))) + `,"subjectType":"user","subjectRef":"` + target.Username + `","roleID":` + strconv.Itoa(role.ID) + `}`
	var resp *http.Response

	resp = resourceAPIRequest(t, fiberApp, token, "POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/asset-assignments", body)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected developer assignment create 403, got %d", resp.StatusCode)
	}
}

func TestResourceAssignmentGrantsOnlyAssignedSubject(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var project *db.Project

	project = createResourceAPIProject(t, "Assigned Resources")
	var manager *db.User

	manager = ensureResourceAPIUser(t, "assignment-manager")
	grantProjectRole(t, project.ID, manager.ID, db.ProjectRoleManager)
	var target *db.User

	target = ensureResourceAPIUser(t, "assignment-target")
	var outsider *db.User

	outsider = ensureResourceAPIUser(t, "assignment-outsider")
	var role *db.Role

	role = createResourceAPIRole(t, "Assigned VM User", project.ID, db.PermissionVMRead, db.PermissionVMConsole)
	var token string

	token = authenticateTestUser(t, manager.Username, false)
	var fiberApp *fiber.App

	fiberApp = newResourceAPITestApp()
	var resource map[string]any

	resource = createResourceAPIResource(t, fiberApp, token, project.ID, `{"name":"Assigned VM","resourceType":"vm"}`)
	var resourceID int

	resourceID = int(resource["id"].(float64))
	var body string

	body = `{"targetType":"resource","resourceID":` + strconv.Itoa(resourceID) + `,"subjectType":"user","subjectRef":"` + target.Username + `","roleID":` + strconv.Itoa(role.ID) + `}`
	var resp *http.Response

	resp = resourceAPIRequest(t, fiberApp, token, "POST", "/api/v1/projects/"+strconv.Itoa(project.ID)+"/asset-assignments", body)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected assignment 201, got %d", resp.StatusCode)
	}
	assertResourceAPIPermission(t, target.ID, db.PermissionVMRead, resourceID, true)
	assertResourceAPIPermission(t, target.ID, db.PermissionVMConsole, resourceID, true)
	assertResourceAPIPermission(t, outsider.ID, db.PermissionVMRead, resourceID, false)
	assertResourceAPIPermission(t, outsider.ID, db.PermissionVMConsole, resourceID, false)
}

func TestProjectOwnedRoleCannotBeAssignedOutsideProjectResource(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var firstProject *db.Project

	firstProject = createResourceAPIProject(t, "First Role Scope")
	var secondProject *db.Project

	secondProject = createResourceAPIProject(t, "Second Role Scope")
	var manager *db.User

	manager = ensureResourceAPIUser(t, "cross-scope-manager")
	grantProjectRole(t, firstProject.ID, manager.ID, db.ProjectRoleManager)
	grantProjectRole(t, secondProject.ID, manager.ID, db.ProjectRoleManager)
	var token string

	token = authenticateTestUser(t, manager.Username, false)
	var fiberApp *fiber.App

	fiberApp = newResourceAPITestApp()
	var firstResource map[string]any

	firstResource = createResourceAPIResource(t, fiberApp, token, firstProject.ID, `{"name":"First VM","resourceType":"vm"}`)
	var secondResource map[string]any

	secondResource = createResourceAPIResource(t, fiberApp, token, secondProject.ID, `{"name":"Second VM","resourceType":"vm"}`)
	var role *db.Role

	role = createResourceAPIRole(t, "First Project VM User", firstProject.ID, db.PermissionVMRead, db.PermissionVMConsole)
	var target *db.User

	target = ensureResourceAPIUser(t, "cross-scope-target")
	var body string

	body = `{"targetType":"resource","resourceID":` + strconv.Itoa(int(firstResource["id"].(float64))) + `,"subjectType":"user","subjectRef":"` + target.Username + `","roleID":` + strconv.Itoa(role.ID) + `}`
	var resp *http.Response

	resp = resourceAPIRequest(t, fiberApp, token, "POST", "/api/v1/projects/"+strconv.Itoa(firstProject.ID)+"/asset-assignments", body)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected in-project assignment 201, got %d", resp.StatusCode)
	}
	body = `{"targetType":"resource","resourceID":` + strconv.Itoa(int(secondResource["id"].(float64))) + `,"subjectType":"user","subjectRef":"` + target.Username + `","roleID":` + strconv.Itoa(role.ID) + `}`
	resp = resourceAPIRequest(t, fiberApp, token, "POST", "/api/v1/projects/"+strconv.Itoa(secondProject.ID)+"/asset-assignments", body)
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected cross-project role assignment 403, got %d", resp.StatusCode)
	}
}

func newResourceAPITestApp() (fiberApp *fiber.App) {
	fiberApp = newAuthenticatedFiberApp()
	fiberApp.Get("/api/v1/projects/:id/resources", getProjectResources)
	fiberApp.Post("/api/v1/projects/:id/resources", postCreateProjectResource)
	fiberApp.Patch("/api/v1/projects/:id/resources/:resourceID", patchProjectResource)
	fiberApp.Delete("/api/v1/projects/:id/resources/:resourceID", deleteProjectResource)
	fiberApp.Get("/api/v1/projects/:id/asset-groups", getProjectAssetGroups)
	fiberApp.Post("/api/v1/projects/:id/asset-groups", postCreateProjectAssetGroup)
	fiberApp.Post("/api/v1/projects/:id/asset-groups/:groupID/resources/:resourceID", postProjectAssetGroupResource)
	fiberApp.Delete("/api/v1/projects/:id/asset-groups/:groupID/resources/:resourceID", deleteProjectAssetGroupResource)
	fiberApp.Get("/api/v1/projects/:id/asset-assignments", getProjectAssetAssignments)
	fiberApp.Post("/api/v1/projects/:id/asset-assignments", postCreateProjectAssetAssignment)
	fiberApp.Delete("/api/v1/projects/:id/asset-assignments/:assignmentID", deleteProjectAssetAssignment)
	return
}

func resourceAPIRequest(t *testing.T, fiberApp *fiber.App, token string, method string, path string, body string) (responseResult *http.Response) {
	t.Helper()
	var req *http.Request

	req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	var (
		resp *http.Response
		err  error
	)

	resp, err = testFiberRequest(fiberApp, req)
	if err != nil {
		t.Fatalf("%s %s returned error: %v", method, path, err)
	}
	return resp
}

func createResourceAPIResource(t *testing.T, fiberApp *fiber.App, token string, projectID int, body string) (mapResult map[string]any) {
	t.Helper()
	var resp *http.Response

	resp = resourceAPIRequest(t, fiberApp, token, "POST", "/api/v1/projects/"+strconv.Itoa(projectID)+"/resources", body)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected resource create 201, got %d", resp.StatusCode)
	}
	var resource map[string]any

	var err error

	if err = json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		t.Fatalf("decode resource: %v", err)
	}
	return resource
}

func createResourceAPIProject(t *testing.T, name string) (projectResult *db.Project) {
	t.Helper()
	var (
		project *db.Project
		err     error
	)

	project, err = db.CreateProject(db.ProjectCreateInput{
		Name:        name,
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	return project
}

func ensureResourceAPIUser(t *testing.T, username string) (userResult *db.User) {
	t.Helper()
	var (
		user *db.User
		err  error
	)

	user, _, err = db.EnsureUser(username, username, username+"@example.test", "local", username)
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	return user
}

func grantProjectRole(t *testing.T, projectID int, userID int, role db.ProjectRole) {
	t.Helper()
	var err error

	if _, err = db.EnsureProjectMembership(projectID, db.ProjectMemberSubjectUser, userID); err != nil {
		t.Fatalf("EnsureProjectMembership returned error: %v", err)
	}
	if err = db.EnsureProjectMemberAccessRole(projectID, db.ProjectMemberSubjectUser, userID, role); err != nil {
		t.Fatalf("EnsureProjectMemberAccessRole returned error: %v", err)
	}
}

func createResourceAPIRole(t *testing.T, name string, projectID int, permissions ...db.PermissionKey) (roleResult *db.Role) {
	t.Helper()
	var (
		role *db.Role
		err  error
	)

	role, err = db.CreateRole(db.RoleCreateInput{
		Name:           name,
		Description:    name,
		OwnerScopeType: db.RoleBindingScopeProject,
		OwnerScopeID:   &projectID,
	})
	if err != nil {
		t.Fatalf("CreateRole returned error: %v", err)
	}
	for _, permissionKey := range permissions {
		var (
			permission *db.Permission
			found      bool
		)
		permission, found, err = db.GetPermissionByName(permissionKey.String())
		if err != nil || !found {
			t.Fatalf("GetPermissionByName returned found=%v err=%v", found, err)
		}
		if _, err = db.EnsureRolePermission(role.ID, permission.ID); err != nil {
			t.Fatalf("EnsureRolePermission returned error: %v", err)
		}
	}
	return role
}

func assertResourceAPIPermission(t *testing.T, userID int, permission db.PermissionKey, resourceID int, expected bool) {
	t.Helper()
	var (
		allowed bool
		err     error
	)

	allowed, err = db.HasPermission(db.PermissionCheck{
		UserID:     userID,
		Permission: permission,
		ScopeType:  db.RoleBindingScopeResource,
		ScopeID:    &resourceID,
	})
	if err != nil {
		t.Fatalf("HasPermission returned error: %v", err)
	}
	if allowed != expected {
		t.Fatalf("expected allowed=%v for %s on resource %d, got %v", expected, permission.String(), resourceID, allowed)
	}
}

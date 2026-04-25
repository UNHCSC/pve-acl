package app

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/UNHCSC/proxman/db"
	"github.com/gofiber/fiber/v2"
)

func TestAdminCanCreateAndListCloudGroups(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	token := authenticateTestUser(t, "group-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Get("/api/v1/groups", getCloudGroups)
	fiberApp.Post("/api/v1/groups", postCreateCloudGroup)

	createReq := httptest.NewRequest("POST", "/api/v1/groups", bytes.NewBufferString(`{
		"name": "Operators",
		"slug": "operators",
		"description": "Project operators"
	}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := fiberApp.Test(createReq)
	if err != nil {
		t.Fatalf("create group route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	listReq := httptest.NewRequest("GET", "/api/v1/groups", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)

	listResp, err := fiberApp.Test(listReq)
	if err != nil {
		t.Fatalf("list group route returned error: %v", err)
	}
	if listResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var groups []db.CloudGroup
	if err := json.NewDecoder(listResp.Body).Decode(&groups); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	for _, group := range groups {
		if group.Slug == "operators" {
			return
		}
	}
	t.Fatalf("expected groups to include operators, got %#v", groups)
}

func TestRoleBoundAdminCanCreateCloudGroups(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	dbUser, _, err := db.EnsureUser("role-admin", "Role Admin", "role-admin@example.test", "local", "role-admin")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}

	adminGroup := findTestGroupBySlug(t, db.DefaultAdminGroupSlug)
	if _, err := db.EnsureCloudGroupMembership(dbUser.ID, adminGroup.ID, db.MembershipRoleMember); err != nil {
		t.Fatalf("EnsureCloudGroupMembership returned error: %v", err)
	}

	token := authenticateTestUser(t, "role-admin", false)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/groups", postCreateCloudGroup)

	createReq := httptest.NewRequest("POST", "/api/v1/groups", bytes.NewBufferString(`{
		"name": "Course Staff",
		"slug": "course-staff"
	}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := fiberApp.Test(createReq)
	if err != nil {
		t.Fatalf("create group route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}
}

func findTestGroupBySlug(t *testing.T, slug string) *db.CloudGroup {
	t.Helper()

	groups, err := db.ListCloudGroups()
	if err != nil {
		t.Fatalf("ListCloudGroups returned error: %v", err)
	}
	for _, group := range groups {
		if group.Slug == slug {
			return group
		}
	}
	t.Fatalf("expected group %q", slug)
	return nil
}

func TestGroupManagerCanManageMembershipRoles(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	group, _, err := db.EnsureCloudGroup("Project Team", "project-team", db.GroupTypeProject)
	if err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	manager, _, err := db.EnsureUser("group-manager", "Group Manager", "manager@example.test", "local", "group-manager")
	if err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	target, _, err := db.EnsureUser("group-target", "Group Target", "target@example.test", "local", "group-target")
	if err != nil {
		t.Fatalf("EnsureUser target returned error: %v", err)
	}
	if _, err := db.EnsureCloudGroupMembership(manager.ID, group.ID, db.MembershipRoleManager); err != nil {
		t.Fatalf("EnsureCloudGroupMembership manager returned error: %v", err)
	}

	token := authenticateTestUser(t, "group-manager", false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/groups/:id/memberships", postCreateGroupMembership)

	req := httptest.NewRequest("POST", "/api/v1/groups/"+strconv.Itoa(group.ID)+"/memberships", bytes.NewBufferString(`{
		"userRef": "group-target",
		"membershipRole": "owner"
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

	membership, found, err := db.CloudGroupMembershipForUserAndGroup(target.ID, group.ID)
	if err != nil || !found {
		t.Fatalf("expected target membership, found=%v err=%v", found, err)
	}
	if membership.MembershipRole != db.MembershipRoleOwner {
		t.Fatalf("expected owner membership, got %#v", membership.MembershipRole)
	}
}

func TestGroupManagerCanPatchMembershipRole(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	group, _, err := db.EnsureCloudGroup("Role Patch Group", "role-patch-group", db.GroupTypeProject)
	if err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	manager, _, err := db.EnsureUser("group-role-manager", "Group Role Manager", "manager@example.test", "local", "group-role-manager")
	if err != nil {
		t.Fatalf("EnsureUser manager returned error: %v", err)
	}
	target, _, err := db.EnsureUser("group-role-target", "Group Role Target", "target@example.test", "local", "group-role-target")
	if err != nil {
		t.Fatalf("EnsureUser target returned error: %v", err)
	}
	if _, err := db.EnsureCloudGroupMembership(manager.ID, group.ID, db.MembershipRoleManager); err != nil {
		t.Fatalf("EnsureCloudGroupMembership manager returned error: %v", err)
	}
	if _, err := db.EnsureCloudGroupMembership(target.ID, group.ID, db.MembershipRoleMember); err != nil {
		t.Fatalf("EnsureCloudGroupMembership target returned error: %v", err)
	}
	membership, found, err := db.CloudGroupMembershipForUserAndGroup(target.ID, group.ID)
	if err != nil || !found {
		t.Fatalf("expected target membership, found=%v err=%v", found, err)
	}

	token := authenticateTestUser(t, "group-role-manager", false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Patch("/api/v1/groups/:id/memberships/:membershipID", patchGroupMembership)

	req := httptest.NewRequest("PATCH", "/api/v1/groups/"+strconv.Itoa(group.ID)+"/memberships/"+strconv.Itoa(membership.ID), bytes.NewBufferString(`{
		"membershipRole": "owner"
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

	updated, found, err := db.CloudGroupMembershipForUserAndGroup(target.ID, group.ID)
	if err != nil || !found {
		t.Fatalf("expected updated membership, found=%v err=%v", found, err)
	}
	if updated.MembershipRole != db.MembershipRoleOwner {
		t.Fatalf("expected owner role, got %#v", updated.MembershipRole)
	}
}

func TestAdminCanCreateCustomRoleAndGrantPermission(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	token := authenticateTestUser(t, "role-catalog-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/roles", postCreateRole)
	fiberApp.Post("/api/v1/roles/:id/permissions", postCreateRolePermission)
	fiberApp.Get("/api/v1/roles/:id/permissions", getRolePermissions)

	createReq := httptest.NewRequest("POST", "/api/v1/roles", bytes.NewBufferString(`{
		"name": "ProjectAuditor",
		"description": "Can inspect project resources"
	}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := fiberApp.Test(createReq)
	if err != nil {
		t.Fatalf("create role route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	var role map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&role); err != nil {
		t.Fatalf("decode role response: %v", err)
	}
	roleID := int(role["id"].(float64))

	grantReq := httptest.NewRequest("POST", "/api/v1/roles/"+strconv.Itoa(roleID)+"/permissions", bytes.NewBufferString(`{
		"permissionName": "vm.read"
	}`))
	grantReq.Header.Set("Authorization", "Bearer "+token)
	grantReq.Header.Set("Content-Type", "application/json")

	grantResp, err := fiberApp.Test(grantReq)
	if err != nil {
		t.Fatalf("grant permission route returned error: %v", err)
	}
	if grantResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", grantResp.StatusCode)
	}

	listReq := httptest.NewRequest("GET", "/api/v1/roles/"+strconv.Itoa(roleID)+"/permissions", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := fiberApp.Test(listReq)
	if err != nil {
		t.Fatalf("list permissions route returned error: %v", err)
	}
	if listResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var grants []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&grants); err != nil {
		t.Fatalf("decode grants response: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("expected one permission grant, got %#v", grants)
	}
	permission := grants[0]["permission"].(map[string]any)
	if permission["name"] != "vm.read" {
		t.Fatalf("expected vm.read permission, got %#v", permission)
	}
}

func TestAdminCanManageAccessGrants(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	group, _, err := db.EnsureCloudGroup("Grant Group", "grant-group", db.GroupTypeCustom)
	if err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	role := findTestRoleByName(t, db.DefaultLabAdminRoleName)
	token := authenticateTestUser(t, "grant-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/role-bindings", postCreateRoleBinding)
	fiberApp.Get("/api/v1/role-bindings", getRoleBindings)
	fiberApp.Delete("/api/v1/role-bindings/:bindingID", deleteRoleBinding)

	createBody := `{"roleID":` + strconv.Itoa(role.ID) + `,"subjectType":"group","subjectID":` + strconv.Itoa(group.ID) + `,"scopeType":"group","scopeID":` + strconv.Itoa(group.ID) + `}`
	createReq := httptest.NewRequest("POST", "/api/v1/role-bindings", bytes.NewBufferString(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := fiberApp.Test(createReq)
	if err != nil {
		t.Fatalf("create role binding route returned error: %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	listReq := httptest.NewRequest("GET", "/api/v1/role-bindings", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := fiberApp.Test(listReq)
	if err != nil {
		t.Fatalf("list role bindings route returned error: %v", err)
	}
	if listResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var grants []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&grants); err != nil {
		t.Fatalf("decode grants response: %v", err)
	}
	var grantID int
	for _, grant := range grants {
		subject, _ := grant["subject"].(map[string]any)
		if subject["slug"] == "grant-group" {
			grantID = int(grant["id"].(float64))
			break
		}
	}
	if grantID == 0 {
		t.Fatalf("expected readable grant for grant-group, got %#v", grants)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/v1/role-bindings/"+strconv.Itoa(grantID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteResp, err := fiberApp.Test(deleteReq)
	if err != nil {
		t.Fatalf("delete role binding route returned error: %v", err)
	}
	if deleteResp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteResp.StatusCode)
	}
}

func TestGroupRoleBindingRequiresScopedRoleManage(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	group, _, err := db.EnsureCloudGroup("Scoped Group", "scoped-group", db.GroupTypeProject)
	if err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	user, _, err := db.EnsureUser("scoped-role-admin", "Scoped Role Admin", "scoped@example.test", "local", "scoped-role-admin")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	role := findTestRoleByName(t, db.DefaultLabAdminRoleName)
	if _, err := db.EnsureRoleBinding(role.ID, db.RoleBindingSubjectUser, user.ID, db.RoleBindingScopeGroup, &group.ID); err != nil {
		t.Fatalf("EnsureRoleBinding returned error: %v", err)
	}

	token := authenticateTestUser(t, "scoped-role-admin", false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/groups/:id/role-bindings", postCreateGroupRoleBinding)

	globalReq := httptest.NewRequest("POST", "/api/v1/groups/"+strconv.Itoa(group.ID)+"/role-bindings", bytes.NewBufferString(`{
		"roleName": "LabAdmin",
		"scopeType": "global"
	}`))
	globalReq.Header.Set("Authorization", "Bearer "+token)
	globalReq.Header.Set("Content-Type", "application/json")
	globalResp, err := fiberApp.Test(globalReq)
	if err != nil {
		t.Fatalf("global role binding route returned error: %v", err)
	}
	if globalResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected global binding to be forbidden, got %d", globalResp.StatusCode)
	}

	scopedBody := `{"roleName":"LabAdmin","scopeType":"group","scopeID":` + strconv.Itoa(group.ID) + `}`
	scopedReq := httptest.NewRequest("POST", "/api/v1/groups/"+strconv.Itoa(group.ID)+"/role-bindings", bytes.NewBufferString(scopedBody))
	scopedReq.Header.Set("Authorization", "Bearer "+token)
	scopedReq.Header.Set("Content-Type", "application/json")
	scopedResp, err := fiberApp.Test(scopedReq)
	if err != nil {
		t.Fatalf("scoped role binding route returned error: %v", err)
	}
	if scopedResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected scoped binding to be created, got %d", scopedResp.StatusCode)
	}
}

func findTestRoleByName(t *testing.T, name string) *db.Role {
	t.Helper()

	roles, err := db.Roles.SelectAll()
	if err != nil {
		t.Fatalf("Roles.SelectAll returned error: %v", err)
	}
	for _, role := range roles {
		if role.Name == name {
			return role
		}
	}
	t.Fatalf("expected role %q", name)
	return nil
}

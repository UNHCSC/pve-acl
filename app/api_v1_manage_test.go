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
		"name": "Teaching Staff",
		"slug": "teaching-staff"
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

func TestAdminCanUpdateAndArchiveCloudGroup(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	group, _, err := db.EnsureCloudGroup("Archive Me", "archive-me", db.GroupTypeCustom)
	if err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	token := authenticateTestUser(t, "group-archive-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Patch("/api/v1/groups/:id", patchCloudGroup)
	fiberApp.Delete("/api/v1/groups/:id", deleteCloudGroup)
	fiberApp.Get("/api/v1/groups", getCloudGroups)

	patchReq := httptest.NewRequest("PATCH", "/api/v1/groups/"+strconv.Itoa(group.ID), bytes.NewBufferString(`{
		"name": "Updated Group",
		"description": "Updated description"
	}`))
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := fiberApp.Test(patchReq)
	if err != nil {
		t.Fatalf("patch group route returned error: %v", err)
	}
	if patchResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", patchResp.StatusCode)
	}
	updated, found, err := db.GetCloudGroupByID(group.ID)
	if err != nil || !found {
		t.Fatalf("expected updated group, found=%v err=%v", found, err)
	}
	if updated.Name != "Updated Group" || updated.Description != "Updated description" {
		t.Fatalf("group was not updated: %#v", updated)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/v1/groups/"+strconv.Itoa(group.ID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteResp, err := fiberApp.Test(deleteReq)
	if err != nil {
		t.Fatalf("delete group route returned error: %v", err)
	}
	if deleteResp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteResp.StatusCode)
	}
	archived, found, err := db.GetCloudGroupByID(group.ID)
	if err != nil || !found {
		t.Fatalf("expected archived group to remain, found=%v err=%v", found, err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("expected group to be archived")
	}

	listReq := httptest.NewRequest("GET", "/api/v1/groups", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listResp, err := fiberApp.Test(listReq)
	if err != nil {
		t.Fatalf("list groups route returned error: %v", err)
	}
	if listResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}
	var groups []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&groups); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	for _, item := range groups {
		if item["slug"] == "archive-me" {
			t.Fatalf("archived group should be omitted from list: %#v", groups)
		}
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
	var grant map[string]any
	if err := json.NewDecoder(grantResp.Body).Decode(&grant); err != nil {
		t.Fatalf("decode grant response: %v", err)
	}
	if grant["permission_id"] == nil || grant["permission"] == nil {
		t.Fatalf("expected role permission payload, got %#v", grant)
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
	if permission["name"] != db.PermissionVMRead.String() {
		t.Fatalf("expected vm.read permission, got %#v", permission)
	}
}

func TestAdminCanUpdateAndDeleteUnboundCustomRole(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	role, _, err := db.EnsureRole("TemporaryOperator", "Temporary role", false)
	if err != nil {
		t.Fatalf("EnsureRole returned error: %v", err)
	}
	token := authenticateTestUser(t, "role-edit-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Patch("/api/v1/roles/:id", patchRole)
	fiberApp.Delete("/api/v1/roles/:id", deleteRole)

	patchReq := httptest.NewRequest("PATCH", "/api/v1/roles/"+strconv.Itoa(role.ID), bytes.NewBufferString(`{
		"name": "TemporaryViewer",
		"description": "Updated temporary role"
	}`))
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := fiberApp.Test(patchReq)
	if err != nil {
		t.Fatalf("patch role route returned error: %v", err)
	}
	if patchResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", patchResp.StatusCode)
	}
	updated, found, err := db.GetRoleByName("TemporaryViewer")
	if err != nil || !found {
		t.Fatalf("expected updated role, found=%v err=%v", found, err)
	}
	if updated.Description != "Updated temporary role" {
		t.Fatalf("expected updated description, got %#v", updated)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/v1/roles/"+strconv.Itoa(updated.ID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteResp, err := fiberApp.Test(deleteReq)
	if err != nil {
		t.Fatalf("delete role route returned error: %v", err)
	}
	if deleteResp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteResp.StatusCode)
	}
	if deleted, err := db.Roles.Select(updated.ID); err != nil {
		t.Fatalf("select deleted role: %v", err)
	} else if deleted != nil {
		t.Fatalf("expected role to be deleted, got %#v", deleted)
	}
}

func TestSystemRoleEditAndDeleteAreProtected(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	role := findTestRoleByName(t, db.DefaultLabAdminRoleName)
	token := authenticateTestUser(t, "system-role-admin", true)

	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Patch("/api/v1/roles/:id", patchRole)
	fiberApp.Delete("/api/v1/roles/:id", deleteRole)

	patchReq := httptest.NewRequest("PATCH", "/api/v1/roles/"+strconv.Itoa(role.ID), bytes.NewBufferString(`{"description":"changed"}`))
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := fiberApp.Test(patchReq)
	if err != nil {
		t.Fatalf("patch system role route returned error: %v", err)
	}
	if patchResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", patchResp.StatusCode)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/v1/roles/"+strconv.Itoa(role.ID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteResp, err := fiberApp.Test(deleteReq)
	if err != nil {
		t.Fatalf("delete system role route returned error: %v", err)
	}
	if deleteResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", deleteResp.StatusCode)
	}
}

func TestBoundCustomRoleCannotBeDeleted(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	role, _, err := db.EnsureRole("StillInUse", "Bound role", false)
	if err != nil {
		t.Fatalf("EnsureRole returned error: %v", err)
	}
	user, _, err := db.EnsureUser("bound-role-user", "Bound Role User", "bound@example.test", "local", "bound-role-user")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if _, err := db.EnsureRoleBinding(role.ID, db.RoleBindingSubjectUser, user.ID, db.RoleBindingScopeGlobal, nil); err != nil {
		t.Fatalf("EnsureRoleBinding returned error: %v", err)
	}

	token := authenticateTestUser(t, "bound-role-admin", true)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Delete("/api/v1/roles/:id", deleteRole)

	req := httptest.NewRequest("DELETE", "/api/v1/roles/"+strconv.Itoa(role.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatalf("delete bound role route returned error: %v", err)
	}
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
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

func TestGroupOwnerCanManageOnlyOwnScopedRoleBindings(t *testing.T) {
	initACLTestDB(t)
	if err := db.EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}

	group, _, err := db.EnsureCloudGroup("Owner Scoped Group", "owner-scoped-group", db.GroupTypeProject)
	if err != nil {
		t.Fatalf("EnsureCloudGroup returned error: %v", err)
	}
	otherGroup, _, err := db.EnsureCloudGroup("Other Scoped Group", "other-scoped-group", db.GroupTypeProject)
	if err != nil {
		t.Fatalf("EnsureCloudGroup other returned error: %v", err)
	}
	owner, _, err := db.EnsureUser("group-owner", "Group Owner", "owner@example.test", "local", "group-owner")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if _, err := db.EnsureCloudGroupMembership(owner.ID, group.ID, db.MembershipRoleOwner); err != nil {
		t.Fatalf("EnsureCloudGroupMembership returned error: %v", err)
	}

	token := authenticateTestUser(t, "group-owner", false)
	fiberApp := fiber.New()
	fiberApp.Use(requireAPIAuth)
	fiberApp.Post("/api/v1/groups/:id/role-bindings", postCreateGroupRoleBinding)
	fiberApp.Delete("/api/v1/groups/:id/role-bindings/:bindingID", deleteGroupRoleBinding)

	globalReq := httptest.NewRequest("POST", "/api/v1/groups/"+strconv.Itoa(group.ID)+"/role-bindings", bytes.NewBufferString(`{
		"roleName": "ProjectViewer",
		"scopeType": "global"
	}`))
	globalReq.Header.Set("Authorization", "Bearer "+token)
	globalReq.Header.Set("Content-Type", "application/json")
	globalResp, err := fiberApp.Test(globalReq)
	if err != nil {
		t.Fatalf("global owner grant route returned error: %v", err)
	}
	if globalResp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected global binding to be forbidden, got %d", globalResp.StatusCode)
	}

	scopedBody := `{"roleName":"ProjectViewer","scopeType":"group","scopeID":` + strconv.Itoa(group.ID) + `}`
	scopedReq := httptest.NewRequest("POST", "/api/v1/groups/"+strconv.Itoa(group.ID)+"/role-bindings", bytes.NewBufferString(scopedBody))
	scopedReq.Header.Set("Authorization", "Bearer "+token)
	scopedReq.Header.Set("Content-Type", "application/json")
	scopedResp, err := fiberApp.Test(scopedReq)
	if err != nil {
		t.Fatalf("scoped owner grant route returned error: %v", err)
	}
	if scopedResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected scoped binding to be created, got %d", scopedResp.StatusCode)
	}

	bindings, err := db.RoleBindingsForSubject(db.RoleBindingSubjectGroup, group.ID)
	if err != nil {
		t.Fatalf("RoleBindingsForSubject returned error: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected one group binding, got %#v", bindings)
	}
	wrongGroupReq := httptest.NewRequest("DELETE", "/api/v1/groups/"+strconv.Itoa(otherGroup.ID)+"/role-bindings/"+strconv.Itoa(bindings[0].ID), nil)
	wrongGroupReq.Header.Set("Authorization", "Bearer "+token)
	wrongGroupResp, err := fiberApp.Test(wrongGroupReq)
	if err != nil {
		t.Fatalf("wrong group delete route returned error: %v", err)
	}
	if wrongGroupResp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected wrong group delete to return 404, got %d", wrongGroupResp.StatusCode)
	}

	deleteReq := httptest.NewRequest("DELETE", "/api/v1/groups/"+strconv.Itoa(group.ID)+"/role-bindings/"+strconv.Itoa(bindings[0].ID), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteResp, err := fiberApp.Test(deleteReq)
	if err != nil {
		t.Fatalf("delete owner grant route returned error: %v", err)
	}
	if deleteResp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected scoped binding to be deleted, got %d", deleteResp.StatusCode)
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

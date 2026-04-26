package app

import (
	"strconv"
	"strings"

	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

type userCreateRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type userImportRequest struct {
	Entries   string   `json:"entries"`
	Usernames []string `json:"usernames"`
}

type userImportResult struct {
	Query       string   `json:"query"`
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
	User        *db.User `json:"user,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Email       string   `json:"email,omitempty"`
}

type groupCreateRequest struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description"`
	GroupType      string `json:"groupType"`
	ParentGroupID  *int   `json:"parentGroupID"`
	OwnerScopeType string `json:"ownerScopeType"`
	OwnerScopeID   *int   `json:"ownerScopeID"`
	SyncSource     string `json:"syncSource"`
	ExternalID     string `json:"externalID"`
	SyncMembership bool   `json:"syncMembership"`
}

type groupMembershipRequest struct {
	UserID         int    `json:"userID"`
	UserRef        string `json:"userRef"`
	MembershipRole string `json:"membershipRole"`
}

type groupRoleBindingRequest struct {
	RoleID      int    `json:"roleID"`
	RoleName    string `json:"roleName"`
	SubjectType string `json:"subjectType"`
	SubjectID   int    `json:"subjectID"`
	SubjectRef  string `json:"subjectRef"`
	ScopeType   string `json:"scopeType"`
	ScopeID     *int   `json:"scopeID"`
}

type projectMembershipRequest struct {
	SubjectType string `json:"subjectType"`
	SubjectID   int    `json:"subjectID"`
	SubjectRef  string `json:"subjectRef"`
	ProjectRole string `json:"projectRole"`
}

type roleCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type rolePermissionRequest struct {
	PermissionID   int    `json:"permissionID"`
	PermissionName string `json:"permissionName"`
}

func getUsers(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	users, err := db.ListUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load users"})
	}
	return c.JSON(users)
}

func postCreateUser(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	var req userCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user request"})
	}

	user, _, err := db.EnsureUser(
		strings.TrimSpace(req.Username),
		strings.TrimSpace(req.DisplayName),
		strings.TrimSpace(req.Email),
		"local",
		strings.TrimSpace(req.Username),
	)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(user)
}

func postImportUsers(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	var req userImportRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user import request"})
	}

	queries := userImportQueries(req)
	if len(queries) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "at least one FreeIPA username or email is required"})
	}

	results := make([]userImportResult, 0, len(queries))
	imported := 0
	failed := 0
	for _, query := range queries {
		result := userImportResult{Query: query}
		ldapUser, found, lookupErr := auth.LookupUser(query)
		if lookupErr != nil {
			result.Status = "failed"
			result.Error = lookupErr.Error()
			failed++
			results = append(results, result)
			continue
		}
		if !found {
			result.Status = "failed"
			result.Error = "user was not found in FreeIPA"
			failed++
			results = append(results, result)
			continue
		}

		user, created, ensureErr := db.EnsureUser(ldapUser.Username, ldapUser.DisplayName, ldapUser.Email, "ldap", ldapUser.Username)
		if ensureErr != nil {
			result.Status = "failed"
			result.Error = ensureErr.Error()
			failed++
			results = append(results, result)
			continue
		}

		if created {
			result.Status = "imported"
		} else {
			result.Status = "already-imported"
		}
		result.User = user
		result.DisplayName = ldapUser.DisplayName
		result.Email = ldapUser.Email
		imported++
		results = append(results, result)
	}

	return c.JSON(fiber.Map{
		"failed":   failed,
		"imported": imported,
		"results":  results,
		"total":    len(results),
	})
}

func userImportQueries(req userImportRequest) []string {
	raw := append([]string{}, req.Usernames...)
	raw = append(raw, strings.FieldsFunc(req.Entries, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})...)

	seen := make(map[string]bool, len(raw))
	queries := make([]string, 0, len(raw))
	for _, value := range raw {
		query := strings.TrimSpace(value)
		if query == "" {
			continue
		}
		key := strings.ToLower(query)
		if seen[key] {
			continue
		}
		seen[key] = true
		queries = append(queries, query)
	}
	return queries
}

func getCloudGroups(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionGroupManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	groups, err := db.ListCloudGroups()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load groups"})
	}
	items := make([]fiber.Map, 0, len(groups))
	for _, group := range groups {
		item, itemErr := groupResponse(group)
		if itemErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group metadata"})
		}
		items = append(items, item)
	}
	return c.JSON(items)
}

func postCreateCloudGroup(c *fiber.Ctx) error {
	var req groupCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group request"})
	}
	ownerScopeType := parseGroupOwnerScope(req.OwnerScopeType)
	allowed, err := currentUserCan(c, db.PermissionGroupManage, ownerScopeType, req.OwnerScopeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	if req.SyncMembership || strings.EqualFold(strings.TrimSpace(req.SyncSource), db.CloudGroupSyncSourceLDAP) {
		if !currentUserIsSiteAdmin(c) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "LDAP group sync requires site admin"})
		}
	}

	groupType := parseGroupType(req.GroupType)
	group, _, err := db.EnsureCloudGroup(strings.TrimSpace(req.Name), strings.TrimSpace(req.Slug), groupType)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	group.ParentGroupID = req.ParentGroupID
	group.OwnerScopeType = ownerScopeType
	group.OwnerScopeID = req.OwnerScopeID
	if req.Description != "" {
		group.Description = strings.TrimSpace(req.Description)
	}
	syncSource := parseCloudGroupSyncSource(req.SyncSource)
	if req.SyncMembership && syncSource == db.CloudGroupSyncSourceLocal {
		syncSource = db.CloudGroupSyncSourceLDAP
	}
	group.SyncSource = syncSource
	group.SyncMembership = syncSource == db.CloudGroupSyncSourceLDAP && req.SyncMembership
	if syncSource == db.CloudGroupSyncSourceLDAP {
		group.ExternalID = strings.TrimSpace(req.ExternalID)
		if group.ExternalID == "" {
			group.ExternalID = strings.TrimSpace(req.Name)
		}
	} else {
		group.ExternalID = ""
		group.SyncMembership = false
	}
	if err := db.UpdateCloudGroup(group); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update group metadata"})
	}

	return c.Status(fiber.StatusCreated).JSON(group)
}

func parseGroupOwnerScope(value string) db.RoleBindingScope {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "org":
		return db.RoleBindingScopeOrg
	case "project":
		return db.RoleBindingScopeProject
	default:
		return db.RoleBindingScopeGlobal
	}
}

func getCloudGroupByID(c *fiber.Ctx) error {
	groupID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	allowed, err := currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	group, found, err := db.GetCloudGroupByID(groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}

	item, err := groupResponse(group)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group metadata"})
	}
	return c.JSON(item)
}

func getGroupMemberships(c *fiber.Ctx) error {
	groupID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	allowed, err := currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	memberships, err := db.CloudGroupMembershipsForGroup(groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load memberships"})
	}

	items := make([]fiber.Map, 0, len(memberships))
	for _, membership := range memberships {
		item := fiber.Map{
			"id":                    membership.ID,
			"user_id":               membership.UserID,
			"group_id":              membership.GroupID,
			"membership_role":       membership.MembershipRole,
			"membership_role_label": membershipRoleLabel(membership.MembershipRole),
			"created_at":            membership.CreatedAt,
		}
		if user, found, userErr := db.GetUserByID(membership.UserID); userErr == nil && found {
			item["user"] = fiber.Map{
				"id":           user.ID,
				"username":     user.Username,
				"display_name": user.DisplayName,
				"email":        user.Email,
				"label":        userLabel(user),
			}
		}
		items = append(items, item)
	}

	return c.JSON(items)
}

func postCreateGroupMembership(c *fiber.Ctx) error {
	groupID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}

	allowed, err := currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var req groupMembershipRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}

	userID, err := resolveUserID(req.UserID, req.UserRef)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	role := parseMembershipRole(req.MembershipRole)

	if _, err := db.EnsureCloudGroupMembership(userID, groupID, role); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusCreated)
}

func patchGroupMembership(c *fiber.Ctx) error {
	groupID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	membershipID, err := strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}

	allowed, err := currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	membership, err := db.CloudGroupMemberships.Select(membershipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	if membership == nil || membership.GroupID != groupID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
	}

	var req groupMembershipRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}
	updated, err := db.UpdateCloudGroupMembershipRole(membershipID, parseMembershipRole(req.MembershipRole))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update membership"})
	}
	return c.JSON(updated)
}

func deleteGroupMembership(c *fiber.Ctx) error {
	groupID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	membershipID, err := strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}

	allowed, err := currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	if err := db.RemoveCloudGroupMembership(membershipID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove membership"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func getGroupRoleBindings(c *fiber.Ctx) error {
	groupID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	allowed, err := currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	bindings, err := db.RoleBindingsForSubject(db.RoleBindingSubjectGroup, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role bindings"})
	}

	items, err := roleBindingResponse(bindings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role binding roles"})
	}
	return c.JSON(items)
}

func postCreateGroupRoleBinding(c *fiber.Ctx) error {
	groupID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}

	var req groupRoleBindingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role binding request"})
	}

	scopeType := parseRoleBindingScope(req.ScopeType)
	allowed, err := currentUserCanBindRolesForGroup(c, groupID, scopeType, req.ScopeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	roleID, err := resolveRoleID(req.RoleID, req.RoleName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if _, err := db.EnsureRoleBinding(roleID, db.RoleBindingSubjectGroup, groupID, scopeType, req.ScopeID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusCreated)
}

func deleteGroupRoleBinding(c *fiber.Ctx) error {
	groupID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	bindingID, err := strconv.Atoi(c.Params("bindingID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role binding id"})
	}

	binding, err := db.RoleBindings.Select(bindingID)
	if err != nil || binding == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "role binding not found"})
	}

	allowed, err := currentUserCanBindRolesForGroup(c, groupID, binding.ScopeType, binding.ScopeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	if err := db.RemoveRoleBinding(bindingID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove role binding"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func getRoleBindings(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	bindings, err := db.RoleBindings.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role bindings"})
	}
	items, err := roleBindingResponse(bindings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role binding metadata"})
	}
	return c.JSON(items)
}

func postCreateRoleBinding(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	var req groupRoleBindingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role binding request"})
	}

	roleID, err := resolveRoleID(req.RoleID, req.RoleName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	subjectType := parseRoleBindingSubject(req.SubjectType)
	subjectID, err := resolveRoleBindingSubject(subjectType, req.SubjectID, req.SubjectRef)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if _, err := db.EnsureRoleBinding(roleID, subjectType, subjectID, parseRoleBindingScope(req.ScopeType), req.ScopeID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusCreated)
}

func deleteRoleBinding(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	bindingID, err := strconv.Atoi(c.Params("bindingID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role binding id"})
	}
	if err := db.RemoveRoleBinding(bindingID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove role binding"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func getProjectBySlug(c *fiber.Ctx) error {
	project, found, err := db.GetProjectBySlug(c.Params("slug"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}

	allowed, err := currentUserCanViewProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	return c.JSON(project)
}

func getProjectMemberships(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}

	project, found, err := db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}

	allowed, err := currentUserCanViewProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	memberships, err := db.ProjectMembershipsForProject(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load memberships"})
	}

	items, err := projectMembershipResponse(memberships)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership subjects"})
	}
	return c.JSON(items)
}

func postCreateProjectMembership(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}

	project, found, err := db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}

	allowed, err := currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var req projectMembershipRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}

	subjectType := db.ProjectMemberSubjectUser
	if strings.EqualFold(req.SubjectType, "group") {
		subjectType = db.ProjectMemberSubjectGroup
	}

	subjectID, err := resolveProjectMembershipSubject(subjectType, req.SubjectID, req.SubjectRef, currentUserIsSiteAdmin(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if subjectType == db.ProjectMemberSubjectGroup {
		group, found, err := db.GetCloudGroupByID(subjectID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group"})
		}
		if found && group.OwnerScopeType == db.RoleBindingScopeGlobal && group.GroupType == db.GroupTypeProject {
			group.OwnerScopeType = db.RoleBindingScopeProject
			group.OwnerScopeID = &projectID
			if err := db.UpdateCloudGroup(group); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scope group"})
			}
		}
	}

	if _, err := db.EnsureProjectMembership(projectID, subjectType, subjectID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if strings.TrimSpace(req.ProjectRole) != "" {
		role := parseProjectRole(req.ProjectRole)
		if err := db.EnsureProjectMemberAccessRole(projectID, subjectType, subjectID, role); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}

	memberships, err := db.ProjectMembershipsForProject(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	for _, membership := range memberships {
		if membership.SubjectType == subjectType && membership.SubjectID == subjectID {
			items, err := projectMembershipResponse([]*db.ProjectMembership{membership})
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership subject"})
			}
			return c.Status(fiber.StatusCreated).JSON(items[0])
		}
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
}

func patchProjectMembership(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	membershipID, err := strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}

	project, found, err := db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}

	allowed, err := currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	membership, err := db.ProjectMemberships.Select(membershipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	if membership == nil || membership.ProjectID != projectID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
	}

	var req projectMembershipRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}
	if err := db.EnsureProjectMemberAccessRole(projectID, membership.SubjectType, membership.SubjectID, parseProjectRole(req.ProjectRole)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update membership access"})
	}
	items, err := projectMembershipResponse([]*db.ProjectMembership{membership})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	return c.JSON(items[0])
}

func deleteProjectMembership(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	membershipID, err := strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}

	project, found, err := db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}

	allowed, err := currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	membership, err := db.ProjectMemberships.Select(membershipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	if membership == nil || membership.ProjectID != projectID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
	}

	if err := db.RemoveProjectMemberAccessRoles(projectID, membership.SubjectType, membership.SubjectID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove membership access"})
	}
	if err := db.RemoveProjectMembership(membershipID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove membership"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func deleteProjectBySlug(c *fiber.Ctx) error {
	project, found, err := db.GetProjectBySlug(c.Params("slug"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}

	allowed, err := currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	if err := db.DeleteProject(project.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete project"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func resolveProjectMembershipSubject(subjectType db.ProjectMemberSubject, subjectID int, subjectRef string, allowLDAPGroupSync bool) (int, error) {
	if subjectID > 0 {
		return subjectID, nil
	}

	subjectRef = strings.TrimSpace(subjectRef)
	if subjectRef == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "subject is required")
	}

	if subjectType == db.ProjectMemberSubjectGroup {
		group, found, err := findCloudGroupByRef(subjectRef)
		if err != nil {
			return 0, err
		}
		if !found && allowLDAPGroupSync {
			group, found, err = syncOrFindCloudGroup(subjectRef)
			if err != nil {
				return 0, err
			}
		}
		if !found {
			return 0, fiber.NewError(fiber.StatusBadRequest, "group was not found")
		}
		return group.ID, nil
	}

	user, found, err := syncOrFindUser(subjectRef)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "user was not found in local users or IPA")
	}

	return user.ID, nil
}

func syncOrFindCloudGroup(ref string) (*db.CloudGroup, bool, error) {
	group, found, err := findCloudGroupByRef(ref)
	if err != nil || found {
		return group, found, err
	}

	ldapGroup, found, err := auth.LookupGroup(ref)
	if err != nil || !found {
		return nil, false, err
	}

	group, _, err = db.EnsureCloudGroup(ldapGroup.Name, "", db.GroupTypeProject)
	if err != nil {
		return nil, false, err
	}
	if group.SyncSource != db.CloudGroupSyncSourceLDAP || group.ExternalID != ldapGroup.Name || !group.SyncMembership {
		group.SyncSource = db.CloudGroupSyncSourceLDAP
		group.ExternalID = ldapGroup.Name
		group.SyncMembership = true
		if err := db.UpdateCloudGroup(group); err != nil {
			return nil, false, err
		}
	}
	return group, true, nil
}

func findCloudGroupByRef(ref string) (*db.CloudGroup, bool, error) {
	ref = strings.TrimSpace(ref)
	refSlug := slugForComparison(ref)
	groups, err := db.ListCloudGroups()
	if err != nil {
		return nil, false, err
	}
	for _, group := range groups {
		candidates := []string{group.Slug, group.Name, group.ExternalID}
		for _, candidate := range candidates {
			clean := strings.TrimSpace(candidate)
			if clean == "" {
				continue
			}
			if strings.EqualFold(clean, ref) || slugForComparison(clean) == refSlug {
				return group, true, nil
			}
		}
	}
	return nil, false, nil
}

func syncUserFromLDAP(ref string) (*db.User, bool, error) {
	ldapUser, found, err := auth.LookupUser(ref)
	if err != nil || !found {
		return nil, false, err
	}

	user, _, err := db.EnsureUser(ldapUser.Username, ldapUser.DisplayName, ldapUser.Email, "ldap", ldapUser.Username)
	if err != nil {
		return nil, false, err
	}

	return user, true, nil
}

func getRoles(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	roles, err := db.Roles.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load roles"})
	}
	items, err := roleResponse(roles)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role metadata"})
	}
	return c.JSON(items)
}

func postCreateRole(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	var req roleCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role request"})
	}

	role, created, err := db.EnsureRole(req.Name, req.Description, false)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if !created && !role.IsSystemRole && strings.TrimSpace(req.Description) != "" && role.Description != strings.TrimSpace(req.Description) {
		role.Description = strings.TrimSpace(req.Description)
		if err := db.UpdateRole(role); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update role"})
		}
	}
	items, err := roleResponse([]*db.Role{role})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role metadata"})
	}
	status := fiber.StatusOK
	if created {
		status = fiber.StatusCreated
	}
	return c.Status(status).JSON(items[0])
}

func getRolePermissions(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	role, err := roleFromParam(c)
	if err != nil {
		return roleParamError(c, err)
	}
	items, err := rolePermissionResponse(role.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role permissions"})
	}
	return c.JSON(items)
}

func postCreateRolePermission(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	role, err := roleFromParam(c)
	if err != nil {
		return roleParamError(c, err)
	}
	if role.IsSystemRole {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "system role permissions are managed by setup"})
	}

	var req rolePermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid permission request"})
	}
	permissionID, err := resolvePermissionID(req.PermissionID, req.PermissionName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if _, err := db.EnsureRolePermission(role.ID, permissionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to grant permission"})
	}
	items, err := rolePermissionResponse(role.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role permissions"})
	}
	for _, item := range items {
		if itemPermissionID, ok := item["permission_id"].(int); ok && itemPermissionID == permissionID {
			return c.Status(fiber.StatusCreated).JSON(item)
		}
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role permission"})
}

func deleteRolePermission(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	role, err := roleFromParam(c)
	if err != nil {
		return roleParamError(c, err)
	}
	if role.IsSystemRole {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "system role permissions are managed by setup"})
	}
	permissionID, err := strconv.Atoi(c.Params("permissionID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid permission id"})
	}
	if err := db.RemoveRolePermission(role.ID, permissionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke permission"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func groupResponse(group *db.CloudGroup) (fiber.Map, error) {
	memberships, err := db.CloudGroupMembershipsForGroup(group.ID)
	if err != nil {
		return nil, err
	}
	roleBindings, err := db.RoleBindingsForSubject(db.RoleBindingSubjectGroup, group.ID)
	if err != nil {
		return nil, err
	}

	item := fiber.Map{
		"id":                 group.ID,
		"uuid":               group.UUID,
		"name":               group.Name,
		"slug":               group.Slug,
		"description":        group.Description,
		"group_type":         group.GroupType,
		"group_type_label":   groupTypeLabel(group.GroupType),
		"parent_group_id":    group.ParentGroupID,
		"owner_scope_type":   group.OwnerScopeType,
		"owner_scope_label":  roleBindingScopeLabel(group.OwnerScopeType),
		"owner_scope_id":     group.OwnerScopeID,
		"sync_source":        cloudGroupSyncSource(group),
		"external_id":        group.ExternalID,
		"sync_membership":    group.SyncMembership,
		"member_count":       len(memberships),
		"role_binding_count": len(roleBindings),
		"created_at":         group.CreatedAt,
		"updated_at":         group.UpdatedAt,
	}
	if group.ParentGroupID != nil {
		if parent, found, err := db.GetCloudGroupByID(*group.ParentGroupID); err != nil {
			return nil, err
		} else if found {
			item["parent_group"] = fiber.Map{
				"id":    parent.ID,
				"name":  parent.Name,
				"slug":  parent.Slug,
				"label": parent.Name,
			}
		}
	}
	return item, nil
}

func roleResponse(roles []*db.Role) ([]fiber.Map, error) {
	items := make([]fiber.Map, 0, len(roles))
	for _, role := range roles {
		rolePermissions, err := db.RolePermissionsForRole(role.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, fiber.Map{
			"id":               role.ID,
			"name":             role.Name,
			"description":      role.Description,
			"is_system_role":   role.IsSystemRole,
			"permission_count": len(rolePermissions),
			"created_at":       role.CreatedAt,
			"updated_at":       role.UpdatedAt,
		})
	}
	return items, nil
}

func rolePermissionResponse(roleID int) ([]fiber.Map, error) {
	grants, err := db.RolePermissionsForRole(roleID)
	if err != nil {
		return nil, err
	}
	items := make([]fiber.Map, 0, len(grants))
	for _, grant := range grants {
		permission, err := db.Permissions.Select(grant.PermissionID)
		if err != nil {
			return nil, err
		}
		if permission == nil {
			continue
		}
		items = append(items, fiber.Map{
			"id":            grant.ID,
			"role_id":       grant.RoleID,
			"permission_id": grant.PermissionID,
			"permission": fiber.Map{
				"id":          permission.ID,
				"name":        permission.Name,
				"description": permission.Description,
			},
		})
	}
	return items, nil
}

func roleBindingResponse(bindings []*db.RoleBinding) ([]fiber.Map, error) {
	items := make([]fiber.Map, 0, len(bindings))
	for _, binding := range bindings {
		role, err := db.Roles.Select(binding.RoleID)
		if err != nil {
			return nil, err
		}
		item := fiber.Map{
			"id":                 binding.ID,
			"role_id":            binding.RoleID,
			"subject_type":       binding.SubjectType,
			"subject_type_label": roleBindingSubjectLabel(binding.SubjectType),
			"subject_id":         binding.SubjectID,
			"scope_type":         binding.ScopeType,
			"scope_type_label":   roleBindingScopeLabel(binding.ScopeType),
			"scope_id":           binding.ScopeID,
			"created_at":         binding.CreatedAt,
		}
		if role != nil {
			item["role"] = fiber.Map{
				"id":          role.ID,
				"name":        role.Name,
				"description": role.Description,
			}
		}
		subject, err := roleBindingSubjectResponse(binding.SubjectType, binding.SubjectID)
		if err != nil {
			return nil, err
		}
		if subject != nil {
			item["subject"] = subject
		}
		items = append(items, item)
	}
	return items, nil
}

func roleBindingSubjectResponse(subjectType db.RoleBindingSubject, subjectID int) (fiber.Map, error) {
	if subjectType == db.RoleBindingSubjectGroup {
		group, found, err := db.GetCloudGroupByID(subjectID)
		if err != nil || !found {
			return nil, err
		}
		return fiber.Map{
			"id":    group.ID,
			"name":  group.Name,
			"slug":  group.Slug,
			"label": group.Name,
			"meta":  group.Slug,
		}, nil
	}

	user, found, err := db.GetUserByID(subjectID)
	if err != nil || !found {
		return nil, err
	}
	return fiber.Map{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"email":        user.Email,
		"label":        userLabel(user),
		"meta":         userMeta(user),
	}, nil
}

func resolveUserID(userID int, userRef string) (int, error) {
	if userID > 0 {
		return userID, nil
	}
	user, found, err := syncOrFindUser(userRef)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "user was not found in local users or IPA")
	}
	return user.ID, nil
}

func syncOrFindUser(ref string) (*db.User, bool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, false, fiber.NewError(fiber.StatusBadRequest, "user is required")
	}
	users, err := db.ListUsers()
	if err != nil {
		return nil, false, err
	}
	for _, user := range users {
		if strings.EqualFold(user.Username, ref) || strings.EqualFold(user.Email, ref) || strings.EqualFold(user.DisplayName, ref) {
			return user, true, nil
		}
	}
	return syncUserFromLDAP(ref)
}

func resolveRoleID(roleID int, roleName string) (int, error) {
	if roleID > 0 {
		return roleID, nil
	}
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "role is required")
	}
	roles, err := db.Roles.SelectAll()
	if err != nil {
		return 0, err
	}
	for _, role := range roles {
		if strings.EqualFold(role.Name, roleName) {
			return role.ID, nil
		}
	}
	return 0, fiber.NewError(fiber.StatusBadRequest, "role was not found")
}

func resolveRoleBindingSubject(subjectType db.RoleBindingSubject, subjectID int, subjectRef string) (int, error) {
	if subjectID > 0 {
		return subjectID, nil
	}
	subjectRef = strings.TrimSpace(subjectRef)
	if subjectRef == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "subject is required")
	}

	if subjectType == db.RoleBindingSubjectGroup {
		groups, err := db.ListCloudGroups()
		if err != nil {
			return 0, err
		}
		for _, group := range groups {
			if strings.EqualFold(group.Slug, subjectRef) || strings.EqualFold(group.Name, subjectRef) {
				return group.ID, nil
			}
		}
		return 0, fiber.NewError(fiber.StatusBadRequest, "group was not found")
	}

	user, found, err := syncOrFindUser(subjectRef)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "user was not found in local users or IPA")
	}
	return user.ID, nil
}

func resolvePermissionID(permissionID int, permissionName string) (int, error) {
	if permissionID > 0 {
		return permissionID, nil
	}
	permissionName = strings.TrimSpace(permissionName)
	if permissionName == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "permission is required")
	}
	permission, found, err := db.GetPermissionByName(permissionName)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "permission was not found")
	}
	return permission.ID, nil
}

func roleFromParam(c *fiber.Ctx) (*db.Role, error) {
	roleID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid role id")
	}
	role, err := db.Roles.Select(roleID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to load role")
	}
	if role == nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "role not found")
	}
	return role, nil
}

func roleParamError(c *fiber.Ctx, err error) error {
	if fiberErr, ok := err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role"})
}

func parseCloudGroupSyncSource(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), db.CloudGroupSyncSourceLDAP) {
		return db.CloudGroupSyncSourceLDAP
	}
	return db.CloudGroupSyncSourceLocal
}

func cloudGroupSyncSource(group *db.CloudGroup) string {
	if group != nil && group.SyncSource == db.CloudGroupSyncSourceLDAP {
		return db.CloudGroupSyncSourceLDAP
	}
	return db.CloudGroupSyncSourceLocal
}

func parseGroupType(value string) db.GroupType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "admin":
		return db.GroupTypeAdmin
	case "club":
		return db.GroupTypeClub
	case "competition":
		return db.GroupTypeCompetition
	case "student_group":
		return db.GroupTypeStudentGroup
	case "project":
		return db.GroupTypeProject
	default:
		return db.GroupTypeCustom
	}
}

func groupTypeLabel(value db.GroupType) string {
	switch value {
	case db.GroupTypeAdmin:
		return "admin"
	case db.GroupTypeClub:
		return "club"
	case db.GroupTypeCompetition:
		return "competition"
	case db.GroupTypeStudentGroup:
		return "student_group"
	case db.GroupTypeProject:
		return "project"
	default:
		return "custom"
	}
}

func parseMembershipRole(value string) db.MembershipRole {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "manager":
		return db.MembershipRoleManager
	case "owner":
		return db.MembershipRoleOwner
	default:
		return db.MembershipRoleMember
	}
}

func parseProjectRole(value string) db.ProjectRole {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "operator":
		return db.ProjectRoleOperator
	case "developer":
		return db.ProjectRoleDeveloper
	case "manager":
		return db.ProjectRoleManager
	case "owner":
		return db.ProjectRoleOwner
	default:
		return db.ProjectRoleViewer
	}
}

func membershipRoleLabel(value db.MembershipRole) string {
	switch value {
	case db.MembershipRoleManager:
		return "manager"
	case db.MembershipRoleOwner:
		return "owner"
	default:
		return "member"
	}
}

func parseRoleBindingSubject(value string) db.RoleBindingSubject {
	if strings.EqualFold(strings.TrimSpace(value), "user") {
		return db.RoleBindingSubjectUser
	}
	return db.RoleBindingSubjectGroup
}

func roleBindingSubjectLabel(value db.RoleBindingSubject) string {
	if value == db.RoleBindingSubjectUser {
		return "user"
	}
	return "group"
}

func parseRoleBindingScope(value string) db.RoleBindingScope {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "org":
		return db.RoleBindingScopeOrg
	case "project":
		return db.RoleBindingScopeProject
	case "group":
		return db.RoleBindingScopeGroup
	case "resource":
		return db.RoleBindingScopeResource
	default:
		return db.RoleBindingScopeGlobal
	}
}

func roleBindingScopeLabel(value db.RoleBindingScope) string {
	switch value {
	case db.RoleBindingScopeOrg:
		return "org"
	case db.RoleBindingScopeProject:
		return "project"
	case db.RoleBindingScopeGroup:
		return "group"
	case db.RoleBindingScopeResource:
		return "resource"
	default:
		return "global"
	}
}

func projectMembershipResponse(memberships []*db.ProjectMembership) ([]fiber.Map, error) {
	items := make([]fiber.Map, 0, len(memberships))
	for _, membership := range memberships {
		projectRole, found, err := db.ProjectMemberAccessRole(membership.ProjectID, membership.SubjectType, membership.SubjectID)
		if err != nil {
			return nil, err
		}
		if !found {
			projectRole = db.ProjectRoleViewer
		}
		item := fiber.Map{
			"id":                 membership.ID,
			"project_id":         membership.ProjectID,
			"subject_type":       membership.SubjectType,
			"subject_id":         membership.SubjectID,
			"project_role":       projectRole,
			"project_role_label": projectRoleLabel(projectRole),
			"created_at":         membership.CreatedAt,
		}

		if membership.SubjectType == db.ProjectMemberSubjectGroup {
			group, found, err := db.GetCloudGroupByID(membership.SubjectID)
			if err != nil {
				return nil, err
			}
			if found {
				item["subject"] = fiber.Map{
					"id":    group.ID,
					"name":  group.Name,
					"slug":  group.Slug,
					"label": group.Name,
					"meta":  group.Slug,
				}
			}
		} else {
			user, found, err := db.GetUserByID(membership.SubjectID)
			if err != nil {
				return nil, err
			}
			if found {
				item["subject"] = fiber.Map{
					"id":           user.ID,
					"username":     user.Username,
					"display_name": user.DisplayName,
					"email":        user.Email,
					"label":        userLabel(user),
					"meta":         userMeta(user),
				}
			}
		}

		items = append(items, item)
	}
	return items, nil
}

func projectRoleLabel(value db.ProjectRole) string {
	switch value {
	case db.ProjectRoleOperator:
		return "operator"
	case db.ProjectRoleDeveloper:
		return "developer"
	case db.ProjectRoleManager:
		return "manager"
	case db.ProjectRoleOwner:
		return "owner"
	default:
		return "viewer"
	}
}

func userLabel(user *db.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

func userMeta(user *db.User) string {
	if user.Email != "" {
		return user.Username + " · " + user.Email
	}
	return user.Username
}

func currentUserCanViewProject(c *fiber.Ctx, project *db.Project) (bool, error) {
	allowed, err := currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || allowed {
		return allowed, err
	}
	allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeProject, &project.ID)
	if err != nil || allowed {
		return allowed, err
	}
	allowed, err = currentUserCan(c, db.PermissionVMRead, db.RoleBindingScopeProject, &project.ID)
	if err != nil || allowed {
		return allowed, err
	}

	dbUser := currentDBUser(c)
	if dbUser == nil {
		return false, nil
	}

	groupIDs, err := db.CloudGroupIDsForUser(dbUser.ID)
	if err != nil {
		return false, err
	}

	if member, err := db.SubjectInProjectOrAncestor(project.ID, db.ProjectMemberSubjectUser, dbUser.ID); err != nil || member {
		return member, err
	}
	for _, groupID := range groupIDs {
		if member, err := db.SubjectInProjectOrAncestor(project.ID, db.ProjectMemberSubjectGroup, groupID); err != nil || member {
			return true, nil
		}
	}
	return false, nil
}

func currentUserCanManageProject(c *fiber.Ctx, project *db.Project) (bool, error) {
	allowed, err := currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || allowed {
		return allowed, err
	}
	allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeProject, &project.ID)
	if err != nil || allowed {
		return allowed, err
	}

	return false, nil
}

func currentUserCanManageGroup(c *fiber.Ctx, groupID int) (bool, error) {
	allowed, err := currentUserCan(c, db.PermissionGroupManage, db.RoleBindingScopeGroup, &groupID)
	if err != nil || allowed {
		return allowed, err
	}

	group, found, err := db.GetCloudGroupByID(groupID)
	if err != nil || !found {
		return false, err
	}
	if group.OwnerScopeType != db.RoleBindingScopeGlobal {
		allowed, err = currentUserCan(c, db.PermissionGroupManage, group.OwnerScopeType, group.OwnerScopeID)
		if err != nil || allowed {
			return allowed, err
		}
	}

	dbUser := currentDBUser(c)
	if dbUser == nil {
		return false, nil
	}

	membership, found, err := db.CloudGroupMembershipForUserAndGroup(dbUser.ID, groupID)
	if err != nil || !found {
		return false, err
	}

	return membership.MembershipRole == db.MembershipRoleManager || membership.MembershipRole == db.MembershipRoleOwner, nil
}

func currentUserCanBindRolesForGroup(c *fiber.Ctx, groupID int, scopeType db.RoleBindingScope, scopeID *int) (bool, error) {
	if currentUserIsSiteAdmin(c) {
		return true, nil
	}

	if scopeType != db.RoleBindingScopeGroup || scopeID == nil || *scopeID != groupID {
		return false, nil
	}

	return currentUserCan(c, db.PermissionRoleManage, db.RoleBindingScopeGroup, &groupID)
}

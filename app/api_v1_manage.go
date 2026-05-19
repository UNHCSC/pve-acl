package app

import (
	"strconv"
	"strings"

	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

type (
	userCreateRequest struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
	}

	userImportRequest struct {
		Entries   string   `json:"entries"`
		Usernames []string `json:"usernames"`
	}

	userImportResult struct {
		Query       string   `json:"query"`
		Status      string   `json:"status"`
		Error       string   `json:"error,omitempty"`
		User        *db.User `json:"user,omitempty"`
		DisplayName string   `json:"displayName,omitempty"`
		Email       string   `json:"email,omitempty"`
	}

	groupCreateRequest struct {
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

	groupUpdateRequest struct {
		Name           *string `json:"name"`
		Slug           *string `json:"slug"`
		Description    *string `json:"description"`
		GroupType      *string `json:"groupType"`
		ParentGroupID  *int    `json:"parentGroupID"`
		OwnerScopeType *string `json:"ownerScopeType"`
		OwnerScopeID   *int    `json:"ownerScopeID"`
		SyncSource     *string `json:"syncSource"`
		ExternalID     *string `json:"externalID"`
		SyncMembership *bool   `json:"syncMembership"`
	}

	groupMembershipRequest struct {
		UserID         int    `json:"userID"`
		UserRef        string `json:"userRef"`
		MembershipRole string `json:"membershipRole"`
	}

	groupRoleBindingRequest struct {
		RoleID      int    `json:"roleID"`
		RoleName    string `json:"roleName"`
		SubjectType string `json:"subjectType"`
		SubjectID   int    `json:"subjectID"`
		SubjectRef  string `json:"subjectRef"`
		ScopeType   string `json:"scopeType"`
		ScopeID     *int   `json:"scopeID"`
	}

	projectMembershipRequest struct {
		SubjectType string `json:"subjectType"`
		SubjectID   int    `json:"subjectID"`
		SubjectRef  string `json:"subjectRef"`
		ProjectRole string `json:"projectRole"`
		RoleID      int    `json:"roleID"`
	}

	roleCreateRequest struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ScopeType   string `json:"scopeType"`
		ScopeID     *int   `json:"scopeID"`
	}

	roleUpdateRequest struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}

	rolePermissionRequest struct {
		PermissionID   int    `json:"permissionID"`
		PermissionName string `json:"permissionName"`
	}
)

// getUsers lists all local users for user managers.
func getUsers(c *fiber.Ctx) (err error) {
	var allowed bool
	if allowed, err = requirePermission(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil); err != nil || !allowed {
		return err
	}

	var users []*db.User
	if users, err = db.ListUsers(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load users"})
	}

	return c.JSON(users)
}

// postCreateUser creates or updates a local user entry.
func postCreateUser(c *fiber.Ctx) (errResult error) {
	var (
		allowed bool
		err     error
	)

	if allowed, err = requirePermission(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil); err != nil || !allowed {
		return err
	}

	var req userCreateRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user request"})
	}

	var user *db.User
	if user, _, err = db.EnsureUser(
		strings.TrimSpace(req.Username),
		strings.TrimSpace(req.DisplayName),
		strings.TrimSpace(req.Email),
		"local",
		strings.TrimSpace(req.Username),
	); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(user)
}

// postImportUsers imports one or more users from LDAP.
func postImportUsers(c *fiber.Ctx) (errResult error) {
	var (
		allowed bool
		err     error
	)
	allowed, err = requirePermission(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	var req userImportRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user import request"})
	}
	var queries []string

	queries = userImportQueries(req)
	if len(queries) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "at least one FreeIPA username or email is required"})
	}
	var results []userImportResult

	results = make([]userImportResult, 0, len(queries))
	var imported int
	imported = 0
	var failed int
	failed = 0
	for _, query := range queries {
		var result userImportResult
		result = userImportResult{Query: query}
		var (
			ldapUser  *auth.LDAPUser
			found     bool
			lookupErr error
		)
		ldapUser, found, lookupErr = auth.LookupUser(query)
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
		var (
			user      *db.User
			created   bool
			ensureErr error
		)

		user, created, ensureErr = db.EnsureUser(ldapUser.Username, ldapUser.DisplayName, ldapUser.Email, "ldap", ldapUser.Username)
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

// userImportQueries normalizes and deduplicates user import request entries.
func userImportQueries(req userImportRequest) (itemsResult []string) {
	var raw []string
	raw = append([]string{}, req.Usernames...)
	raw = append(raw, strings.FieldsFunc(req.Entries, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})...)
	var seen map[string]bool

	seen = make(map[string]bool, len(raw))
	var queries []string
	queries = make([]string, 0, len(raw))
	for _, value := range raw {
		var query string
		query = strings.TrimSpace(value)
		if query == "" {
			continue
		}
		var key string
		key = strings.ToLower(query)
		if seen[key] {
			continue
		}
		seen[key] = true
		queries = append(queries, query)
	}
	return queries
}

// getCloudGroups lists cloud groups visible to group managers.
func getCloudGroups(c *fiber.Ctx) (errResult error) {
	var (
		allowed bool
		err     error
	)
	allowed, err = requirePermission(c, db.PermissionGroupManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}
	var groups []*db.CloudGroup

	groups, err = db.ListCloudGroups()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load groups"})
	}
	var items []fiber.Map
	items = make([]fiber.Map, 0, len(groups))
	for _, group := range groups {
		var (
			item    fiber.Map
			itemErr error
		)
		item, itemErr = groupResponse(group)
		if itemErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group metadata"})
		}
		items = append(items, item)
	}
	return c.JSON(items)
}

// postCreateCloudGroup creates a cloud group in an allowed owner scope.
func postCreateCloudGroup(c *fiber.Ctx) (errResult error) {
	var req groupCreateRequest
	var err error
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group request"})
	}
	var ownerScopeType db.RoleBindingScope
	ownerScopeType = parseGroupOwnerScope(req.OwnerScopeType)
	if ownerScopeType == db.RoleBindingScopeGlobal {
		req.OwnerScopeID = nil
	} else if req.OwnerScopeID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "owner scope id is required"})
	}
	var allowed bool
	allowed, err = currentUserCan(c, db.PermissionGroupManage, ownerScopeType, req.OwnerScopeID)
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
	var groupType db.GroupType

	groupType = parseGroupType(req.GroupType)
	var group *db.CloudGroup
	group, _, err = db.EnsureCloudGroup(strings.TrimSpace(req.Name), strings.TrimSpace(req.Slug), groupType)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	group.ParentGroupID = req.ParentGroupID
	group.OwnerScopeType = ownerScopeType
	group.OwnerScopeID = req.OwnerScopeID
	if req.Description != "" {
		group.Description = strings.TrimSpace(req.Description)
	}
	var syncSource string
	syncSource = parseCloudGroupSyncSource(req.SyncSource)
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
	if err = db.UpdateCloudGroup(group); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update group metadata"})
	}

	return c.Status(fiber.StatusCreated).JSON(group)
}

// parseGroupOwnerScope converts a request scope string into a role binding scope.
func parseGroupOwnerScope(value string) (roleBindingScopeResult db.RoleBindingScope) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "org":
		return db.RoleBindingScopeOrg
	case "project":
		return db.RoleBindingScopeProject
	default:
		return db.RoleBindingScopeGlobal
	}
}

// getCloudGroupByID returns a single cloud group with metadata.
func getCloudGroupByID(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var allowed bool
	allowed, err = currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var (
		group *db.CloudGroup
		found bool
	)

	group, found, err = db.GetCloudGroupByID(groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}
	var item fiber.Map

	item, err = groupResponse(group)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group metadata"})
	}
	return c.JSON(item)
}

// patchCloudGroup updates mutable cloud group fields and sync settings.
func patchCloudGroup(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var allowed bool
	allowed, err = currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var (
		group *db.CloudGroup
		found bool
	)

	group, found, err = db.GetCloudGroupByID(groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}
	if group.ArchivedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "archived groups cannot be updated"})
	}

	var req groupUpdateRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group request"})
	}

	if req.Name != nil {
		var name string
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "group name is required"})
		}
		group.Name = name
	}
	if req.Slug != nil {
		var slug string
		slug = strings.TrimSpace(*req.Slug)
		if slug == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "group slug is required"})
		}
		var (
			existing      *db.CloudGroup
			existingFound bool
			findErr       error
		)
		if existing, existingFound, findErr = db.GetCloudGroupBySlug(slug); findErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to validate group slug"})
		} else if existingFound && existing.ID != group.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "group slug is already in use"})
		}
		group.Slug = slug
	}
	if req.Description != nil {
		group.Description = strings.TrimSpace(*req.Description)
	}
	if req.GroupType != nil {
		group.GroupType = parseGroupType(*req.GroupType)
	}
	if req.ParentGroupID != nil {
		if *req.ParentGroupID == group.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "group cannot be its own parent"})
		}
		group.ParentGroupID = req.ParentGroupID
	}
	if req.OwnerScopeType != nil || req.OwnerScopeID != nil {
		var ownerScopeType db.RoleBindingScope
		ownerScopeType = group.OwnerScopeType
		if req.OwnerScopeType != nil {
			ownerScopeType = parseGroupOwnerScope(*req.OwnerScopeType)
		}
		var ownerScopeID *int
		ownerScopeID = group.OwnerScopeID
		if req.OwnerScopeID != nil {
			ownerScopeID = req.OwnerScopeID
		}
		if ownerScopeType == db.RoleBindingScopeGlobal {
			ownerScopeID = nil
		} else if ownerScopeID == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "owner scope id is required"})
		}
		allowed, err = currentUserCan(c, db.PermissionGroupManage, ownerScopeType, ownerScopeID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		}
		if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied for new owner scope"})
		}
		group.OwnerScopeType = ownerScopeType
		group.OwnerScopeID = ownerScopeID
	}
	if req.SyncSource != nil || req.SyncMembership != nil || req.ExternalID != nil {
		var syncSource string
		syncSource = group.SyncSource
		if req.SyncSource != nil {
			syncSource = parseCloudGroupSyncSource(*req.SyncSource)
		}
		var syncMembership bool
		syncMembership = group.SyncMembership
		if req.SyncMembership != nil {
			syncMembership = *req.SyncMembership
		}
		if syncMembership && syncSource == db.CloudGroupSyncSourceLocal {
			syncSource = db.CloudGroupSyncSourceLDAP
		}
		if syncSource == db.CloudGroupSyncSourceLDAP && !currentUserIsSiteAdmin(c) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "LDAP group sync requires site admin"})
		}
		group.SyncSource = syncSource
		group.SyncMembership = syncSource == db.CloudGroupSyncSourceLDAP && syncMembership
		if group.SyncSource == db.CloudGroupSyncSourceLDAP {
			if req.ExternalID != nil {
				group.ExternalID = strings.TrimSpace(*req.ExternalID)
			}
			if group.ExternalID == "" {
				group.ExternalID = group.Name
			}
		} else {
			group.ExternalID = ""
			group.SyncMembership = false
		}
	}

	if err = db.UpdateCloudGroup(group); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update group"})
	}
	var item fiber.Map
	item, err = groupResponse(group)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group metadata"})
	}
	return c.JSON(item)
}

// deleteCloudGroup archives a cloud group.
func deleteCloudGroup(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var allowed bool
	allowed, err = currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var (
		group *db.CloudGroup
		found bool
	)

	group, found, err = db.GetCloudGroupByID(groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}
	if group.ArchivedAt == nil {
		if err = db.ArchiveCloudGroup(group); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to archive group"})
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// getGroupMemberships lists members for a cloud group.
func getGroupMemberships(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var allowed bool
	allowed, err = currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var memberships []*db.CloudGroupMembership

	memberships, err = db.CloudGroupMembershipsForGroup(groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load memberships"})
	}
	var items []fiber.Map

	items = make([]fiber.Map, 0, len(memberships))
	for _, membership := range memberships {
		var item fiber.Map
		item = fiber.Map{
			"id":                    membership.ID,
			"user_id":               membership.UserID,
			"group_id":              membership.GroupID,
			"membership_role":       membership.MembershipRole,
			"membership_role_label": membershipRoleLabel(membership.MembershipRole),
			"created_at":            membership.CreatedAt,
		}
		var (
			user    *db.User
			found   bool
			userErr error
		)
		if user, found, userErr = db.GetUserByID(membership.UserID); userErr == nil && found {
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

// postCreateGroupMembership adds a user to a cloud group.
func postCreateGroupMembership(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var allowed bool

	allowed, err = currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var req groupMembershipRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}
	var userID int

	userID, err = resolveUserID(req.UserID, req.UserRef)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var role db.MembershipRole
	role = parseMembershipRole(req.MembershipRole)

	if _, err = db.EnsureCloudGroupMembership(userID, groupID, role); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusCreated)
}

// patchGroupMembership updates a cloud group membership role.
func patchGroupMembership(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var membershipID int
	membershipID, err = strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}
	var allowed bool

	allowed, err = currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var membership *db.CloudGroupMembership

	membership, err = db.CloudGroupMemberships.Select(membershipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	if membership == nil || membership.GroupID != groupID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
	}

	var req groupMembershipRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}
	var updated *db.CloudGroupMembership
	updated, err = db.UpdateCloudGroupMembershipRole(membershipID, parseMembershipRole(req.MembershipRole))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update membership"})
	}
	return c.JSON(updated)
}

// deleteGroupMembership removes a cloud group membership.
func deleteGroupMembership(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var membershipID int
	membershipID, err = strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}
	var allowed bool

	allowed, err = currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	if err = db.RemoveCloudGroupMembership(membershipID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove membership"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// getGroupRoleBindings lists role bindings assigned to a cloud group.
func getGroupRoleBindings(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var allowed bool
	allowed, err = currentUserCanManageGroup(c, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var bindings []*db.RoleBinding

	bindings, err = db.RoleBindingsForSubject(db.RoleBindingSubjectGroup, groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role bindings"})
	}
	var items []fiber.Map

	items, err = roleBindingResponse(bindings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role binding roles"})
	}
	return c.JSON(items)
}

// postCreateGroupRoleBinding grants a role to a cloud group.
func postCreateGroupRoleBinding(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}

	var req groupRoleBindingRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role binding request"})
	}
	var scopeType db.RoleBindingScope

	scopeType = parseRoleBindingScope(req.ScopeType)
	var allowed bool
	allowed, err = currentUserCanBindRolesForGroup(c, groupID, scopeType, req.ScopeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var roleID int

	roleID, err = resolveRoleID(req.RoleID, req.RoleName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if _, err = db.EnsureRoleBinding(roleID, db.RoleBindingSubjectGroup, groupID, scopeType, req.ScopeID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusCreated)
}

// deleteGroupRoleBinding removes a role binding from a cloud group.
func deleteGroupRoleBinding(c *fiber.Ctx) (errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid group id"})
	}
	var bindingID int
	bindingID, err = strconv.Atoi(c.Params("bindingID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role binding id"})
	}
	var binding *db.RoleBinding

	binding, err = db.RoleBindings.Select(bindingID)
	if err != nil || binding == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "role binding not found"})
	}
	if binding.SubjectType != db.RoleBindingSubjectGroup || binding.SubjectID != groupID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "role binding not found"})
	}
	var allowed bool

	allowed, err = currentUserCanBindRolesForGroup(c, groupID, binding.ScopeType, binding.ScopeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	if err = db.RemoveRoleBinding(bindingID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove role binding"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// getProjectBySlug returns a project by slug when the current user can view it.
func getProjectBySlug(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		found   bool
		err     error
	)
	project, found, err = db.GetProjectBySlug(c.Params("slug"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanViewProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	return c.JSON(project)
}

// getProjectMemberships lists project memberships with subject metadata.
func getProjectMemberships(c *fiber.Ctx) (errResult error) {
	var (
		projectID int
		err       error
	)
	projectID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	var (
		project *db.Project
		found   bool
	)

	project, found, err = db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanViewProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var memberships []*db.ProjectMembership

	memberships, err = db.ProjectMembershipsForProject(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load memberships"})
	}
	var items []fiber.Map

	items, err = projectMembershipResponse(memberships)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership subjects"})
	}
	return c.JSON(items)
}

// getProjectAssignableRoles lists roles the current user may assign on a project.
func getProjectAssignableRoles(c *fiber.Ctx) (errResult error) {
	var (
		projectID int
		err       error
	)
	projectID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	var (
		project *db.Project
		found   bool
	)

	project, found, err = db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.JSON([]fiber.Map{})
	}
	var roles []*db.Role

	roles, err = db.Roles.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load roles"})
	}
	var assignable []*db.Role
	assignable = make([]*db.Role, 0, len(roles))
	for _, role := range roles {
		var allowErr error
		allowed, allowErr = currentUserCanAssignRoleAtScope(c, role.ID, db.RoleBindingScopeProject, &project.ID)
		if allowErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		}
		if allowed {
			assignable = append(assignable, role)
		}
	}
	var items []fiber.Map
	items, err = roleResponse(assignable)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role metadata"})
	}
	return c.JSON(items)
}

// getProjectOwnedGroups lists cloud groups owned by a project.
func getProjectOwnedGroups(c *fiber.Ctx) (errResult error) {
	var (
		projectID int
		err       error
	)
	projectID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	var (
		project *db.Project
		found   bool
	)

	project, found, err = db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var groups []*db.CloudGroup

	groups, err = db.CloudGroupsForOwner(db.RoleBindingScopeProject, &projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load groups"})
	}
	var items []fiber.Map
	items = make([]fiber.Map, 0, len(groups))
	for _, group := range groups {
		var (
			item    fiber.Map
			itemErr error
		)
		item, itemErr = groupResponse(group)
		if itemErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group metadata"})
		}
		items = append(items, item)
	}
	return c.JSON(items)
}

// getOrganizationMemberships lists organization memberships with subject metadata.
func getOrganizationMemberships(c *fiber.Ctx) (errResult error) {
	var (
		org *db.Organization
		err error
	)
	org, err = organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	var allowed bool
	allowed, err = currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var memberships []*db.OrganizationMembership
	memberships, err = db.OrganizationMembershipsForOrganization(org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load memberships"})
	}
	var items []fiber.Map
	items, err = organizationMembershipResponse(memberships)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership subjects"})
	}
	return c.JSON(items)
}

// getOrganizationOwnedGroups lists cloud groups owned by an organization.
func getOrganizationOwnedGroups(c *fiber.Ctx) (errResult error) {
	var (
		org *db.Organization
		err error
	)
	org, err = organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	var allowed bool
	allowed, err = currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var groups []*db.CloudGroup

	groups, err = db.CloudGroupsForOwner(db.RoleBindingScopeOrg, &org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load groups"})
	}
	var items []fiber.Map
	items = make([]fiber.Map, 0, len(groups))
	for _, group := range groups {
		var (
			item    fiber.Map
			itemErr error
		)
		item, itemErr = groupResponse(group)
		if itemErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group metadata"})
		}
		items = append(items, item)
	}
	return c.JSON(items)
}

// getOrganizationAssignableRoles lists roles the current user may assign on an organization.
func getOrganizationAssignableRoles(c *fiber.Ctx) (errResult error) {
	var (
		org *db.Organization
		err error
	)
	org, err = organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	var allowed bool
	allowed, err = currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.JSON([]fiber.Map{})
	}
	var roles []*db.Role
	roles, err = db.Roles.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load roles"})
	}
	var assignable []*db.Role
	assignable = make([]*db.Role, 0, len(roles))
	for _, role := range roles {
		var allowErr error
		allowed, allowErr = currentUserCanAssignRoleAtScope(c, role.ID, db.RoleBindingScopeOrg, &org.ID)
		if allowErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		}
		if allowed {
			assignable = append(assignable, role)
		}
	}
	var items []fiber.Map
	items, err = roleResponse(assignable)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role metadata"})
	}
	return c.JSON(items)
}

// postCreateOrganizationMembership adds a subject to an organization with an access role.
func postCreateOrganizationMembership(c *fiber.Ctx) (errResult error) {
	var (
		org *db.Organization
		err error
	)
	org, err = organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	var allowed bool
	allowed, err = currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var req projectMembershipRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}
	if req.RoleID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role is required"})
	}
	if allowed, err = currentUserCanAssignRoleAtScope(c, req.RoleID, db.RoleBindingScopeOrg, &org.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	} else if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "role grants permissions outside your organization access"})
	}
	var subjectType db.ProjectMemberSubject

	subjectType = db.ProjectMemberSubjectUser
	if strings.EqualFold(req.SubjectType, "group") {
		subjectType = db.ProjectMemberSubjectGroup
	}
	var subjectID int
	subjectID, err = resolveProjectMembershipSubject(subjectType, req.SubjectID, req.SubjectRef, currentUserIsSiteAdmin(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if _, err = db.EnsureOrganizationMembership(org.ID, subjectType, subjectID, db.MembershipRoleMember); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err = db.EnsureOrganizationMemberRoleBinding(org.ID, subjectType, subjectID, req.RoleID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var memberships []*db.OrganizationMembership
	memberships, err = db.OrganizationMembershipsForOrganization(org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	for _, membership := range memberships {
		if membership.SubjectType == subjectType && membership.SubjectID == subjectID {
			var items []fiber.Map
			items, err = organizationMembershipResponse([]*db.OrganizationMembership{membership})
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership subject"})
			}
			return c.Status(fiber.StatusCreated).JSON(items[0])
		}
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
}

// patchOrganizationMembership updates an organization membership access role.
func patchOrganizationMembership(c *fiber.Ctx) (errResult error) {
	var (
		org *db.Organization
		err error
	)
	org, err = organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	var membershipID int
	membershipID, err = strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}
	var allowed bool
	allowed, err = currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var membership *db.OrganizationMembership
	membership, err = db.OrganizationMemberships.Select(membershipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	if membership == nil || membership.OrganizationID != org.ID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
	}
	var req projectMembershipRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}
	if req.RoleID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role is required"})
	}
	if allowed, err = currentUserCanAssignRoleAtScope(c, req.RoleID, db.RoleBindingScopeOrg, &org.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	} else if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "role grants permissions outside your organization access"})
	}
	if err = db.EnsureOrganizationMemberRoleBinding(org.ID, membership.SubjectType, membership.SubjectID, req.RoleID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update membership access"})
	}
	var items []fiber.Map
	items, err = organizationMembershipResponse([]*db.OrganizationMembership{membership})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	return c.JSON(items[0])
}

// deleteOrganizationMembership removes an organization membership and its access grants.
func deleteOrganizationMembership(c *fiber.Ctx) (errResult error) {
	var (
		org *db.Organization
		err error
	)
	org, err = organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	var membershipID int
	membershipID, err = strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}
	var allowed bool
	allowed, err = currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var membership *db.OrganizationMembership
	membership, err = db.OrganizationMemberships.Select(membershipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	if membership == nil || membership.OrganizationID != org.ID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
	}
	if err = db.RemoveOrganizationMemberAccessRoles(org.ID, membership.SubjectType, membership.SubjectID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove membership access"})
	}
	if err = db.RemoveOrganizationMembership(membershipID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove membership"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// postCreateProjectMembership adds a subject to a project with optional access.
func postCreateProjectMembership(c *fiber.Ctx) (errResult error) {
	var (
		projectID int
		err       error
	)
	projectID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	var (
		project *db.Project
		found   bool
	)

	project, found, err = db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var req projectMembershipRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}
	var subjectType db.ProjectMemberSubject

	subjectType = db.ProjectMemberSubjectUser
	if strings.EqualFold(req.SubjectType, "group") {
		subjectType = db.ProjectMemberSubjectGroup
	}
	var subjectID int

	subjectID, err = resolveProjectMembershipSubject(subjectType, req.SubjectID, req.SubjectRef, currentUserIsSiteAdmin(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if subjectType == db.ProjectMemberSubjectGroup {
		var group *db.CloudGroup
		group, found, err = db.GetCloudGroupByID(subjectID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group"})
		}
		if found && group.OwnerScopeType == db.RoleBindingScopeGlobal && group.GroupType == db.GroupTypeProject {
			group.OwnerScopeType = db.RoleBindingScopeProject
			group.OwnerScopeID = &projectID
			if err = db.UpdateCloudGroup(group); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scope group"})
			}
		}
	}

	if _, err = db.EnsureProjectMembership(projectID, subjectType, subjectID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.RoleID > 0 {
		if allowed, err = currentUserCanAssignRoleAtScope(c, req.RoleID, db.RoleBindingScopeProject, &projectID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		} else if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "role grants permissions outside your project access"})
		}
		if err = db.EnsureProjectMemberRoleBinding(projectID, subjectType, subjectID, req.RoleID); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	} else if strings.TrimSpace(req.ProjectRole) != "" {
		var role db.ProjectRole
		role = parseProjectRole(req.ProjectRole)
		var roleID int
		roleID, err = projectRoleID(role)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if allowed, err = currentUserCanAssignRoleAtScope(c, roleID, db.RoleBindingScopeProject, &projectID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		} else if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "role grants permissions outside your project access"})
		}
		if err = db.EnsureProjectMemberAccessRole(projectID, subjectType, subjectID, role); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}
	var memberships []*db.ProjectMembership

	memberships, err = db.ProjectMembershipsForProject(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	for _, membership := range memberships {
		if membership.SubjectType == subjectType && membership.SubjectID == subjectID {
			var items []fiber.Map
			items, err = projectMembershipResponse([]*db.ProjectMembership{membership})
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership subject"})
			}
			return c.Status(fiber.StatusCreated).JSON(items[0])
		}
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
}

// patchProjectMembership updates project membership access.
func patchProjectMembership(c *fiber.Ctx) (errResult error) {
	var (
		projectID int
		err       error
	)
	projectID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	var membershipID int
	membershipID, err = strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}
	var (
		project *db.Project
		found   bool
	)

	project, found, err = db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var membership *db.ProjectMembership

	membership, err = db.ProjectMemberships.Select(membershipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	if membership == nil || membership.ProjectID != projectID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
	}

	var req projectMembershipRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership request"})
	}
	if req.RoleID > 0 {
		if allowed, err = currentUserCanAssignRoleAtScope(c, req.RoleID, db.RoleBindingScopeProject, &projectID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		} else if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "role grants permissions outside your project access"})
		}
		if err = db.EnsureProjectMemberRoleBinding(projectID, membership.SubjectType, membership.SubjectID, req.RoleID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update membership access"})
		}
	} else {
		var role db.ProjectRole
		role = parseProjectRole(req.ProjectRole)
		var roleID int
		roleID, err = projectRoleID(role)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if allowed, err = currentUserCanAssignRoleAtScope(c, roleID, db.RoleBindingScopeProject, &projectID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		} else if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "role grants permissions outside your project access"})
		}
		if err = db.EnsureProjectMemberAccessRole(projectID, membership.SubjectType, membership.SubjectID, role); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update membership access"})
		}
	}
	var items []fiber.Map
	items, err = projectMembershipResponse([]*db.ProjectMembership{membership})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	return c.JSON(items[0])
}

// deleteProjectMembership removes project membership and project access grants.
func deleteProjectMembership(c *fiber.Ctx) (errResult error) {
	var (
		projectID int
		err       error
	)
	projectID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	var membershipID int
	membershipID, err = strconv.Atoi(c.Params("membershipID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid membership id"})
	}
	var (
		project *db.Project
		found   bool
	)

	project, found, err = db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var membership *db.ProjectMembership

	membership, err = db.ProjectMemberships.Select(membershipID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load membership"})
	}
	if membership == nil || membership.ProjectID != projectID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
	}

	if err = db.RemoveProjectMemberAccessRoles(projectID, membership.SubjectType, membership.SubjectID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove membership access"})
	}
	if err = db.RemoveProjectMembership(membershipID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove membership"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// deleteProjectBySlug deletes a project when the current user can manage it.
func deleteProjectBySlug(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		found   bool
		err     error
	)
	project, found, err = db.GetProjectBySlug(c.Params("slug"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	if err = db.DeleteProject(project.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete project"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// resolveProjectMembershipSubject resolves a user or group reference for project membership.
func resolveProjectMembershipSubject(subjectType db.ProjectMemberSubject, subjectID int, subjectRef string, allowLDAPGroupSync bool) (countResult int, errResult error) {
	if subjectID > 0 {
		return subjectID, nil
	}

	subjectRef = strings.TrimSpace(subjectRef)
	if subjectRef == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "subject is required")
	}

	if subjectType == db.ProjectMemberSubjectGroup {
		var (
			group *db.CloudGroup
			found bool
			err   error
		)
		group, found, err = findCloudGroupByRef(subjectRef)
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
	var (
		user  *db.User
		found bool
		err   error
	)

	user, found, err = syncOrFindUser(subjectRef)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "user was not found in local users or IPA")
	}

	return user.ID, nil
}

// syncOrFindCloudGroup finds a cloud group locally or imports it from LDAP.
func syncOrFindCloudGroup(ref string) (cloudGroupResult *db.CloudGroup, okResult bool, errResult error) {
	var (
		group *db.CloudGroup
		found bool
		err   error
	)
	group, found, err = findCloudGroupByRef(ref)
	if err != nil || found {
		return group, found, err
	}
	var ldapGroup *auth.LDAPGroup

	ldapGroup, found, err = auth.LookupGroup(ref)
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
		if err = db.UpdateCloudGroup(group); err != nil {
			return nil, false, err
		}
	}
	return group, true, nil
}

// findCloudGroupByRef finds a cloud group by slug, name, or external id.
func findCloudGroupByRef(ref string) (cloudGroupResult *db.CloudGroup, okResult bool, errResult error) {
	ref = strings.TrimSpace(ref)
	var refSlug string
	refSlug = slugForComparison(ref)
	var (
		groups []*db.CloudGroup
		err    error
	)
	groups, err = db.ListCloudGroups()
	if err != nil {
		return nil, false, err
	}
	for _, group := range groups {
		var candidates []string
		candidates = []string{group.Slug, group.Name, group.ExternalID}
		for _, candidate := range candidates {
			var clean string
			clean = strings.TrimSpace(candidate)
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

// syncUserFromLDAP imports a user from LDAP into the local database.
func syncUserFromLDAP(ref string) (userResult *db.User, okResult bool, errResult error) {
	var (
		ldapUser *auth.LDAPUser
		found    bool
		err      error
	)
	ldapUser, found, err = auth.LookupUser(ref)
	if err != nil || !found {
		return nil, false, err
	}
	var user *db.User

	user, _, err = db.EnsureUser(ldapUser.Username, ldapUser.DisplayName, ldapUser.Email, "ldap", ldapUser.Username)
	if err != nil {
		return nil, false, err
	}

	return user, true, nil
}

// getRoles lists roles available to role managers.
func getRoles(c *fiber.Ctx) (errResult error) {
	var (
		allowed bool
		err     error
	)
	allowed, err = requirePermission(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}
	var roles []*db.Role

	roles, err = db.Roles.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load roles"})
	}
	var items []fiber.Map
	items, err = roleResponse(roles)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role metadata"})
	}
	return c.JSON(items)
}

// getPermissions lists all known permissions for authenticated users.
func getPermissions(c *fiber.Ctx) (errResult error) {
	if currentDBUser(c) == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	var (
		permissions []*db.Permission
		err         error
	)
	permissions, err = db.Permissions.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load permissions"})
	}
	return c.JSON(permissions)
}

// postCreateRole creates a role in a permitted owner scope.
func postCreateRole(c *fiber.Ctx) (errResult error) {
	var req roleCreateRequest
	var err error
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role request"})
	}
	var scopeType db.RoleBindingScope

	scopeType = parseRoleBindingScope(req.ScopeType)
	if strings.TrimSpace(req.ScopeType) == "" {
		scopeType = db.RoleBindingScopeGlobal
	}
	var scopeID *int
	scopeID = req.ScopeID
	if scopeType == db.RoleBindingScopeGlobal {
		scopeID = nil
	} else if scopeID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role scope id is required"})
	}
	var allowed bool
	allowed, err = currentUserCanCreateRoleAtScope(c, scopeType, scopeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var createdBy *int
	var dbUser *db.User
	if dbUser = currentDBUser(c); dbUser != nil {
		createdBy = &dbUser.ID
	}
	var role *db.Role
	role, err = db.CreateRole(db.RoleCreateInput{
		Name:            req.Name,
		Description:     req.Description,
		OwnerScopeType:  scopeType,
		OwnerScopeID:    scopeID,
		CreatedByUserID: createdBy,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var items []fiber.Map
	items, err = roleResponse([]*db.Role{role})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role metadata"})
	}
	return c.Status(fiber.StatusCreated).JSON(items[0])
}

// patchRole updates mutable fields on a custom role.
func patchRole(c *fiber.Ctx) (errResult error) {
	var (
		role *db.Role
		err  error
	)
	role, err = roleFromParam(c)
	if err != nil {
		return roleParamError(c, err)
	}
	if role.IsSystemRole {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "system roles are managed by setup"})
	}
	var allowed bool
	allowed, err = currentUserCanManageRole(c, role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var req roleUpdateRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid role request"})
	}
	if req.Name != nil {
		var name string
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role name is required"})
		}
		var (
			existing *db.Role
			found    bool
			findErr  error
		)
		if existing, found, findErr = db.GetRoleByName(name); findErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to validate role name"})
		} else if found && existing.ID != role.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role name is already in use"})
		}
		role.Name = name
	}
	if req.Description != nil {
		role.Description = strings.TrimSpace(*req.Description)
	}

	if err = db.UpdateRole(role); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update role"})
	}
	var items []fiber.Map
	items, err = roleResponse([]*db.Role{role})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role metadata"})
	}
	return c.JSON(items[0])
}

// deleteRole deletes an unused custom role.
func deleteRole(c *fiber.Ctx) (errResult error) {
	var (
		role *db.Role
		err  error
	)
	role, err = roleFromParam(c)
	if err != nil {
		return roleParamError(c, err)
	}
	if role.IsSystemRole {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "system roles cannot be deleted"})
	}
	var allowed bool
	allowed, err = currentUserCanManageRole(c, role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var bindingCount int
	bindingCount, err = db.RoleBindingCountForRole(role.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to inspect role bindings"})
	}
	if bindingCount > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "role is still used by access grants"})
	}

	if err = db.DeleteRole(role.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete role"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// getRolePermissions lists permissions granted to a role.
func getRolePermissions(c *fiber.Ctx) (errResult error) {
	var (
		role *db.Role
		err  error
	)
	role, err = roleFromParam(c)
	if err != nil {
		return roleParamError(c, err)
	}
	var allowed bool
	allowed, err = currentUserCanManageRole(c, role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var items []fiber.Map
	items, err = rolePermissionResponse(role.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role permissions"})
	}
	return c.JSON(items)
}

// postCreateRolePermission grants a permission to a custom role.
func postCreateRolePermission(c *fiber.Ctx) (errResult error) {
	var (
		role *db.Role
		err  error
	)
	role, err = roleFromParam(c)
	if err != nil {
		return roleParamError(c, err)
	}
	if role.IsSystemRole {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "system role permissions are managed by setup"})
	}
	var allowed bool
	allowed, err = currentUserCanManageRole(c, role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var req rolePermissionRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid permission request"})
	}
	var permissionID int
	permissionID, err = resolvePermissionID(req.PermissionID, req.PermissionName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if allowed, err = currentUserCanGrantPermissionToRole(c, role, permissionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	} else if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission is outside your scoped access"})
	}
	if _, err = db.EnsureRolePermission(role.ID, permissionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to grant permission"})
	}
	var items []fiber.Map
	items, err = rolePermissionResponse(role.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role permissions"})
	}
	for _, item := range items {
		var (
			itemPermissionID int
			ok               bool
		)
		if itemPermissionID, ok = item["permission_id"].(int); ok && itemPermissionID == permissionID {
			return c.Status(fiber.StatusCreated).JSON(item)
		}
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role permission"})
}

// deleteRolePermission revokes a permission from a custom role.
func deleteRolePermission(c *fiber.Ctx) (errResult error) {
	var (
		role *db.Role
		err  error
	)
	role, err = roleFromParam(c)
	if err != nil {
		return roleParamError(c, err)
	}
	if role.IsSystemRole {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "system role permissions are managed by setup"})
	}
	var allowed bool
	allowed, err = currentUserCanManageRole(c, role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var permissionID int
	permissionID, err = strconv.Atoi(c.Params("permissionID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid permission id"})
	}
	if err = db.RemoveRolePermission(role.ID, permissionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke permission"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// groupResponse serializes a cloud group with counts and parent metadata.
func groupResponse(group *db.CloudGroup) (mapResult fiber.Map, errResult error) {
	var (
		memberships []*db.CloudGroupMembership
		err         error
	)
	memberships, err = db.CloudGroupMembershipsForGroup(group.ID)
	if err != nil {
		return nil, err
	}
	var roleBindings []*db.RoleBinding
	roleBindings, err = db.RoleBindingsForSubject(db.RoleBindingSubjectGroup, group.ID)
	if err != nil {
		return nil, err
	}
	var item fiber.Map

	item = fiber.Map{
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
		"archived_at":        group.ArchivedAt,
		"created_at":         group.CreatedAt,
		"updated_at":         group.UpdatedAt,
	}
	if group.ParentGroupID != nil {
		var (
			parent *db.CloudGroup
			found  bool
		)
		if parent, found, err = db.GetCloudGroupByID(*group.ParentGroupID); err != nil {
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

// roleResponse serializes roles with permission counts.
func roleResponse(roles []*db.Role) (itemsResult []fiber.Map, errResult error) {
	var items []fiber.Map
	items = make([]fiber.Map, 0, len(roles))
	for _, role := range roles {
		var (
			rolePermissions []*db.RolePermission
			err             error
		)
		rolePermissions, err = db.RolePermissionsForRole(role.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, fiber.Map{
			"id":                role.ID,
			"name":              role.Name,
			"description":       role.Description,
			"is_system_role":    role.IsSystemRole,
			"owner_scope_type":  role.OwnerScopeType,
			"owner_scope_label": roleBindingScopeLabel(role.OwnerScopeType),
			"owner_scope_id":    role.OwnerScopeID,
			"permission_count":  len(rolePermissions),
			"created_at":        role.CreatedAt,
			"updated_at":        role.UpdatedAt,
		})
	}
	return items, nil
}

// rolePermissionResponse serializes permissions granted to a role.
func rolePermissionResponse(roleID int) (itemsResult []fiber.Map, errResult error) {
	var (
		grants []*db.RolePermission
		err    error
	)
	grants, err = db.RolePermissionsForRole(roleID)
	if err != nil {
		return nil, err
	}
	var items []fiber.Map
	items = make([]fiber.Map, 0, len(grants))
	for _, grant := range grants {
		var permission *db.Permission
		permission, err = db.Permissions.Select(grant.PermissionID)
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

// roleBindingResponse serializes role bindings with role and subject metadata.
func roleBindingResponse(bindings []*db.RoleBinding) (itemsResult []fiber.Map, errResult error) {
	var items []fiber.Map
	items = make([]fiber.Map, 0, len(bindings))
	for _, binding := range bindings {
		var (
			role *db.Role
			err  error
		)
		role, err = db.Roles.Select(binding.RoleID)
		if err != nil {
			return nil, err
		}
		var item fiber.Map
		item = fiber.Map{
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
		var subject fiber.Map
		subject, err = roleBindingSubjectResponse(binding.SubjectType, binding.SubjectID)
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

// roleBindingSubjectResponse serializes the subject of a role binding.
func roleBindingSubjectResponse(subjectType db.RoleBindingSubject, subjectID int) (mapResult fiber.Map, errResult error) {
	if subjectType == db.RoleBindingSubjectGroup {
		var (
			group *db.CloudGroup
			found bool
			err   error
		)
		group, found, err = db.GetCloudGroupByID(subjectID)
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
	var (
		user  *db.User
		found bool
		err   error
	)

	user, found, err = db.GetUserByID(subjectID)
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

// resolveUserID resolves a user id or reference into a local user id.
func resolveUserID(userID int, userRef string) (countResult int, errResult error) {
	if userID > 0 {
		return userID, nil
	}
	var (
		user  *db.User
		found bool
		err   error
	)
	user, found, err = syncOrFindUser(userRef)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "user was not found in local users or IPA")
	}
	return user.ID, nil
}

// syncOrFindUser finds a local user or imports the user from LDAP.
func syncOrFindUser(ref string) (userResult *db.User, okResult bool, errResult error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, false, fiber.NewError(fiber.StatusBadRequest, "user is required")
	}
	var (
		users []*db.User
		err   error
	)
	users, err = db.ListUsers()
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

// resolveRoleID resolves a role id or name into a role id.
func resolveRoleID(roleID int, roleName string) (countResult int, errResult error) {
	if roleID > 0 {
		return roleID, nil
	}
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "role is required")
	}
	var (
		roles []*db.Role
		err   error
	)
	roles, err = db.Roles.SelectAll()
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

// resolvePermissionID resolves a permission id or name into a permission id.
func resolvePermissionID(permissionID int, permissionName string) (countResult int, errResult error) {
	if permissionID > 0 {
		return permissionID, nil
	}
	permissionName = strings.TrimSpace(permissionName)
	if permissionName == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "permission is required")
	}
	var (
		permission *db.Permission
		found      bool
		err        error
	)
	permission, found, err = db.GetPermissionByName(permissionName)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "permission was not found")
	}
	return permission.ID, nil
}

// roleFromParam loads a role from the route id parameter.
func roleFromParam(c *fiber.Ctx) (roleResult *db.Role, errResult error) {
	var (
		roleID int
		err    error
	)
	roleID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid role id")
	}
	var role *db.Role
	role, err = db.Roles.Select(roleID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to load role")
	}
	if role == nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "role not found")
	}
	return role, nil
}

// roleParamError writes a JSON response for role lookup errors.
func roleParamError(c *fiber.Ctx, err error) (errResult error) {
	var (
		fiberErr *fiber.Error
		ok       bool
	)
	if fiberErr, ok = err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role"})
}

// parseCloudGroupSyncSource converts a request value into a cloud group sync source.
func parseCloudGroupSyncSource(value string) (valueResult string) {
	if strings.EqualFold(strings.TrimSpace(value), db.CloudGroupSyncSourceLDAP) {
		return db.CloudGroupSyncSourceLDAP
	}
	return db.CloudGroupSyncSourceLocal
}

// cloudGroupSyncSource returns a normalized cloud group sync source.
func cloudGroupSyncSource(group *db.CloudGroup) (valueResult string) {
	if group != nil && group.SyncSource == db.CloudGroupSyncSourceLDAP {
		return db.CloudGroupSyncSourceLDAP
	}
	return db.CloudGroupSyncSourceLocal
}

// parseGroupType converts a request value into a group type.
func parseGroupType(value string) (groupTypeResult db.GroupType) {
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

// groupTypeLabel returns the API label for a group type.
func groupTypeLabel(value db.GroupType) (valueResult string) {
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

// parseMembershipRole converts a request value into a group membership role.
func parseMembershipRole(value string) (membershipRoleResult db.MembershipRole) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "manager":
		return db.MembershipRoleManager
	case "owner":
		return db.MembershipRoleOwner
	default:
		return db.MembershipRoleMember
	}
}

// parseProjectRole converts a request value into a project role.
func parseProjectRole(value string) (projectRoleResult db.ProjectRole) {
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

// membershipRoleLabel returns the API label for a membership role.
func membershipRoleLabel(value db.MembershipRole) (valueResult string) {
	switch value {
	case db.MembershipRoleManager:
		return "manager"
	case db.MembershipRoleOwner:
		return "owner"
	default:
		return "member"
	}
}

// roleBindingSubjectLabel returns the API label for a role binding subject.
func roleBindingSubjectLabel(value db.RoleBindingSubject) (valueResult string) {
	if value == db.RoleBindingSubjectUser {
		return "user"
	}
	return "group"
}

// parseRoleBindingScope converts a request value into a role binding scope.
func parseRoleBindingScope(value string) (roleBindingScopeResult db.RoleBindingScope) {
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

// roleBindingScopeLabel returns the API label for a role binding scope.
func roleBindingScopeLabel(value db.RoleBindingScope) (valueResult string) {
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

// projectMembershipResponse serializes project memberships with subject and role metadata.
func projectMembershipResponse(memberships []*db.ProjectMembership) (itemsResult []fiber.Map, errResult error) {
	var items []fiber.Map
	items = make([]fiber.Map, 0, len(memberships))
	for _, membership := range memberships {
		var (
			projectRole db.ProjectRole
			found       bool
			err         error
		)
		projectRole, found, err = db.ProjectMemberAccessRole(membership.ProjectID, membership.SubjectType, membership.SubjectID)
		if err != nil {
			return nil, err
		}
		if !found {
			projectRole = db.ProjectRoleViewer
		}
		var assignedRole *db.Role
		assignedRole, err = projectMemberAssignedRole(membership.ProjectID, membership.SubjectType, membership.SubjectID)
		if err != nil {
			return nil, err
		}
		var item fiber.Map
		item = fiber.Map{
			"id":                 membership.ID,
			"project_id":         membership.ProjectID,
			"subject_type":       membership.SubjectType,
			"subject_id":         membership.SubjectID,
			"project_role":       projectRole,
			"project_role_label": projectRoleLabel(projectRole),
			"created_at":         membership.CreatedAt,
		}
		if assignedRole != nil {
			item["access_role_id"] = assignedRole.ID
			item["access_role_name"] = assignedRole.Name
		}

		if membership.SubjectType == db.ProjectMemberSubjectGroup {
			var group *db.CloudGroup
			group, found, err = db.GetCloudGroupByID(membership.SubjectID)
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
			var user *db.User
			user, found, err = db.GetUserByID(membership.SubjectID)
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

// organizationMembershipResponse serializes organization memberships with subject and role metadata.
func organizationMembershipResponse(memberships []*db.OrganizationMembership) (itemsResult []fiber.Map, errResult error) {
	var items []fiber.Map
	items = make([]fiber.Map, 0, len(memberships))
	for _, membership := range memberships {
		var (
			assignedRole *db.Role
			err          error
		)
		assignedRole, err = organizationMemberAssignedRole(membership.OrganizationID, membership.SubjectType, membership.SubjectID)
		if err != nil {
			return nil, err
		}
		var item fiber.Map
		item = fiber.Map{
			"id":              membership.ID,
			"organization_id": membership.OrganizationID,
			"subject_type":    membership.SubjectType,
			"subject_id":      membership.SubjectID,
			"created_at":      membership.CreatedAt,
		}
		if assignedRole != nil {
			item["access_role_id"] = assignedRole.ID
			item["access_role_name"] = assignedRole.Name
		}
		if membership.SubjectType == db.ProjectMemberSubjectGroup {
			var (
				group *db.CloudGroup
				found bool
			)
			group, found, err = db.GetCloudGroupByID(membership.SubjectID)
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
			var (
				user  *db.User
				found bool
			)
			user, found, err = db.GetUserByID(membership.SubjectID)
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

// projectRoleLabel returns the API label for a project role.
func projectRoleLabel(value db.ProjectRole) (valueResult string) {
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

// projectRoleID resolves a built-in project role to its database role id.
func projectRoleID(role db.ProjectRole) (countResult int, errResult error) {
	var (
		dbRole *db.Role
		found  bool
		err    error
	)
	dbRole, found, err = db.GetRoleByName(db.ProjectRoleName(role))
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "project access role was not found")
	}
	return dbRole.ID, nil
}

// projectMemberAssignedRole returns the role assigned to a project member.
func projectMemberAssignedRole(projectID int, subjectType db.ProjectMemberSubject, subjectID int) (roleResult *db.Role, errResult error) {
	var (
		bindings []*db.RoleBinding
		err      error
	)
	bindings, err = db.RoleBindingsForSubject(db.RoleBindingSubject(subjectType), subjectID)
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		if binding.ScopeType != db.RoleBindingScopeProject || binding.ScopeID == nil || *binding.ScopeID != projectID {
			continue
		}
		var role *db.Role
		role, err = db.Roles.Select(binding.RoleID)
		if err != nil {
			return nil, err
		}
		if role != nil {
			return role, nil
		}
	}
	return nil, nil
}

// organizationMemberAssignedRole returns the role assigned to an organization member.
func organizationMemberAssignedRole(orgID int, subjectType db.ProjectMemberSubject, subjectID int) (roleResult *db.Role, errResult error) {
	var (
		bindings []*db.RoleBinding
		err      error
	)
	bindings, err = db.RoleBindingsForSubject(db.RoleBindingSubject(subjectType), subjectID)
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		if binding.ScopeType != db.RoleBindingScopeOrg || binding.ScopeID == nil || *binding.ScopeID != orgID {
			continue
		}
		var role *db.Role
		role, err = db.Roles.Select(binding.RoleID)
		if err != nil {
			return nil, err
		}
		if role != nil {
			return role, nil
		}
	}
	return nil, nil
}

// userLabel returns the best display label for a user.
func userLabel(user *db.User) (valueResult string) {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

// userMeta returns secondary display metadata for a user.
func userMeta(user *db.User) (valueResult string) {
	if user.Email != "" {
		return user.Username + " · " + user.Email
	}
	return user.Username
}

// currentUserCanViewProject reports whether the current user can view a project.
func currentUserCanViewProject(c *fiber.Ctx, project *db.Project) (okResult bool, errResult error) {
	var (
		allowed bool
		err     error
	)
	allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeGlobal, nil)
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
	var dbUser *db.User

	dbUser = currentDBUser(c)
	if dbUser == nil {
		return false, nil
	}
	var groupIDs []int

	groupIDs, err = db.CloudGroupIDsForUser(dbUser.ID)
	if err != nil {
		return false, err
	}
	var member bool

	if member, err = db.SubjectInProjectOrAncestor(project.ID, db.ProjectMemberSubjectUser, dbUser.ID); err != nil || member {
		return member, err
	}
	for _, groupID := range groupIDs {
		if member, err = db.SubjectInProjectOrAncestor(project.ID, db.ProjectMemberSubjectGroup, groupID); err != nil || member {
			return true, nil
		}
	}
	return false, nil
}

// currentUserCanManageProject reports whether the current user can manage a project.
func currentUserCanManageProject(c *fiber.Ctx, project *db.Project) (okResult bool, errResult error) {
	var (
		allowed bool
		err     error
	)
	allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || allowed {
		return allowed, err
	}
	allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeProject, &project.ID)
	if err != nil || allowed {
		return allowed, err
	}

	return false, nil
}

// currentUserCanManageGroup reports whether the current user can manage a cloud group.
func currentUserCanManageGroup(c *fiber.Ctx, groupID int) (okResult bool, errResult error) {
	var (
		allowed bool
		err     error
	)
	allowed, err = currentUserCan(c, db.PermissionGroupManage, db.RoleBindingScopeGroup, &groupID)
	if err != nil || allowed {
		return allowed, err
	}
	var (
		group *db.CloudGroup
		found bool
	)

	group, found, err = db.GetCloudGroupByID(groupID)
	if err != nil || !found {
		return false, err
	}
	if group.OwnerScopeType != db.RoleBindingScopeGlobal {
		allowed, err = currentUserCan(c, db.PermissionGroupManage, group.OwnerScopeType, group.OwnerScopeID)
		if err != nil || allowed {
			return allowed, err
		}
	}
	var dbUser *db.User

	dbUser = currentDBUser(c)
	if dbUser == nil {
		return false, nil
	}
	var membership *db.CloudGroupMembership

	membership, found, err = db.CloudGroupMembershipForUserAndGroup(dbUser.ID, groupID)
	if err != nil || !found {
		return false, err
	}

	return membership.MembershipRole == db.MembershipRoleManager || membership.MembershipRole == db.MembershipRoleOwner, nil
}

// currentUserCanBindRolesForGroup reports whether the current user may bind roles for a group.
func currentUserCanBindRolesForGroup(c *fiber.Ctx, groupID int, scopeType db.RoleBindingScope, scopeID *int) (okResult bool, errResult error) {
	if currentUserIsSiteAdmin(c) {
		return true, nil
	}

	if scopeType != db.RoleBindingScopeGroup || scopeID == nil || *scopeID != groupID {
		return false, nil
	}
	var (
		allowed bool
		err     error
	)

	allowed, err = currentUserCan(c, db.PermissionRoleManage, db.RoleBindingScopeGroup, &groupID)
	if err != nil || allowed {
		return allowed, err
	}
	var dbUser *db.User

	dbUser = currentDBUser(c)
	if dbUser == nil {
		return false, nil
	}
	var (
		membership *db.CloudGroupMembership
		found      bool
	)
	membership, found, err = db.CloudGroupMembershipForUserAndGroup(dbUser.ID, groupID)
	if err != nil || !found {
		return false, err
	}
	return membership.MembershipRole == db.MembershipRoleOwner, nil
}

// currentUserCanCreateRoleAtScope reports whether the current user may create roles in a scope.
func currentUserCanCreateRoleAtScope(c *fiber.Ctx, scopeType db.RoleBindingScope, scopeID *int) (okResult bool, errResult error) {
	if currentUserIsSiteAdmin(c) {
		return true, nil
	}
	switch scopeType {
	case db.RoleBindingScopeGlobal:
		return currentUserCan(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	case db.RoleBindingScopeOrg:
		if scopeID == nil {
			return false, nil
		}
		return currentUserCanManageOrganization(c, *scopeID)
	case db.RoleBindingScopeProject:
		if scopeID == nil {
			return false, nil
		}
		var (
			project *db.Project
			found   bool
			err     error
		)
		project, found, err = db.GetProjectByID(*scopeID)
		if err != nil || !found {
			return false, err
		}
		return currentUserCanManageProject(c, project)
	default:
		return false, nil
	}
}

// currentUserCanManageRole reports whether the current user may update a role.
func currentUserCanManageRole(c *fiber.Ctx, role *db.Role) (okResult bool, errResult error) {
	if currentUserIsSiteAdmin(c) {
		return true, nil
	}
	if role.IsSystemRole {
		return false, nil
	}
	return currentUserCanCreateRoleAtScope(c, role.OwnerScopeType, role.OwnerScopeID)
}

// currentUserCanGrantPermissionToRole reports whether the current user may grant a permission to a role.
func currentUserCanGrantPermissionToRole(c *fiber.Ctx, role *db.Role, permissionID int) (okResult bool, errResult error) {
	if currentUserIsSiteAdmin(c) {
		return true, nil
	}
	var (
		permission *db.Permission
		err        error
	)
	permission, err = db.Permissions.Select(permissionID)
	if err != nil || permission == nil {
		return false, err
	}
	var (
		key db.PermissionKey
		ok  bool
	)
	key, ok = db.PermissionKeyFromName(permission.Name)
	if !ok {
		return false, nil
	}
	return currentUserCan(c, key, role.OwnerScopeType, role.OwnerScopeID)
}

// currentUserCanAssignRoleAtScope reports whether the current user may assign a role at a scope.
func currentUserCanAssignRoleAtScope(c *fiber.Ctx, roleID int, scopeType db.RoleBindingScope, scopeID *int) (okResult bool, errResult error) {
	if currentUserIsSiteAdmin(c) {
		return true, nil
	}
	var (
		role *db.Role
		err  error
	)
	role, err = db.Roles.Select(roleID)
	if err != nil || role == nil {
		return false, err
	}
	if !roleOwnerAllowsScope(role, scopeType, scopeID) {
		return false, nil
	}
	var keys []db.PermissionKey
	keys, err = db.PermissionKeysForRole(roleID)
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		var allowed bool
		allowed, err = currentUserCan(c, key, scopeType, scopeID)
		if err != nil || !allowed {
			return allowed, err
		}
	}
	return true, nil
}

// roleOwnerAllowsScope reports whether a role's owner scope permits use at another scope.
func roleOwnerAllowsScope(role *db.Role, scopeType db.RoleBindingScope, scopeID *int) (okResult bool) {
	if role.OwnerScopeType == db.RoleBindingScopeGlobal {
		return true
	}
	if role.OwnerScopeType == scopeType {
		if role.OwnerScopeID == nil || scopeID == nil {
			return role.OwnerScopeID == nil && scopeID == nil
		}
		return *role.OwnerScopeID == *scopeID
	}
	if role.OwnerScopeType == db.RoleBindingScopeOrg {
		if role.OwnerScopeID == nil || scopeID == nil {
			return false
		}
		var ancestors []int
		var err error
		switch scopeType {
		case db.RoleBindingScopeOrg:
			ancestors, err = db.OrganizationAncestorIDs(*scopeID)
		case db.RoleBindingScopeProject:
			ancestors, err = db.ProjectOrganizationAncestorIDs(*scopeID)
		default:
			return false
		}
		if err != nil {
			return false
		}
		for _, ancestorID := range ancestors {
			if ancestorID == *role.OwnerScopeID {
				return true
			}
		}
	}
	return false
}

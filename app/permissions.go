package app

import (
	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

// currentUserCan checks whether the current request user has a scoped permission.
func currentUserCan(c *fiber.Ctx, permission db.PermissionKey, scopeType db.RoleBindingScope, scopeID *int) (allowed bool, err error) {
	var (
		authUser *auth.AuthUser = currentUser(c)
		dbUser   *db.User       = currentDBUser(c)
		groupIDs []int
	)

	if authUser == nil || dbUser == nil {
		return
	}

	if currentUserIsSiteAdmin(c) {
		allowed = true
		return
	}

	if groupIDs, err = db.CloudGroupIDsForUser(dbUser.ID); err != nil {
		return
	}

	allowed, err = db.HasPermission(db.PermissionCheck{
		UserID:     dbUser.ID,
		GroupIDs:   groupIDs,
		Permission: permission,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
	})
	return
}

// currentUserIsSiteAdmin reports whether the current user has global administrator access.
func currentUserIsSiteAdmin(c *fiber.Ctx) (allowed bool) {
	var (
		authUser      *auth.AuthUser = currentUser(c)
		dbUser        *db.User       = currentDBUser(c)
		groupIDs      []int
		bindings      []*db.RoleBinding
		roles         []*db.Role
		roleNamesByID map[int]string
		err           error
	)

	if authUser == nil || dbUser == nil {
		return
	}

	if authUser.Permissions() == auth.AuthPermsAdministrator || dbUser.IsSystemAdmin {
		allowed = true
		return
	}

	if groupIDs, err = db.CloudGroupIDsForUser(dbUser.ID); err != nil {
		return
	}
	if bindings, err = db.RoleBindingsForUserAndGroups(dbUser.ID, groupIDs); err != nil {
		return
	}
	if roles, err = db.RolesForBindings(bindings); err != nil {
		return
	}

	roleNamesByID = make(map[int]string, len(roles))
	for _, role := range roles {
		roleNamesByID[role.ID] = role.Name
	}
	for _, binding := range bindings {
		if binding.ScopeType == db.RoleBindingScopeGlobal && roleNamesByID[binding.RoleID] == db.DefaultLabAdminRoleName {
			allowed = true
			return
		}
	}

	return
}

// requirePermission enforces a scoped permission and writes a JSON error when denied.
func requirePermission(c *fiber.Ctx, permission db.PermissionKey, scopeType db.RoleBindingScope, scopeID *int) (allowed bool, err error) {
	if allowed, err = currentUserCan(c, permission, scopeType, scopeID); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "permission check failed",
		})
		return
	}
	if !allowed {
		err = c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "permission denied",
		})
		return
	}
	return
}

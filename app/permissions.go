package app

import (
	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func currentUserCan(c *fiber.Ctx, permission db.PermissionKey, scopeType db.RoleBindingScope, scopeID *int) (bool, error) {
	authUser := currentUser(c)
	dbUser := currentDBUser(c)
	if authUser == nil || dbUser == nil {
		return false, nil
	}

	if currentUserIsSiteAdmin(c) {
		return true, nil
	}

	groupIDs, err := db.CloudGroupIDsForUser(dbUser.ID)
	if err != nil {
		return false, err
	}

	return db.HasPermission(db.PermissionCheck{
		UserID:     dbUser.ID,
		GroupIDs:   groupIDs,
		Permission: permission,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
	})
}

func currentUserIsSiteAdmin(c *fiber.Ctx) bool {
	authUser := currentUser(c)
	dbUser := currentDBUser(c)
	if authUser == nil || dbUser == nil {
		return false
	}

	if authUser.Permissions() == auth.AuthPermsAdministrator || dbUser.IsSystemAdmin {
		return true
	}

	groupIDs, err := db.CloudGroupIDsForUser(dbUser.ID)
	if err != nil {
		return false
	}
	bindings, err := db.RoleBindingsForUserAndGroups(dbUser.ID, groupIDs)
	if err != nil {
		return false
	}
	roles, err := db.RolesForBindings(bindings)
	if err != nil {
		return false
	}

	roleNamesByID := make(map[int]string, len(roles))
	for _, role := range roles {
		roleNamesByID[role.ID] = role.Name
	}
	for _, binding := range bindings {
		if binding.ScopeType == db.RoleBindingScopeGlobal && roleNamesByID[binding.RoleID] == db.DefaultLabAdminRoleName {
			return true
		}
	}

	return false
}

func requirePermission(c *fiber.Ctx, permission db.PermissionKey, scopeType db.RoleBindingScope, scopeID *int) (bool, error) {
	allowed, err := currentUserCan(c, permission, scopeType, scopeID)
	if err != nil {
		return false, c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "permission check failed",
		})
	}
	if !allowed {
		return false, c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "permission denied",
		})
	}
	return true, nil
}

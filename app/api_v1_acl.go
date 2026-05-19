package app

import (
	"strings"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

// getGroupsForUser returns the directory groups visible for the requested user.
func getGroupsForUser(c *fiber.Ctx) (err error) {
	var (
		username   string
		groupnames []string
		dbUser     *db.User
		allowed    bool
	)

	if username = c.Params("username"); username == "" {
		err = fiber.NewError(fiber.StatusBadRequest, "username parameter is required")
		return
	}

	if dbUser = currentDBUser(c); dbUser == nil {
		err = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
		return
	}

	if !strings.EqualFold(username, dbUser.Username) {
		if allowed, err = requirePermission(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil); err != nil || !allowed {
			return
		}
	}

	if groupnames, err = db.GroupsForUser(username); err != nil {
		return
	}

	err = c.JSON(groupnames)
	return
}

// getUsersForGroup returns the directory users visible for the requested group.
func getUsersForGroup(c *fiber.Ctx) (err error) {
	var (
		groupname string
		usernames []string
		allowed   bool
	)

	if groupname = c.Params("groupname"); groupname == "" {
		err = fiber.NewError(fiber.StatusBadRequest, "groupname parameter is required")
		return
	}

	if allowed, err = requirePermission(c, db.PermissionGroupManage, db.RoleBindingScopeGlobal, nil); err != nil || !allowed {
		return
	}

	if usernames, err = db.UsersForGroup(groupname); err != nil {
		return
	}

	err = c.JSON(usernames)
	return
}

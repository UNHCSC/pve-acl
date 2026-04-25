package app

import (
	"strings"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func getGroupsForUser(c *fiber.Ctx) (err error) {
	var (
		username   string
		groupnames []string
	)

	if username = c.Params("username"); username == "" {
		err = fiber.NewError(fiber.StatusBadRequest, "username parameter is required")
		return
	}

	dbUser := currentDBUser(c)
	if dbUser == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	if !strings.EqualFold(username, dbUser.Username) {
		allowed, allowErr := requirePermission(c, "user.manage", db.RoleBindingScopeGlobal, nil)
		if allowErr != nil || !allowed {
			return allowErr
		}
	}

	if groupnames, err = db.GroupsForUser(username); err != nil {
		return
	}

	err = c.JSON(groupnames)
	return
}

func getUsersForGroup(c *fiber.Ctx) (err error) {
	var (
		groupname string
		usernames []string
	)

	if groupname = c.Params("groupname"); groupname == "" {
		err = fiber.NewError(fiber.StatusBadRequest, "groupname parameter is required")
		return
	}

	allowed, allowErr := requirePermission(c, "group.manage", db.RoleBindingScopeGlobal, nil)
	if allowErr != nil || !allowed {
		return allowErr
	}

	if usernames, err = db.UsersForGroup(groupname); err != nil {
		return
	}

	err = c.JSON(usernames)
	return
}

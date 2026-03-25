package app

import (
	"github.com/UNHCSC/pve-acl/db"
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

	if usernames, err = db.UsersForGroup(groupname); err != nil {
		return
	}

	err = c.JSON(usernames)
	return
}

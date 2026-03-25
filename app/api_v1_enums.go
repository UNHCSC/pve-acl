package app

import (
	"github.com/UNHCSC/pve-acl/db"
	"github.com/gofiber/fiber/v2"
)

func getAssetTypes(c *fiber.Ctx) (err error) {
	err = c.JSON(db.AssetTypes)
	return
}

func getAssetTypesReverse(c *fiber.Ctx) (err error) {
	err = c.JSON(db.AssetTypesReverse)
	return
}

func getAssetPermissions(c *fiber.Ctx) (err error) {
	err = c.JSON(db.AssetPermissions)
	return
}

func getAssetPermissionsReverse(c *fiber.Ctx) (err error) {
	err = c.JSON(db.AssetPermissionsReverse)
	return
}

func getManagementPermissions(c *fiber.Ctx) (err error) {
	err = c.JSON(db.ManagementPermissions)
	return
}

func getManagementPermissionsReverse(c *fiber.Ctx) (err error) {
	err = c.JSON(db.ManagementPermissionsReverse)
	return
}

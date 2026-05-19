package app

import (
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

// getAssetTypes returns the asset type enum map.
func getAssetTypes(c *fiber.Ctx) (err error) {
	err = c.JSON(db.AssetTypes)
	return
}

// getAssetTypesReverse returns the reverse asset type enum map.
func getAssetTypesReverse(c *fiber.Ctx) (err error) {
	err = c.JSON(db.AssetTypesReverse)
	return
}

// getAssetPermissions returns the asset permission enum map.
func getAssetPermissions(c *fiber.Ctx) (err error) {
	err = c.JSON(db.AssetPermissions)
	return
}

// getAssetPermissionsReverse returns the reverse asset permission enum map.
func getAssetPermissionsReverse(c *fiber.Ctx) (err error) {
	err = c.JSON(db.AssetPermissionsReverse)
	return
}

// getManagementPermissions returns the management permission enum map.
func getManagementPermissions(c *fiber.Ctx) (err error) {
	err = c.JSON(db.ManagementPermissions)
	return
}

// getManagementPermissionsReverse returns the reverse management permission enum map.
func getManagementPermissionsReverse(c *fiber.Ctx) (err error) {
	err = c.JSON(db.ManagementPermissionsReverse)
	return
}

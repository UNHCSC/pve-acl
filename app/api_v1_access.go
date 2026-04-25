package app

import (
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func getAccessData(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, "role.manage", db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	groups, err := db.CloudGroups.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load groups"})
	}
	groupItems := make([]fiber.Map, 0, len(groups))
	for _, group := range groups {
		item, itemErr := groupResponse(group)
		if itemErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load group metadata"})
		}
		groupItems = append(groupItems, item)
	}

	roles, err := db.Roles.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load roles"})
	}
	roleItems, err := roleResponse(roles)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role metadata"})
	}

	permissions, err := db.Permissions.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load permissions"})
	}

	roleBindings, err := db.RoleBindings.SelectAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role bindings"})
	}
	roleBindingItems, err := roleBindingResponse(roleBindings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load role binding metadata"})
	}

	return c.JSON(fiber.Map{
		"groups":       groupItems,
		"roles":        roleItems,
		"permissions":  permissions,
		"roleBindings": roleBindingItems,
	})
}

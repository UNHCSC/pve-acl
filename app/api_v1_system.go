package app

import (
	"github.com/UNHCSC/proxman/db"
	"github.com/gofiber/fiber/v2"
)

func getSystemSummary(c *fiber.Ctx) error {
	dbUser := currentDBUser(c)
	if dbUser == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
		})
	}

	counts, err := systemCounts()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load system summary",
		})
	}

	groupIDs, err := db.CloudGroupIDsForUser(dbUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load current groups",
		})
	}

	canCreateProjects, err := currentUserCanCreateProjects(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check project permissions",
		})
	}
	canManageUsers, err := currentUserCan(c, "user.manage", db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check user permissions",
		})
	}
	canManageGroups, err := currentUserCan(c, "group.manage", db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check group permissions",
		})
	}
	canManageRoles, err := currentUserCan(c, "role.manage", db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check role permissions",
		})
	}
	canManageOrgs, err := currentUserCan(c, "org.manage", db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check organization permissions",
		})
	}

	return c.JSON(fiber.Map{
		"counts": counts,
		"currentUser": fiber.Map{
			"id":          dbUser.ID,
			"username":    dbUser.Username,
			"displayName": dbUser.DisplayName,
			"email":       dbUser.Email,
			"authSource":  dbUser.AuthSource,
			"groupCount":  len(groupIDs),
			"isSiteAdmin": currentUserIsSiteAdmin(c),
		},
		"capabilities": fiber.Map{
			"canCreateProjects": canCreateProjects,
			"canManageUsers":    canManageUsers,
			"canManageGroups":   canManageGroups,
			"canManageRoles":    canManageRoles,
			"canManageOrgs":     canManageOrgs,
		},
	})
}

func systemCounts() (fiber.Map, error) {
	counts := fiber.Map{}

	for key, countFn := range map[string]func() (int64, error){
		"users":         db.Users.Count,
		"groups":        db.CloudGroups.Count,
		"organizations": db.Organizations.Count,
		"projects":      db.Projects.Count,
		"roles":         db.Roles.Count,
		"permissions":   db.Permissions.Count,
		"roleBindings":  db.RoleBindings.Count,
		"auditEvents":   db.AuditEvents.Count,
	} {
		count, err := countFn()
		if err != nil {
			return nil, err
		}
		counts[key] = count
	}

	return counts, nil
}

func currentUserCanCreateProjects(c *fiber.Ctx) (bool, error) {
	allowed, err := currentUserCan(c, "project.manage", db.RoleBindingScopeGlobal, nil)
	if err != nil || allowed {
		return allowed, err
	}

	orgs, err := db.ListOrganizations()
	if err != nil {
		return false, err
	}
	for _, org := range orgs {
		allowed, err = currentUserCan(c, "project.manage", db.RoleBindingScopeOrg, &org.ID)
		if err != nil || allowed {
			return allowed, err
		}
	}
	return false, nil
}

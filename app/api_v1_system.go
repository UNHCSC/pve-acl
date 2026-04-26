package app

import (
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func getSystemSummary(c *fiber.Ctx) error {
	dbUser := currentDBUser(c)
	if dbUser == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
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
	canManageUsers, err := currentUserCan(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check user permissions",
		})
	}
	canManageGroups, err := currentUserCan(c, db.PermissionGroupManage, db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check group permissions",
		})
	}
	canManageRoles, err := currentUserCan(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check role permissions",
		})
	}
	canManageOrgs, err := currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check organization permissions",
		})
	}
	counts, err := systemCounts(c, fiber.Map{
		"canManageUsers":  canManageUsers,
		"canManageGroups": canManageGroups,
		"canManageRoles":  canManageRoles,
		"canManageOrgs":   canManageOrgs,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load system summary",
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
			"canViewUsers":      canManageUsers,
		},
	})
}

func systemCounts(c *fiber.Ctx, capabilities fiber.Map) (fiber.Map, error) {
	counts := fiber.Map{}

	projectCount, orgCount, err := visibleDirectoryCounts(c)
	if err != nil {
		return nil, err
	}
	counts["projects"] = projectCount
	counts["organizations"] = orgCount

	counts["users"] = int64(0)
	if capabilities["canManageUsers"] == true {
		if counts["users"], err = db.Users.Count(); err != nil {
			return nil, err
		}
	}

	counts["groups"] = int64(0)
	if capabilities["canManageGroups"] == true {
		if counts["groups"], err = db.CloudGroups.Count(); err != nil {
			return nil, err
		}
	}

	counts["roles"] = int64(0)
	counts["permissions"] = int64(0)
	counts["roleBindings"] = int64(0)
	if capabilities["canManageRoles"] == true {
		if counts["roles"], err = db.Roles.Count(); err != nil {
			return nil, err
		}
		if counts["permissions"], err = db.Permissions.Count(); err != nil {
			return nil, err
		}
		if counts["roleBindings"], err = db.RoleBindings.Count(); err != nil {
			return nil, err
		}
	}

	counts["auditEvents"] = int64(0)
	if currentUserIsSiteAdmin(c) {
		if counts["auditEvents"], err = db.AuditEvents.Count(); err != nil {
			return nil, err
		}
	}

	return counts, nil
}

func visibleDirectoryCounts(c *fiber.Ctx) (int64, int64, error) {
	orgs, err := db.ListOrganizations()
	if err != nil {
		return 0, 0, err
	}
	projects, err := db.ListProjects()
	if err != nil {
		return 0, 0, err
	}

	visibleProjects := make([]fiber.Map, 0, len(projects))
	for _, project := range projects {
		allowed, allowErr := currentUserCanViewProject(c, project)
		if allowErr != nil {
			return 0, 0, allowErr
		}
		if allowed {
			visibleProjects = append(visibleProjects, fiber.Map{"organization_id": project.OrganizationID})
		}
	}

	visibleOrgs, err := visibleOrganizationIDs(c, orgs, visibleProjects)
	if err != nil {
		return 0, 0, err
	}
	return int64(len(visibleProjects)), int64(len(visibleOrgs)), nil
}

func currentUserCanCreateProjects(c *fiber.Ctx) (bool, error) {
	allowed, err := currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || allowed {
		return allowed, err
	}

	orgs, err := db.ListOrganizations()
	if err != nil {
		return false, err
	}
	for _, org := range orgs {
		allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeOrg, &org.ID)
		if err != nil || allowed {
			return allowed, err
		}
	}
	return false, nil
}

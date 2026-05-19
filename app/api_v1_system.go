package app

import (
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

// getSystemSummary returns dashboard counts and capabilities for the current user.
func getSystemSummary(c *fiber.Ctx) (err error) {
	var (
		dbUser            *db.User = currentDBUser(c)
		groupIDs          []int
		canCreateProjects bool
		canManageUsers    bool
		canManageGroups   bool
		canManageRoles    bool
		canManageOrgs     bool
		counts            fiber.Map
	)

	if dbUser == nil {
		err = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
		})
		return
	}

	if groupIDs, err = db.CloudGroupIDsForUser(dbUser.ID); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load current groups",
		})
		return
	}

	if canCreateProjects, err = currentUserCanCreateProjects(c); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check project permissions",
		})
		return
	}
	if canManageUsers, err = currentUserCan(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check user permissions",
		})
		return
	}
	if canManageGroups, err = currentUserCan(c, db.PermissionGroupManage, db.RoleBindingScopeGlobal, nil); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check group permissions",
		})
		return
	}
	if canManageRoles, err = currentUserCan(c, db.PermissionRoleManage, db.RoleBindingScopeGlobal, nil); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check role permissions",
		})
		return
	}
	if canManageOrgs, err = currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeGlobal, nil); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check organization permissions",
		})
		return
	}
	if counts, err = systemCounts(c, fiber.Map{
		"canManageUsers":  canManageUsers,
		"canManageGroups": canManageGroups,
		"canManageRoles":  canManageRoles,
		"canManageOrgs":   canManageOrgs,
	}); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load system summary",
		})
		return
	}

	err = c.JSON(fiber.Map{
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
	return
}

// systemCounts builds visible dashboard counts based on current user capabilities.
func systemCounts(c *fiber.Ctx, capabilities fiber.Map) (counts fiber.Map, err error) {
	var (
		projectCount int64
		orgCount     int64
	)

	counts = fiber.Map{}

	if projectCount, orgCount, err = visibleDirectoryCounts(c); err != nil {
		return
	}
	counts["projects"] = projectCount
	counts["organizations"] = orgCount

	counts["users"] = int64(0)
	if capabilities["canManageUsers"] == true {
		if counts["users"], err = db.Users.Count(); err != nil {
			return
		}
	}

	counts["groups"] = int64(0)
	if capabilities["canManageGroups"] == true {
		if counts["groups"], err = db.CloudGroups.Count(); err != nil {
			return
		}
	}

	counts["roles"] = int64(0)
	counts["permissions"] = int64(0)
	counts["roleBindings"] = int64(0)
	if capabilities["canManageRoles"] == true {
		if counts["roles"], err = db.Roles.Count(); err != nil {
			return
		}
		if counts["permissions"], err = db.Permissions.Count(); err != nil {
			return
		}
		if counts["roleBindings"], err = db.RoleBindings.Count(); err != nil {
			return
		}
	}

	counts["auditEvents"] = int64(0)
	if currentUserIsSiteAdmin(c) {
		if counts["auditEvents"], err = db.AuditEvents.Count(); err != nil {
			return
		}
	}

	return
}

// visibleDirectoryCounts counts organizations and projects visible to the current user.
func visibleDirectoryCounts(c *fiber.Ctx) (projectCount int64, orgCount int64, err error) {
	var (
		orgs            []*db.Organization
		projects        []*db.Project
		visibleProjects []fiber.Map
		visibleOrgs     map[int]bool
		allowed         bool
		allowErr        error
	)

	if orgs, err = db.ListOrganizations(); err != nil {
		return
	}
	if projects, err = db.ListProjects(); err != nil {
		return
	}

	visibleProjects = make([]fiber.Map, 0, len(projects))
	for _, project := range projects {
		if allowed, allowErr = currentUserCanViewProject(c, project); allowErr != nil {
			err = allowErr
			return
		}
		if allowed {
			visibleProjects = append(visibleProjects, fiber.Map{"organization_id": project.OrganizationID})
		}
	}

	if visibleOrgs, err = visibleOrganizationIDs(c, orgs, visibleProjects); err != nil {
		return
	}
	projectCount = int64(len(visibleProjects))
	orgCount = int64(len(visibleOrgs))
	return
}

// currentUserCanCreateProjects reports whether the current user can create any project.
func currentUserCanCreateProjects(c *fiber.Ctx) (allowed bool, err error) {
	var orgs []*db.Organization

	allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || allowed {
		return
	}

	if orgs, err = db.ListOrganizations(); err != nil {
		return
	}
	for _, org := range orgs {
		allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeOrg, &org.ID)
		if err != nil || allowed {
			return
		}
	}
	return
}

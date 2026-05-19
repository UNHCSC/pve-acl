package app

import (
	"strings"

	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

// getCurrentUser returns profile and permission details for the current user.
func getCurrentUser(c *fiber.Ctx) (err error) {
	var (
		authUser *auth.AuthUser = currentUser(c)
		dbUser   *db.User       = currentDBUser(c)
	)

	if authUser == nil || dbUser == nil {
		err = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
		})
		return
	}

	err = c.JSON(fiber.Map{
		"id":          dbUser.ID,
		"username":    dbUser.Username,
		"displayName": dbUser.DisplayName,
		"email":       dbUser.Email,
		"authSource":  dbUser.AuthSource,
		"permissions": authUser.Permissions().String(),
		"isSiteAdmin": currentUserIsSiteAdmin(c),
	})
	return
}

// getCurrentUserAccess returns group, role, and binding data for the current user.
func getCurrentUserAccess(c *fiber.Ctx) (err error) {
	var (
		dbUser       *db.User = currentDBUser(c)
		groups       []*db.CloudGroup
		groupIDs     []int
		roleBindings []*db.RoleBinding
		roles        []*db.Role
	)

	if dbUser == nil {
		err = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
		})
		return
	}

	if groups, err = db.CloudGroupsForUser(dbUser.ID); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load current groups",
		})
		return
	}

	groupIDs = make([]int, len(groups))
	for index, group := range groups {
		groupIDs[index] = group.ID
	}

	if roleBindings, err = db.RoleBindingsForUserAndGroups(dbUser.ID, groupIDs); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load current role bindings",
		})
		return
	}

	if roles, err = db.RolesForBindings(roleBindings); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load current roles",
		})
		return
	}

	err = c.JSON(fiber.Map{
		"groups":       groups,
		"roles":        roles,
		"roleBindings": roleBindings,
		"isSiteAdmin":  currentUserIsSiteAdmin(c),
	})
	return
}

// getResolveUser resolves a local or LDAP user by username, email, or display name.
func getResolveUser(c *fiber.Ctx) (err error) {
	var (
		query    string = strings.TrimSpace(c.Query("query"))
		dbUser   *db.User
		allowed  bool
		users    []*db.User
		ldapUser *auth.LDAPUser
		found    bool
		user     *db.User
	)

	if query == "" {
		err = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "query is required"})
		return
	}

	if dbUser = currentDBUser(c); dbUser == nil {
		err = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
		return
	}
	if !strings.EqualFold(query, dbUser.Username) && !strings.EqualFold(query, dbUser.Email) && !strings.EqualFold(query, dbUser.DisplayName) {
		if allowed, err = requirePermission(c, db.PermissionUserManage, db.RoleBindingScopeGlobal, nil); err != nil || !allowed {
			return
		}
	}

	if users, err = db.ListUsers(); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load users"})
		return
	}
	for _, user := range users {
		if strings.EqualFold(user.Username, query) || strings.EqualFold(user.Email, query) || strings.EqualFold(user.DisplayName, query) {
			err = c.JSON(fiber.Map{"user": user, "source": "local"})
			return
		}
	}

	if ldapUser, found, err = auth.LookupUser(query); err != nil || !found {
		err = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user was not found in local users or IPA"})
		return
	}

	if user, _, err = db.EnsureUser(ldapUser.Username, ldapUser.DisplayName, ldapUser.Email, "ldap", ldapUser.Username); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to sync IPA user"})
		return
	}

	err = c.JSON(fiber.Map{"user": user, "source": "ipa"})
	return
}

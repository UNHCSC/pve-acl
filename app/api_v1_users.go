package app

import (
	"strings"

	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

func getCurrentUser(c *fiber.Ctx) error {
	authUser := currentUser(c)
	dbUser := currentDBUser(c)
	if authUser == nil || dbUser == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
		})
	}

	return c.JSON(fiber.Map{
		"id":          dbUser.ID,
		"username":    dbUser.Username,
		"displayName": dbUser.DisplayName,
		"email":       dbUser.Email,
		"authSource":  dbUser.AuthSource,
		"permissions": authUser.Permissions().String(),
		"isSiteAdmin": currentUserIsSiteAdmin(c),
	})
}

func getCurrentUserAccess(c *fiber.Ctx) error {
	dbUser := currentDBUser(c)
	if dbUser == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
		})
	}

	groups, err := db.CloudGroupsForUser(dbUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load current groups",
		})
	}

	groupIDs := make([]int, len(groups))
	for i, group := range groups {
		groupIDs[i] = group.ID
	}

	roleBindings, err := db.RoleBindingsForUserAndGroups(dbUser.ID, groupIDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load current role bindings",
		})
	}

	roles, err := db.RolesForBindings(roleBindings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load current roles",
		})
	}

	return c.JSON(fiber.Map{
		"groups":       groups,
		"roles":        roles,
		"roleBindings": roleBindings,
		"isSiteAdmin":  currentUserIsSiteAdmin(c),
	})
}

func getResolveUser(c *fiber.Ctx) error {
	query := strings.TrimSpace(c.Query("query"))
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "query is required"})
	}

	dbUser := currentDBUser(c)
	if dbUser == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
	}
	if !strings.EqualFold(query, dbUser.Username) && !strings.EqualFold(query, dbUser.Email) && !strings.EqualFold(query, dbUser.DisplayName) {
		allowed, err := requirePermission(c, "user.manage", db.RoleBindingScopeGlobal, nil)
		if err != nil || !allowed {
			return err
		}
	}

	users, err := db.ListUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load users"})
	}
	for _, user := range users {
		if strings.EqualFold(user.Username, query) || strings.EqualFold(user.Email, query) || strings.EqualFold(user.DisplayName, query) {
			return c.JSON(fiber.Map{"user": user, "source": "local"})
		}
	}

	ldapUser, found, err := auth.LookupUser(query)
	if err != nil || !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user was not found in local users or IPA"})
	}

	user, _, err := db.EnsureUser(ldapUser.Username, ldapUser.DisplayName, ldapUser.Email, "ldap", ldapUser.Username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to sync IPA user"})
	}

	return c.JSON(fiber.Map{"user": user, "source": "ipa"})
}

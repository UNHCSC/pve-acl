package app

import (
	"strings"

	"github.com/UNHCSC/organesson/auth"
	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

const (
	currentUserLocalKey   = "current_user"
	currentDBUserLocalKey = "current_db_user"
)

// securityHeaders applies defensive browser headers to every response.
func securityHeaders(c *fiber.Ctx) (err error) {
	c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Referrer-Policy", "same-origin")
	err = c.Next()
	return
}

// requireAPIAuth validates API sessions and stores current auth context on Fiber locals.
func requireAPIAuth(c *fiber.Ctx) (err error) {
	var (
		user   *auth.AuthUser = auth.IsAuthenticated(c, jwtSigningKey)
		dbUser *db.User
	)

	if user == nil {
		err = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
		})
		return
	}

	if dbUser, err = ensureDBUserForAuthUser(user); err != nil {
		err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to sync authenticated user",
		})
		return
	}

	auth.RefreshToken(user)
	c.Locals(currentUserLocalKey, user)
	c.Locals(currentDBUserLocalKey, dbUser)
	err = c.Next()
	return
}

// currentUser returns the authenticated user stored on the request context.
func currentUser(c *fiber.Ctx) (authUserResult *auth.AuthUser) {
	var user *auth.AuthUser
	user, _ = c.Locals(currentUserLocalKey).(*auth.AuthUser)
	return user
}

// currentDBUser returns the database user stored on the request context.
func currentDBUser(c *fiber.Ctx) (userResult *db.User) {
	var user *db.User
	user, _ = c.Locals(currentDBUserLocalKey).(*db.User)
	return user
}

// ensureDBUserForAuthUser synchronizes the authenticated identity into the local database.
func ensureDBUserForAuthUser(user *auth.AuthUser) (dbUser *db.User, err error) {
	var (
		displayName string
		email       string
		authSource  string
		value       string
		groups      []string
		lookupErr   error
	)

	authSource = "local"
	if user.LDAPConn != nil {
		authSource = "ldap"
		if value, lookupErr = user.LDAPConn.DisplayName(); lookupErr == nil {
			displayName = value
		}
		if value, lookupErr = user.LDAPConn.Email(); lookupErr == nil {
			email = value
		}
	}

	dbUser, _, err = db.EnsureUser(user.Username, displayName, email, authSource, user.Username)
	if err != nil {
		return
	}

	if user.LDAPConn != nil {
		if groups, lookupErr = user.LDAPConn.Groups(); lookupErr == nil {
			if err = syncLDAPCloudGroupMemberships(dbUser, groups); err != nil {
				return
			}
		}
	}

	return
}

// syncLDAPCloudGroupMemberships updates opt-in cloud group memberships from LDAP groups.
func syncLDAPCloudGroupMemberships(dbUser *db.User, ldapGroups []string) (err error) {
	if dbUser == nil {
		return
	}

	var (
		groupNames map[string]bool = make(map[string]bool, len(ldapGroups))
		groupSlugs map[string]bool = make(map[string]bool, len(ldapGroups))
		clean      string
	)

	for _, groupName := range ldapGroups {
		clean = strings.TrimSpace(groupName)
		if clean == "" {
			continue
		}
		groupNames[strings.ToLower(clean)] = true
		groupSlugs[slugForComparison(clean)] = true
	}

	var (
		seenAdminGroups map[string]bool = make(map[string]bool)
		adminSlug       string
		group           *db.CloudGroup
	)

	for _, adminGroupName := range append([]string{db.DefaultAdminGroupSlug}, config.Config.LDAP.AdminGroups...) {
		adminGroupName = strings.TrimSpace(adminGroupName)
		if adminGroupName == "" {
			continue
		}
		adminSlug = slugForComparison(adminGroupName)
		if adminSlug == "" || seenAdminGroups[adminSlug] {
			continue
		}
		seenAdminGroups[adminSlug] = true

		if group, _, err = db.EnsureCloudGroup(adminGroupName, "", db.GroupTypeAdmin); err != nil {
			return
		}
		if group.SyncSource != db.CloudGroupSyncSourceLDAP || group.ExternalID != adminGroupName || !group.SyncMembership || group.GroupType != db.GroupTypeAdmin {
			group.SyncSource = db.CloudGroupSyncSourceLDAP
			group.ExternalID = adminGroupName
			group.SyncMembership = true
			group.GroupType = db.GroupTypeAdmin
			if err = db.UpdateCloudGroup(group); err != nil {
				return
			}
		}
		if ldapGroupMapsContain(groupNames, groupSlugs, adminGroupName, group) {
			if _, err = db.EnsureCloudGroupMembership(dbUser.ID, group.ID, db.MembershipRoleMember); err != nil {
				return
			}
		}
	}

	var groups []*db.CloudGroup
	if groups, err = db.ListCloudGroups(); err != nil {
		return
	}
	for _, group := range groups {
		if group.SyncSource != db.CloudGroupSyncSourceLDAP || !group.SyncMembership {
			continue
		}
		if ldapGroupMapsContain(groupNames, groupSlugs, group.ExternalID, group) {
			if _, err = db.EnsureCloudGroupMembership(dbUser.ID, group.ID, db.MembershipRoleMember); err != nil {
				return
			}
		}
	}

	return
}

// ldapGroupMapsContain reports whether a cloud group matches LDAP group names or slugs.
func ldapGroupMapsContain(names, slugs map[string]bool, externalID string, group *db.CloudGroup) (okResult bool) {
	var candidates []string
	candidates = []string{externalID}
	if group != nil {
		candidates = append(candidates, group.Name, group.Slug)
	}
	for _, candidate := range candidates {
		var clean string
		clean = strings.TrimSpace(candidate)
		if clean == "" {
			continue
		}
		if names[strings.ToLower(clean)] || slugs[slugForComparison(clean)] {
			return true
		}
	}
	return false
}

// slugForComparison normalizes a value into a lowercase dashed comparison key.
func slugForComparison(value string) (valueResult string) {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	var lastDash bool
	lastDash = false
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			out.WriteRune(character)
			lastDash = false
		case !lastDash:
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

package app

import (
	"strings"

	"github.com/UNHCSC/proxman/auth"
	"github.com/UNHCSC/proxman/config"
	"github.com/UNHCSC/proxman/db"
	"github.com/gofiber/fiber/v2"
)

const currentUserLocalKey = "current_user"
const currentDBUserLocalKey = "current_db_user"

func requireAPIAuth(c *fiber.Ctx) error {
	user := auth.IsAuthenticated(c, jwtSigningKey)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "authentication required",
		})
	}

	dbUser, err := ensureDBUserForAuthUser(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to sync authenticated user",
		})
	}

	auth.RefreshToken(user)
	c.Locals(currentUserLocalKey, user)
	c.Locals(currentDBUserLocalKey, dbUser)
	return c.Next()
}

func currentUser(c *fiber.Ctx) *auth.AuthUser {
	user, _ := c.Locals(currentUserLocalKey).(*auth.AuthUser)
	return user
}

func currentDBUser(c *fiber.Ctx) *db.User {
	user, _ := c.Locals(currentDBUserLocalKey).(*db.User)
	return user
}

func ensureDBUserForAuthUser(user *auth.AuthUser) (*db.User, error) {
	var displayName, email, authSource string

	authSource = "local"
	if user.LDAPConn != nil {
		authSource = "ldap"
		if value, err := user.LDAPConn.DisplayName(); err == nil {
			displayName = value
		}
		if value, err := user.LDAPConn.Email(); err == nil {
			email = value
		}
	}

	dbUser, _, err := db.EnsureUser(user.Username, displayName, email, authSource, user.Username)
	if err != nil {
		return nil, err
	}

	if user.LDAPConn != nil {
		if groups, err := user.LDAPConn.Groups(); err == nil {
			if err := syncLDAPCloudGroupMemberships(dbUser, groups); err != nil {
				return nil, err
			}
		}
	}

	return dbUser, nil
}

func syncLDAPCloudGroupMemberships(dbUser *db.User, ldapGroups []string) error {
	if dbUser == nil {
		return nil
	}

	groupNames := make(map[string]bool, len(ldapGroups))
	groupSlugs := make(map[string]bool, len(ldapGroups))
	for _, groupName := range ldapGroups {
		clean := strings.TrimSpace(groupName)
		if clean == "" {
			continue
		}
		groupNames[strings.ToLower(clean)] = true
		groupSlugs[slugForComparison(clean)] = true
	}

	seenAdminGroups := make(map[string]bool)
	for _, adminGroupName := range append([]string{db.DefaultAdminGroupSlug}, config.Config.LDAP.AdminGroups...) {
		adminGroupName = strings.TrimSpace(adminGroupName)
		if adminGroupName == "" {
			continue
		}
		adminSlug := slugForComparison(adminGroupName)
		if adminSlug == "" || seenAdminGroups[adminSlug] {
			continue
		}
		seenAdminGroups[adminSlug] = true

		group, _, err := db.EnsureCloudGroup(adminGroupName, "", db.GroupTypeAdmin)
		if err != nil {
			return err
		}
		if group.SyncSource != db.CloudGroupSyncSourceLDAP || group.ExternalID != adminGroupName || !group.SyncMembership || group.GroupType != db.GroupTypeAdmin {
			group.SyncSource = db.CloudGroupSyncSourceLDAP
			group.ExternalID = adminGroupName
			group.SyncMembership = true
			group.GroupType = db.GroupTypeAdmin
			if err := db.UpdateCloudGroup(group); err != nil {
				return err
			}
		}
		if ldapGroupMapsContain(groupNames, groupSlugs, adminGroupName, group) {
			if _, err := db.EnsureCloudGroupMembership(dbUser.ID, group.ID, db.MembershipRoleMember); err != nil {
				return err
			}
		}
	}

	groups, err := db.ListCloudGroups()
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.SyncSource != db.CloudGroupSyncSourceLDAP || !group.SyncMembership {
			continue
		}
		if ldapGroupMapsContain(groupNames, groupSlugs, group.ExternalID, group) {
			if _, err := db.EnsureCloudGroupMembership(dbUser.ID, group.ID, db.MembershipRoleMember); err != nil {
				return err
			}
		}
	}

	return nil
}

func ldapGroupMapsContain(names, slugs map[string]bool, externalID string, group *db.CloudGroup) bool {
	candidates := []string{externalID}
	if group != nil {
		candidates = append(candidates, group.Name, group.Slug)
	}
	for _, candidate := range candidates {
		clean := strings.TrimSpace(candidate)
		if clean == "" {
			continue
		}
		if names[strings.ToLower(clean)] || slugs[slugForComparison(clean)] {
			return true
		}
	}
	return false
}

func slugForComparison(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			lastDash = false
		case !lastDash:
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

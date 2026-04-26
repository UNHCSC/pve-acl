package db

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/z46-dev/gomysql"
)

const (
	DefaultRootOrganizationName     = "Lab"
	DefaultRootOrganizationSlug     = "lab"
	DefaultAdminGroupName           = "Admins"
	DefaultAdminGroupSlug           = "admins"
	DefaultLabAdminRoleName         = "LabAdmin"
	DefaultProjectViewerRoleName    = "ProjectViewer"
	DefaultProjectOperatorRoleName  = "ProjectOperator"
	DefaultProjectDeveloperRoleName = "ProjectDeveloper"
	DefaultProjectManagerRoleName   = "ProjectManager"
	DefaultProjectOwnerRoleName     = "ProjectOwner"
	DefaultResourceUserRoleName     = "ResourceUser"
)

var SystemRolePermissions = map[string][]PermissionKey{
	DefaultLabAdminRoleName: CorePermissions,
	DefaultProjectViewerRoleName: {
		PermissionVMRead,
		PermissionCTRead,
		PermissionNetworkRead,
		PermissionTemplateRead,
		PermissionQuotaRead,
	},
	DefaultProjectOperatorRoleName: {
		PermissionVMRead,
		PermissionVMStart,
		PermissionVMStop,
		PermissionVMReboot,
		PermissionVMConsole,
		PermissionCTRead,
		PermissionCTStart,
		PermissionCTStop,
		PermissionCTConsole,
		PermissionNetworkRead,
		PermissionTemplateRead,
		PermissionQuotaRead,
	},
	DefaultProjectDeveloperRoleName: {
		PermissionVMRead,
		PermissionVMCreate,
		PermissionVMStart,
		PermissionVMStop,
		PermissionVMReboot,
		PermissionVMConsole,
		PermissionVMSnapshot,
		PermissionVMClone,
		PermissionCTRead,
		PermissionCTCreate,
		PermissionCTStart,
		PermissionCTStop,
		PermissionCTConsole,
		PermissionNetworkRead,
		PermissionNetworkAttach,
		PermissionTemplateRead,
		PermissionTemplateClone,
		PermissionQuotaRead,
	},
	DefaultProjectManagerRoleName: {
		PermissionVMRead,
		PermissionVMCreate,
		PermissionVMStart,
		PermissionVMStop,
		PermissionVMReboot,
		PermissionVMConsole,
		PermissionVMSnapshot,
		PermissionVMClone,
		PermissionVMResize,
		PermissionVMReconfigure,
		PermissionVMDelete,
		PermissionCTRead,
		PermissionCTCreate,
		PermissionCTStart,
		PermissionCTStop,
		PermissionCTConsole,
		PermissionCTDelete,
		PermissionNetworkRead,
		PermissionNetworkCreate,
		PermissionNetworkUpdate,
		PermissionNetworkDelete,
		PermissionNetworkAttach,
		PermissionTemplateRead,
		PermissionTemplateClone,
		PermissionQuotaRead,
		PermissionQuotaUpdate,
		PermissionGroupManage,
		PermissionProjectManage,
	},
	DefaultProjectOwnerRoleName: CorePermissions,
	DefaultResourceUserRoleName: {
		PermissionVMRead,
		PermissionVMStart,
		PermissionVMStop,
		PermissionVMReboot,
		PermissionVMConsole,
		PermissionCTRead,
		PermissionCTStart,
		PermissionCTStop,
		PermissionCTConsole,
	},
}

func EnsureInitialSetup() (err error) {
	var now = time.Now().UTC()

	org, createdOrg, err := ensureOrganization(DefaultRootOrganizationName, DefaultRootOrganizationSlug, now)
	if err != nil {
		return
	}
	if createdOrg {
		if err = writeSetupAudit("setup.organization.create", "organization", org.ID, now); err != nil {
			return
		}
	}

	for _, permissionKey := range CorePermissions {
		permission, createdPermission, setupErr := ensurePermission(permissionKey.String())
		if setupErr != nil {
			err = setupErr
			return
		}
		if createdPermission {
			if err = writeSetupAudit("setup.permission.create", "permission", permission.ID, now); err != nil {
				return
			}
		}

	}

	rolesByName := map[string]*Role{}
	for roleName, permissionKeys := range SystemRolePermissions {
		role, createdRole, setupErr := ensureRole(roleName, systemRoleDescription(roleName), true, now)
		if setupErr != nil {
			err = setupErr
			return
		}
		rolesByName[roleName] = role
		if createdRole {
			if err = writeSetupAudit("setup.role.create", "role", role.ID, now); err != nil {
				return
			}
		}
		for _, permissionKey := range permissionKeys {
			permissionName := permissionKey.String()
			permission, found, findErr := findPermissionByName(permissionName)
			if findErr != nil {
				err = findErr
				return
			}
			if !found {
				err = fmt.Errorf("permission %q was not found", permissionName)
				return
			}
			createdBinding, setupErr := ensureRolePermission(role.ID, permission.ID)
			if setupErr != nil {
				err = setupErr
				return
			}
			if createdBinding {
				if err = writeSetupAudit("setup.role_permission.create", "role_permission", role.ID, now); err != nil {
					return
				}
			}
		}
	}

	for _, adminGroup := range initialAdminGroups() {
		group, createdGroup, setupErr := ensureCloudGroup(displayNameFromSlug(adminGroup.Slug), adminGroup.Slug, GroupTypeAdmin, now)
		if setupErr != nil {
			err = setupErr
			return
		}
		if group.SyncSource != CloudGroupSyncSourceLDAP || group.ExternalID != adminGroup.ExternalID || !group.SyncMembership || group.GroupType != GroupTypeAdmin {
			group.SyncSource = CloudGroupSyncSourceLDAP
			group.ExternalID = adminGroup.ExternalID
			group.SyncMembership = true
			group.GroupType = GroupTypeAdmin
			group.UpdatedAt = now
			if err = CloudGroups.Update(group); err != nil {
				return
			}
		}
		if createdGroup {
			if err = writeSetupAudit("setup.group.create", "group", group.ID, now); err != nil {
				return
			}
		}

		createdRoleBinding, setupErr := ensureRoleBinding(rolesByName[DefaultLabAdminRoleName].ID, RoleBindingSubjectGroup, group.ID, RoleBindingScopeGlobal, nil, now)
		if setupErr != nil {
			err = setupErr
			return
		}
		if createdRoleBinding {
			if err = writeSetupAudit("setup.role_binding.create", "group", group.ID, now); err != nil {
				return
			}
		}
	}

	_ = org
	return
}

func systemRoleDescription(name string) string {
	switch name {
	case DefaultLabAdminRoleName:
		return "Full system-level authority across Organesson Cloud."
	case DefaultProjectViewerRoleName:
		return "Read-only project access."
	case DefaultProjectOperatorRoleName:
		return "Read and lifecycle operation access for project resources."
	case DefaultProjectDeveloperRoleName:
		return "Create and operate normal project resources."
	case DefaultProjectManagerRoleName:
		return "Manage project resources, membership shortcuts, and quota."
	case DefaultProjectOwnerRoleName:
		return "Full project-level authority."
	case DefaultResourceUserRoleName:
		return "Semi-private VM/container console and power access on individual resources."
	default:
		return "System role."
	}
}

func ensureOrganization(name, slug string, now time.Time) (*Organization, bool, error) {
	if existing, found, err := findOrganizationBySlug(slug); err != nil || found {
		return existing, false, err
	}

	uuid, err := randomUUID()
	if err != nil {
		return nil, false, err
	}

	org := &Organization{
		UUID:      uuid,
		Name:      name,
		Slug:      slug,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := Organizations.Insert(org); err != nil {
		return nil, false, err
	}

	return org, true, nil
}

func ensureCloudGroup(name, slug string, groupType GroupType, now time.Time) (*CloudGroup, bool, error) {
	if existing, found, err := findCloudGroupBySlug(slug); err != nil || found {
		return existing, false, err
	}

	uuid, err := randomUUID()
	if err != nil {
		return nil, false, err
	}

	group := &CloudGroup{
		UUID:           uuid,
		Name:           name,
		Slug:           slug,
		GroupType:      groupType,
		OwnerScopeType: RoleBindingScopeGlobal,
		SyncSource:     CloudGroupSyncSourceLocal,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := CloudGroups.Insert(group); err != nil {
		return nil, false, err
	}

	return group, true, nil
}

func EnsureCloudGroup(name, slug string, groupType GroupType) (*CloudGroup, bool, error) {
	if slug == "" {
		slug = slugify(name)
	}
	if name == "" {
		name = displayNameFromSlug(slug)
	}
	return ensureCloudGroup(name, slug, groupType, time.Now().UTC())
}

func EnsureCloudGroupMembership(userID, groupID int, role MembershipRole) (bool, error) {
	filter := gomysql.NewFilter().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("user_id"), gomysql.OpEqual, userID).
		And().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("group_id"), gomysql.OpEqual, groupID)

	existing, err := CloudGroupMemberships.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		membership := existing[0]
		if membership.MembershipRole != role {
			membership.MembershipRole = role
			return false, CloudGroupMemberships.Update(membership)
		}
		return false, nil
	}

	return true, CloudGroupMemberships.Insert(&CloudGroupMembership{
		UserID:         userID,
		GroupID:        groupID,
		MembershipRole: role,
		CreatedAt:      time.Now().UTC(),
	})
}

func ensureRole(name, description string, isSystemRole bool, now time.Time) (*Role, bool, error) {
	if existing, found, err := findRoleByName(name); err != nil || found {
		return existing, false, err
	}

	role := &Role{
		Name:         name,
		Description:  description,
		IsSystemRole: isSystemRole,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := Roles.Insert(role); err != nil {
		return nil, false, err
	}

	return role, true, nil
}

func ensurePermission(name string) (*Permission, bool, error) {
	if existing, found, err := findPermissionByName(name); err != nil || found {
		return existing, false, err
	}

	permission := &Permission{Name: name}
	if err := Permissions.Insert(permission); err != nil {
		return nil, false, err
	}

	return permission, true, nil
}

func ensureRolePermission(roleID, permissionID int) (bool, error) {
	filter := gomysql.NewFilter().
		KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID).
		And().
		KeyCmp(RolePermissions.FieldBySQLName("permission_id"), gomysql.OpEqual, permissionID)

	count, err := RolePermissions.CountWithFilter(filter)
	if err != nil || count > 0 {
		return false, err
	}

	return true, RolePermissions.Insert(&RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	})
}

func ensureRoleBinding(roleID int, subjectType RoleBindingSubject, subjectID int, scopeType RoleBindingScope, scopeID *int, now time.Time) (bool, error) {
	filter := gomysql.NewFilter().
		KeyCmp(RoleBindings.FieldBySQLName("role_id"), gomysql.OpEqual, roleID).
		And().
		KeyCmp(RoleBindings.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(RoleBindings.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID).
		And().
		KeyCmp(RoleBindings.FieldBySQLName("scope_type"), gomysql.OpEqual, scopeType)

	if scopeID == nil {
		filter = filter.And().KeyCmp(RoleBindings.FieldBySQLName("scope_id"), gomysql.OpIsNull, nil)
	} else {
		filter = filter.And().KeyCmp(RoleBindings.FieldBySQLName("scope_id"), gomysql.OpEqual, scopeID)
	}

	count, err := RoleBindings.CountWithFilter(filter)
	if err != nil || count > 0 {
		return false, err
	}

	return true, RoleBindings.Insert(&RoleBinding{
		RoleID:      roleID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		ScopeType:   scopeType,
		ScopeID:     scopeID,
		CreatedAt:   now,
	})
}

func findOrganizationBySlug(slug string) (*Organization, bool, error) {
	return findOneByStringField(Organizations, Organizations.FieldBySQLName("slug"), slug)
}

func findCloudGroupBySlug(slug string) (*CloudGroup, bool, error) {
	return findOneByStringField(CloudGroups, CloudGroups.FieldBySQLName("slug"), slug)
}

func findRoleByName(name string) (*Role, bool, error) {
	return findOneByStringField(Roles, Roles.FieldBySQLName("name"), name)
}

func findPermissionByName(name string) (*Permission, bool, error) {
	return findOneByStringField(Permissions, Permissions.FieldBySQLName("name"), name)
}

func findOneByStringField[T any](table *gomysql.RegisteredStruct[T], field *gomysql.RegisteredStructField, value string) (*T, bool, error) {
	items, err := table.SelectAllWithFilter(gomysql.NewFilter().KeyCmp(field, gomysql.OpEqual, value).Limit(1))
	if err != nil {
		return nil, false, err
	}
	if len(items) == 0 {
		return nil, false, nil
	}
	return items[0], true, nil
}

func writeSetupAudit(action, targetType string, targetID int, now time.Time) error {
	uuid, err := randomUUID()
	if err != nil {
		return err
	}

	return AuditEvents.Insert(&AuditEvent{
		UUID:       uuid,
		Action:     action,
		TargetType: targetType,
		TargetID:   &targetID,
		CreatedAt:  now,
	})
}

type initialAdminGroup struct {
	Slug       string
	ExternalID string
}

func initialAdminGroups() []initialAdminGroup {
	seen := map[string]bool{}
	groups := make([]initialAdminGroup, 0, len(config.Config.LDAP.AdminGroups)+1)

	for _, group := range append([]string{DefaultAdminGroupSlug}, config.Config.LDAP.AdminGroups...) {
		externalID := strings.TrimSpace(group)
		slug := slugify(externalID)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		groups = append(groups, initialAdminGroup{
			Slug:       slug,
			ExternalID: externalID,
		})
	}

	return groups
}

func displayNameFromSlug(slug string) string {
	if slug == DefaultAdminGroupSlug {
		return DefaultAdminGroupName
	}

	parts := strings.Split(slug, "-")
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

func slugify(value string) string {
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

func randomUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

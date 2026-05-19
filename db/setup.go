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

// EnsureInitialSetup seeds the required system records.
func EnsureInitialSetup() (err error) {
	var now = time.Now().UTC()
	var (
		org        *Organization
		createdOrg bool
	)

	org, createdOrg, err = ensureOrganization(DefaultRootOrganizationName, DefaultRootOrganizationSlug, now)
	if err != nil {
		return
	}
	if createdOrg {
		if err = writeSetupAudit("setup.organization.create", "organization", org.ID, now); err != nil {
			return
		}
	}

	for _, permissionKey := range CorePermissions {
		var (
			permission        *Permission
			createdPermission bool
			setupErr          error
		)

		permission, createdPermission, setupErr = ensurePermission(permissionKey.String())
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
	var rolesByName map[string]*Role

	rolesByName = map[string]*Role{}
	for roleName, permissionKeys := range SystemRolePermissions {
		var (
			role        *Role
			createdRole bool
			setupErr    error
		)

		role, createdRole, setupErr = ensureRole(roleName, systemRoleDescription(roleName), true, now)
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
			var permissionName string

			permissionName = permissionKey.String()
			var (
				permission *Permission
				found      bool
				findErr    error
			)

			permission, found, findErr = findPermissionByName(permissionName)
			if findErr != nil {
				err = findErr
				return
			}
			if !found {
				err = fmt.Errorf("permission %q was not found", permissionName)
				return
			}
			var (
				createdBinding bool
				setupErr       error
			)

			createdBinding, setupErr = ensureRolePermission(role.ID, permission.ID)
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
		var (
			group        *CloudGroup
			createdGroup bool
			setupErr     error
		)

		group, createdGroup, setupErr = ensureCloudGroup(displayNameFromSlug(adminGroup.Slug), adminGroup.Slug, GroupTypeAdmin, now)
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
		var createdRoleBinding bool

		createdRoleBinding, setupErr = ensureRoleBinding(rolesByName[DefaultLabAdminRoleName].ID, RoleBindingSubjectGroup, group.ID, RoleBindingScopeGlobal, nil, now)
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

func systemRoleDescription(name string) (valueResult string) {
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

func ensureOrganization(name, slug string, now time.Time) (organizationResult *Organization, okResult bool, errResult error) {
	{
		var (
			existing *Organization
			found    bool
			err      error
		)

		if existing, found, err = findOrganizationBySlug(slug); err != nil || found {
			return existing, false, err
		}
	}
	var (
		uuid string
		err  error
	)

	uuid, err = randomUUID()
	if err != nil {
		return nil, false, err
	}
	var org *Organization

	org = &Organization{
		UUID:      uuid,
		Name:      name,
		Slug:      slug,
		CreatedAt: now,
		UpdatedAt: now,
	}
	{
		var err error

		if err = Organizations.Insert(org); err != nil {
			return nil, false, err
		}
	}

	return org, true, nil
}

func ensureCloudGroup(name, slug string, groupType GroupType, now time.Time) (cloudGroupResult *CloudGroup, okResult bool, errResult error) {
	{
		var (
			existing *CloudGroup
			found    bool
			err      error
		)

		if existing, found, err = findCloudGroupBySlug(slug); err != nil || found {
			return existing, false, err
		}
	}
	var (
		uuid string
		err  error
	)

	uuid, err = randomUUID()
	if err != nil {
		return nil, false, err
	}
	var group *CloudGroup

	group = &CloudGroup{
		UUID:           uuid,
		Name:           name,
		Slug:           slug,
		GroupType:      groupType,
		OwnerScopeType: RoleBindingScopeGlobal,
		SyncSource:     CloudGroupSyncSourceLocal,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	{
		var err error

		if err = CloudGroups.Insert(group); err != nil {
			return nil, false, err
		}
	}

	return group, true, nil
}

// EnsureCloudGroup ensures cloud group exists.
func EnsureCloudGroup(name, slug string, groupType GroupType) (cloudGroupResult *CloudGroup, okResult bool, errResult error) {
	if slug == "" {
		slug = slugify(name)
	}
	if name == "" {
		name = displayNameFromSlug(slug)
	}
	return ensureCloudGroup(name, slug, groupType, time.Now().UTC())
}

// EnsureCloudGroupMembership ensures cloud group membership exists.
func EnsureCloudGroupMembership(userID, groupID int, role MembershipRole) (okResult bool, errResult error) {
	var filter *gomysql.Filter

	filter = gomysql.NewFilter().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("user_id"), gomysql.OpEqual, userID).
		And().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("group_id"), gomysql.OpEqual, groupID)
	var (
		existing []*CloudGroupMembership
		err      error
	)

	existing, err = CloudGroupMemberships.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		var membership *CloudGroupMembership

		membership = existing[0]
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

func ensureRole(name, description string, isSystemRole bool, now time.Time) (roleResult *Role, okResult bool, errResult error) {
	{
		var (
			existing *Role
			found    bool
			err      error
		)

		if existing, found, err = findRoleByName(name); err != nil || found {
			return existing, false, err
		}
	}
	var role *Role

	role = &Role{
		Name:         name,
		Description:  description,
		IsSystemRole: isSystemRole,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	{
		var err error

		if err = Roles.Insert(role); err != nil {
			return nil, false, err
		}
	}

	return role, true, nil
}

func ensurePermission(name string) (permissionResult *Permission, okResult bool, errResult error) {
	{
		var (
			existing *Permission
			found    bool
			err      error
		)

		if existing, found, err = findPermissionByName(name); err != nil || found {
			return existing, false, err
		}
	}
	var permission *Permission

	permission = &Permission{Name: name}
	{
		var err error

		if err = Permissions.Insert(permission); err != nil {
			return nil, false, err
		}
	}

	return permission, true, nil
}

func ensureRolePermission(roleID, permissionID int) (okResult bool, errResult error) {
	var filter *gomysql.Filter

	filter = gomysql.NewFilter().
		KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID).
		And().
		KeyCmp(RolePermissions.FieldBySQLName("permission_id"), gomysql.OpEqual, permissionID)
	var (
		count int64
		err   error
	)

	count, err = RolePermissions.CountWithFilter(filter)
	if err != nil || count > 0 {
		return false, err
	}

	return true, RolePermissions.Insert(&RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	})
}

func ensureRoleBinding(roleID int, subjectType RoleBindingSubject, subjectID int, scopeType RoleBindingScope, scopeID *int, now time.Time) (okResult bool, errResult error) {
	var filter *gomysql.Filter

	filter = gomysql.NewFilter().
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
	var (
		count int64
		err   error
	)

	count, err = RoleBindings.CountWithFilter(filter)
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

func findOrganizationBySlug(slug string) (organizationResult *Organization, okResult bool, errResult error) {
	return findOneByStringField(Organizations, Organizations.FieldBySQLName("slug"), slug)
}

func findCloudGroupBySlug(slug string) (cloudGroupResult *CloudGroup, okResult bool, errResult error) {
	return findOneByStringField(CloudGroups, CloudGroups.FieldBySQLName("slug"), slug)
}

func findRoleByName(name string) (roleResult *Role, okResult bool, errResult error) {
	return findOneByStringField(Roles, Roles.FieldBySQLName("name"), name)
}

func findPermissionByName(name string) (permissionResult *Permission, okResult bool, errResult error) {
	return findOneByStringField(Permissions, Permissions.FieldBySQLName("name"), name)
}

func findOneByStringField[T any](table *gomysql.RegisteredStruct[T], field *gomysql.RegisteredStructField, value string) (tResult *T, okResult bool, errResult error) {
	var (
		items []*T
		err   error
	)

	items, err = table.SelectAllWithFilter(gomysql.NewFilter().KeyCmp(field, gomysql.OpEqual, value).Limit(1))
	if err != nil {
		return nil, false, err
	}
	if len(items) == 0 {
		return nil, false, nil
	}
	return items[0], true, nil
}

func writeSetupAudit(action, targetType string, targetID int, now time.Time) (errResult error) {
	var (
		uuid string
		err  error
	)

	uuid, err = randomUUID()
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

func initialAdminGroups() (itemsResult []initialAdminGroup) {
	var seen map[string]bool

	seen = map[string]bool{}
	var groups []initialAdminGroup

	groups = make([]initialAdminGroup, 0, len(config.Config.LDAP.AdminGroups)+1)

	for _, group := range append([]string{DefaultAdminGroupSlug}, config.Config.LDAP.AdminGroups...) {
		var externalID string

		externalID = strings.TrimSpace(group)
		var slug string

		slug = slugify(externalID)
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

func displayNameFromSlug(slug string) (valueResult string) {
	if slug == DefaultAdminGroupSlug {
		return DefaultAdminGroupName
	}
	var parts []string

	parts = strings.Split(slug, "-")
	for index := range parts {
		if parts[index] == "" {
			continue
		}
		parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
	}
	return strings.Join(parts, " ")
}

func slugify(value string) (valueResult string) {
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

func randomUUID() (valueResult string, errResult error) {
	var b [16]byte
	{
		var err error

		if _, err = rand.Read(b[:]); err != nil {
			return "", err
		}
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

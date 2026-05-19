package db

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/UNHCSC/organesson/authz"
	"github.com/casbin/casbin/v2"
	"github.com/z46-dev/gomysql"
)

type (
	PermissionCheck struct {
		UserID     int
		GroupIDs   []int
		Permission PermissionKey
		ScopeType  RoleBindingScope
		ScopeID    *int
	}

	RoleCreateInput struct {
		Name            string
		Description     string
		IsSystemRole    bool
		OwnerScopeType  RoleBindingScope
		OwnerScopeID    *int
		CreatedByUserID *int
	}
)

// HasPermission checks whether a user has a permission in a scope.
func HasPermission(check PermissionCheck) (okResult bool, errResult error) {
	var (
		permission *Permission
		found      bool
		err        error
	)

	permission, found, err = findPermissionByName(check.Permission.String())
	if err != nil || !found {
		return false, err
	}
	var enforcer *casbin.Enforcer

	enforcer, err = authz.NewScopedEnforcer(scopeDomainMatches)
	if err != nil {
		return false, err
	}
	var bindings []*RoleBinding

	bindings, err = roleBindingsForSubject(RoleBindingSubjectUser, check.UserID)
	if err != nil {
		return false, err
	}

	for _, groupID := range check.GroupIDs {
		var (
			groupBindings []*RoleBinding
			groupErr      error
		)

		groupBindings, groupErr = roleBindingsForSubject(RoleBindingSubjectGroup, groupID)
		if groupErr != nil {
			return false, groupErr
		}
		bindings = append(bindings, groupBindings...)
	}

	for _, binding := range bindings {
		var domain string

		domain = scopeDomain(binding.ScopeType, binding.ScopeID)
		{
			var err error

			if _, err = enforcer.AddGroupingPolicy(userPrincipal(check.UserID), rolePrincipal(binding.RoleID), domain); err != nil {
				return false, err
			}
		}
		var (
			grants []*RolePermission
			err    error
		)

		grants, err = RolePermissionsForRole(binding.RoleID)
		if err != nil {
			return false, err
		}
		for _, grant := range grants {
			var (
				grantedPermission *Permission
				err               error
			)

			grantedPermission, err = Permissions.Select(grant.PermissionID)
			if err != nil {
				return false, err
			}
			if grantedPermission == nil {
				continue
			}
			{
				var err error

				if _, err = enforcer.AddPolicy(rolePrincipal(binding.RoleID), domain, grantedPermission.Name); err != nil {
					return false, err
				}
			}
		}
	}
	var allowed bool

	allowed, err = enforcer.Enforce(userPrincipal(check.UserID), scopeDomain(check.ScopeType, check.ScopeID), permission.Name)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func roleBindingsForSubject(subjectType RoleBindingSubject, subjectID int) (itemsResult []*RoleBinding, errResult error) {
	return RoleBindings.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(RoleBindings.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(RoleBindings.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID))
}

// RoleBindingsForSubject returns role bindings for a subject.
func RoleBindingsForSubject(subjectType RoleBindingSubject, subjectID int) (itemsResult []*RoleBinding, errResult error) {
	return roleBindingsForSubject(subjectType, subjectID)
}

// EnsureRole ensures role exists.
func EnsureRole(name, description string, isSystemRole bool) (roleResult *Role, okResult bool, errResult error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, false, fmt.Errorf("role name is required")
	}
	return ensureRole(name, description, isSystemRole, time.Now().UTC())
}

// CreateRole creates a role from input.
func CreateRole(input RoleCreateInput) (roleResult *Role, errResult error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" {
		return nil, fmt.Errorf("role name is required")
	}
	{
		var (
			existing *Role
			found    bool
			err      error
		)

		if existing, found, err = findRoleByName(input.Name); err != nil || found {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("role name %q already exists", existing.Name)
		}
	}
	var now time.Time

	now = time.Now().UTC()
	var role *Role

	role = &Role{
		Name:            input.Name,
		Description:     input.Description,
		IsSystemRole:    input.IsSystemRole,
		OwnerScopeType:  input.OwnerScopeType,
		OwnerScopeID:    input.OwnerScopeID,
		CreatedByUserID: input.CreatedByUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if role.OwnerScopeType == RoleBindingScopeGlobal {
		role.OwnerScopeID = nil
	}
	{
		var err error

		if err = Roles.Insert(role); err != nil {
			return nil, err
		}
	}
	return role, nil
}

// UpdateRole updates a role.
func UpdateRole(role *Role) (errResult error) {
	role.Name = strings.TrimSpace(role.Name)
	role.Description = strings.TrimSpace(role.Description)
	role.UpdatedAt = time.Now().UTC()
	return Roles.Update(role)
}

// DeleteRole deletes a role and its permission grants.
func DeleteRole(roleID int) (errResult error) {
	{
		var err error

		if _, err = RolePermissions.DeleteWithFilter(gomysql.NewFilter().
			KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID)); err != nil {
			return err
		}
	}
	return Roles.Delete(roleID)
}

// RoleBindingCountForRole returns how many bindings reference a role.
func RoleBindingCountForRole(roleID int) (countResult int, errResult error) {
	var (
		count int64
		err   error
	)

	count, err = RoleBindings.CountWithFilter(gomysql.NewFilter().
		KeyCmp(RoleBindings.FieldBySQLName("role_id"), gomysql.OpEqual, roleID))
	return int(count), err
}

// GetRoleByName returns role by name.
func GetRoleByName(name string) (roleResult *Role, okResult bool, errResult error) {
	return findRoleByName(strings.TrimSpace(name))
}

// GetPermissionByName returns permission by name.
func GetPermissionByName(name string) (permissionResult *Permission, okResult bool, errResult error) {
	return findPermissionByName(strings.TrimSpace(name))
}

// EnsureRolePermission ensures role permission exists.
func EnsureRolePermission(roleID, permissionID int) (okResult bool, errResult error) {
	return ensureRolePermission(roleID, permissionID)
}

// RemoveRolePermission removes role permission.
func RemoveRolePermission(roleID, permissionID int) (errResult error) {
	var err error

	_, err = RolePermissions.DeleteWithFilter(gomysql.NewFilter().
		KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID).
		And().
		KeyCmp(RolePermissions.FieldBySQLName("permission_id"), gomysql.OpEqual, permissionID))
	return err
}

// RolePermissionsForRole returns permission grants for a role.
func RolePermissionsForRole(roleID int) (itemsResult []*RolePermission, errResult error) {
	return RolePermissions.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID))
}

// PermissionKeysForRole returns permission keys granted to a role.
func PermissionKeysForRole(roleID int) (itemsResult []PermissionKey, errResult error) {
	var (
		grants []*RolePermission
		err    error
	)

	grants, err = RolePermissionsForRole(roleID)
	if err != nil {
		return nil, err
	}
	var keys []PermissionKey

	keys = make([]PermissionKey, 0, len(grants))
	for _, grant := range grants {
		var (
			permission *Permission
			err        error
		)

		permission, err = Permissions.Select(grant.PermissionID)
		if err != nil {
			return nil, err
		}
		if permission == nil {
			continue
		}
		var (
			key PermissionKey
			ok  bool
		)

		key, ok = PermissionKeyFromName(permission.Name)
		if !ok {
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// EnsureRoleBinding ensures role binding exists.
func EnsureRoleBinding(roleID int, subjectType RoleBindingSubject, subjectID int, scopeType RoleBindingScope, scopeID *int) (okResult bool, errResult error) {
	return ensureRoleBinding(roleID, subjectType, subjectID, scopeType, scopeID, time.Now().UTC())
}

// RemoveRoleBinding removes role binding.
func RemoveRoleBinding(roleBindingID int) (errResult error) {
	return RoleBindings.Delete(roleBindingID)
}

// RoleBindingsForUserAndGroups returns role bindings for a user and their groups.
func RoleBindingsForUserAndGroups(userID int, groupIDs []int) (itemsResult []*RoleBinding, errResult error) {
	var (
		bindings []*RoleBinding
		err      error
	)

	bindings, err = roleBindingsForSubject(RoleBindingSubjectUser, userID)
	if err != nil {
		return nil, err
	}

	for _, groupID := range groupIDs {
		var (
			groupBindings []*RoleBinding
			groupErr      error
		)

		groupBindings, groupErr = roleBindingsForSubject(RoleBindingSubjectGroup, groupID)
		if groupErr != nil {
			return nil, groupErr
		}
		bindings = append(bindings, groupBindings...)
	}

	return bindings, nil
}

// RolesForBindings returns distinct roles referenced by bindings.
func RolesForBindings(bindings []*RoleBinding) (itemsResult []*Role, errResult error) {
	var seen map[int]bool

	seen = make(map[int]bool)
	var roles []*Role

	roles = make([]*Role, 0, len(bindings))

	for _, binding := range bindings {
		if seen[binding.RoleID] {
			continue
		}
		seen[binding.RoleID] = true
		var (
			role *Role
			err  error
		)

		role, err = Roles.Select(binding.RoleID)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func projectBindingMatchesResource(bindingScopeID *int, requestedScopeID *int) (okResult bool) {
	if bindingScopeID == nil || requestedScopeID == nil {
		return false
	}
	var (
		resource *Resource
		err      error
	)

	resource, err = Resources.Select(*requestedScopeID)
	if err != nil || resource == nil {
		return false
	}
	return resource.ProjectID == *bindingScopeID
}

func scopeDomain(scopeType RoleBindingScope, scopeID *int) (valueResult string) {
	if scopeType == RoleBindingScopeGlobal {
		return "global"
	}
	if scopeID == nil {
		return roleBindingScopeName(scopeType) + ":"
	}
	return roleBindingScopeName(scopeType) + ":" + strconv.Itoa(*scopeID)
}

func scopeDomainMatches(policyDomain, requestDomain string) (okResult bool) {
	if policyDomain == "global" {
		return true
	}
	if policyDomain == requestDomain {
		return true
	}
	var (
		policyType RoleBindingScope
		policyID   int
		ok         bool
	)

	policyType, policyID, ok = parseScopeDomain(policyDomain)
	if !ok {
		return false
	}
	var (
		requestType RoleBindingScope
		requestID   int
	)

	requestType, requestID, ok = parseScopeDomain(requestDomain)
	if !ok {
		return false
	}

	if policyType == RoleBindingScopeOrg {
		var orgIDs []int
		var err error
		switch requestType {
		case RoleBindingScopeOrg:
			orgIDs, err = OrganizationAncestorIDs(requestID)
		case RoleBindingScopeProject:
			orgIDs, err = ProjectOrganizationAncestorIDs(requestID)
		case RoleBindingScopeResource:
			orgIDs, err = ResourceOrganizationAncestorIDs(requestID)
		default:
			return false
		}
		if err != nil {
			return false
		}
		for _, orgID := range orgIDs {
			if orgID == policyID {
				return true
			}
		}
		return false
	}

	if policyType == RoleBindingScopeProject && requestType == RoleBindingScopeResource {
		return projectBindingMatchesResource(&policyID, &requestID)
	}

	return false
}

func parseScopeDomain(value string) (roleBindingScopeResult RoleBindingScope, countResult int, okResult bool) {
	var (
		scopeName string
		rawID     string
		found     bool
	)

	scopeName, rawID, found = strings.Cut(value, ":")
	if !found || rawID == "" {
		return RoleBindingScopeGlobal, 0, false
	}
	var (
		scopeID int
		err     error
	)

	scopeID, err = strconv.Atoi(rawID)
	if err != nil {
		return RoleBindingScopeGlobal, 0, false
	}
	switch scopeName {
	case "org":
		return RoleBindingScopeOrg, scopeID, true
	case "project":
		return RoleBindingScopeProject, scopeID, true
	case "group":
		return RoleBindingScopeGroup, scopeID, true
	case "resource":
		return RoleBindingScopeResource, scopeID, true
	default:
		return RoleBindingScopeGlobal, 0, false
	}
}

func roleBindingScopeName(value RoleBindingScope) (valueResult string) {
	switch value {
	case RoleBindingScopeOrg:
		return "org"
	case RoleBindingScopeProject:
		return "project"
	case RoleBindingScopeGroup:
		return "group"
	case RoleBindingScopeResource:
		return "resource"
	default:
		return "global"
	}
}

func userPrincipal(userID int) (valueResult string) {
	return "user:" + strconv.Itoa(userID)
}

func rolePrincipal(roleID int) (valueResult string) {
	return "role:" + strconv.Itoa(roleID)
}

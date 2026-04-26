package db

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/UNHCSC/organesson/authz"
	"github.com/z46-dev/gomysql"
)

type PermissionCheck struct {
	UserID     int
	GroupIDs   []int
	Permission PermissionKey
	ScopeType  RoleBindingScope
	ScopeID    *int
}

type RoleCreateInput struct {
	Name            string
	Description     string
	IsSystemRole    bool
	OwnerScopeType  RoleBindingScope
	OwnerScopeID    *int
	CreatedByUserID *int
}

func HasPermission(check PermissionCheck) (bool, error) {
	permission, found, err := findPermissionByName(check.Permission.String())
	if err != nil || !found {
		return false, err
	}

	enforcer, err := authz.NewScopedEnforcer(scopeDomainMatches)
	if err != nil {
		return false, err
	}

	bindings, err := roleBindingsForSubject(RoleBindingSubjectUser, check.UserID)
	if err != nil {
		return false, err
	}

	for _, groupID := range check.GroupIDs {
		groupBindings, groupErr := roleBindingsForSubject(RoleBindingSubjectGroup, groupID)
		if groupErr != nil {
			return false, groupErr
		}
		bindings = append(bindings, groupBindings...)
	}

	for _, binding := range bindings {
		domain := scopeDomain(binding.ScopeType, binding.ScopeID)
		if _, err := enforcer.AddGroupingPolicy(userPrincipal(check.UserID), rolePrincipal(binding.RoleID), domain); err != nil {
			return false, err
		}
		grants, err := RolePermissionsForRole(binding.RoleID)
		if err != nil {
			return false, err
		}
		for _, grant := range grants {
			grantedPermission, err := Permissions.Select(grant.PermissionID)
			if err != nil {
				return false, err
			}
			if grantedPermission == nil {
				continue
			}
			if _, err := enforcer.AddPolicy(rolePrincipal(binding.RoleID), domain, grantedPermission.Name); err != nil {
				return false, err
			}
		}
	}

	allowed, err := enforcer.Enforce(userPrincipal(check.UserID), scopeDomain(check.ScopeType, check.ScopeID), permission.Name)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func roleBindingsForSubject(subjectType RoleBindingSubject, subjectID int) ([]*RoleBinding, error) {
	return RoleBindings.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(RoleBindings.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(RoleBindings.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID))
}

func RoleBindingsForSubject(subjectType RoleBindingSubject, subjectID int) ([]*RoleBinding, error) {
	return roleBindingsForSubject(subjectType, subjectID)
}

func EnsureRole(name, description string, isSystemRole bool) (*Role, bool, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, false, fmt.Errorf("role name is required")
	}
	return ensureRole(name, description, isSystemRole, time.Now().UTC())
}

func CreateRole(input RoleCreateInput) (*Role, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" {
		return nil, fmt.Errorf("role name is required")
	}
	if existing, found, err := findRoleByName(input.Name); err != nil || found {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("role name %q already exists", existing.Name)
	}

	now := time.Now().UTC()
	role := &Role{
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
	if err := Roles.Insert(role); err != nil {
		return nil, err
	}
	return role, nil
}

func UpdateRole(role *Role) error {
	role.Name = strings.TrimSpace(role.Name)
	role.Description = strings.TrimSpace(role.Description)
	role.UpdatedAt = time.Now().UTC()
	return Roles.Update(role)
}

func DeleteRole(roleID int) error {
	if _, err := RolePermissions.DeleteWithFilter(gomysql.NewFilter().
		KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID)); err != nil {
		return err
	}
	return Roles.Delete(roleID)
}

func RoleBindingCountForRole(roleID int) (int, error) {
	count, err := RoleBindings.CountWithFilter(gomysql.NewFilter().
		KeyCmp(RoleBindings.FieldBySQLName("role_id"), gomysql.OpEqual, roleID))
	return int(count), err
}

func GetRoleByName(name string) (*Role, bool, error) {
	return findRoleByName(strings.TrimSpace(name))
}

func GetPermissionByName(name string) (*Permission, bool, error) {
	return findPermissionByName(strings.TrimSpace(name))
}

func EnsureRolePermission(roleID, permissionID int) (bool, error) {
	return ensureRolePermission(roleID, permissionID)
}

func RemoveRolePermission(roleID, permissionID int) error {
	_, err := RolePermissions.DeleteWithFilter(gomysql.NewFilter().
		KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID).
		And().
		KeyCmp(RolePermissions.FieldBySQLName("permission_id"), gomysql.OpEqual, permissionID))
	return err
}

func RolePermissionsForRole(roleID int) ([]*RolePermission, error) {
	return RolePermissions.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID))
}

func PermissionKeysForRole(roleID int) ([]PermissionKey, error) {
	grants, err := RolePermissionsForRole(roleID)
	if err != nil {
		return nil, err
	}
	keys := make([]PermissionKey, 0, len(grants))
	for _, grant := range grants {
		permission, err := Permissions.Select(grant.PermissionID)
		if err != nil {
			return nil, err
		}
		if permission == nil {
			continue
		}
		key, ok := PermissionKeyFromName(permission.Name)
		if !ok {
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func EnsureRoleBinding(roleID int, subjectType RoleBindingSubject, subjectID int, scopeType RoleBindingScope, scopeID *int) (bool, error) {
	return ensureRoleBinding(roleID, subjectType, subjectID, scopeType, scopeID, time.Now().UTC())
}

func RemoveRoleBinding(roleBindingID int) error {
	return RoleBindings.Delete(roleBindingID)
}

func RoleBindingsForUserAndGroups(userID int, groupIDs []int) ([]*RoleBinding, error) {
	bindings, err := roleBindingsForSubject(RoleBindingSubjectUser, userID)
	if err != nil {
		return nil, err
	}

	for _, groupID := range groupIDs {
		groupBindings, groupErr := roleBindingsForSubject(RoleBindingSubjectGroup, groupID)
		if groupErr != nil {
			return nil, groupErr
		}
		bindings = append(bindings, groupBindings...)
	}

	return bindings, nil
}

func RolesForBindings(bindings []*RoleBinding) ([]*Role, error) {
	seen := make(map[int]bool)
	roles := make([]*Role, 0, len(bindings))

	for _, binding := range bindings {
		if seen[binding.RoleID] {
			continue
		}
		seen[binding.RoleID] = true

		role, err := Roles.Select(binding.RoleID)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func projectBindingMatchesResource(bindingScopeID *int, requestedScopeID *int) bool {
	if bindingScopeID == nil || requestedScopeID == nil {
		return false
	}
	resource, err := Resources.Select(*requestedScopeID)
	if err != nil || resource == nil {
		return false
	}
	return resource.ProjectID == *bindingScopeID
}

func scopeDomain(scopeType RoleBindingScope, scopeID *int) string {
	if scopeType == RoleBindingScopeGlobal {
		return "global"
	}
	if scopeID == nil {
		return roleBindingScopeName(scopeType) + ":"
	}
	return roleBindingScopeName(scopeType) + ":" + strconv.Itoa(*scopeID)
}

func scopeDomainMatches(policyDomain, requestDomain string) bool {
	if policyDomain == "global" {
		return true
	}
	if policyDomain == requestDomain {
		return true
	}

	policyType, policyID, ok := parseScopeDomain(policyDomain)
	if !ok {
		return false
	}
	requestType, requestID, ok := parseScopeDomain(requestDomain)
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

func parseScopeDomain(value string) (RoleBindingScope, int, bool) {
	scopeName, rawID, found := strings.Cut(value, ":")
	if !found || rawID == "" {
		return RoleBindingScopeGlobal, 0, false
	}
	scopeID, err := strconv.Atoi(rawID)
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

func roleBindingScopeName(value RoleBindingScope) string {
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

func userPrincipal(userID int) string {
	return "user:" + strconv.Itoa(userID)
}

func rolePrincipal(roleID int) string {
	return "role:" + strconv.Itoa(roleID)
}

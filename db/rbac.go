package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/z46-dev/gomysql"
)

type PermissionCheck struct {
	UserID         int
	GroupIDs       []int
	PermissionName string
	ScopeType      RoleBindingScope
	ScopeID        *int
}

func HasPermission(check PermissionCheck) (bool, error) {
	permission, found, err := findPermissionByName(check.PermissionName)
	if err != nil || !found {
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
		if !roleBindingMatchesScope(binding, check.ScopeType, check.ScopeID) {
			continue
		}

		hasPermission, err := roleHasPermission(binding.RoleID, permission.ID)
		if err != nil {
			return false, err
		}
		if hasPermission {
			return true, nil
		}
	}

	return false, nil
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

func UpdateRole(role *Role) error {
	role.Name = strings.TrimSpace(role.Name)
	role.Description = strings.TrimSpace(role.Description)
	role.UpdatedAt = time.Now().UTC()
	return Roles.Update(role)
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

func roleBindingMatchesScope(binding *RoleBinding, scopeType RoleBindingScope, scopeID *int) bool {
	if binding.ScopeType == RoleBindingScopeGlobal {
		return true
	}

	if binding.ScopeType == RoleBindingScopeOrg {
		return orgBindingMatchesScope(binding.ScopeID, scopeType, scopeID)
	}

	if binding.ScopeType != scopeType {
		return false
	}
	if binding.ScopeID == nil || scopeID == nil {
		return binding.ScopeID == nil && scopeID == nil
	}

	return *binding.ScopeID == *scopeID
}

func orgBindingMatchesScope(bindingScopeID *int, requestedScopeType RoleBindingScope, requestedScopeID *int) bool {
	if bindingScopeID == nil || requestedScopeID == nil {
		return bindingScopeID == nil && requestedScopeID == nil && requestedScopeType == RoleBindingScopeOrg
	}

	var orgIDs []int
	var err error
	switch requestedScopeType {
	case RoleBindingScopeOrg:
		orgIDs, err = OrganizationAncestorIDs(*requestedScopeID)
	case RoleBindingScopeProject:
		orgIDs, err = ProjectOrganizationAncestorIDs(*requestedScopeID)
	default:
		return false
	}
	if err != nil {
		return false
	}

	for _, orgID := range orgIDs {
		if orgID == *bindingScopeID {
			return true
		}
	}
	return false
}

func roleHasPermission(roleID, permissionID int) (bool, error) {
	count, err := RolePermissions.CountWithFilter(gomysql.NewFilter().
		KeyCmp(RolePermissions.FieldBySQLName("role_id"), gomysql.OpEqual, roleID).
		And().
		KeyCmp(RolePermissions.FieldBySQLName("permission_id"), gomysql.OpEqual, permissionID))
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

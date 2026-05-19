package db

import (
	"time"

	"github.com/z46-dev/gomysql"
)

// ResourceOwnersForResource returns ownership records for a resource.
func ResourceOwnersForResource(resourceID int) (itemsResult []*ResourceOwner, errResult error) {
	return ResourceOwners.SelectAllWithFilter(
		gomysql.NewFilter().KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gomysql.OpEqual, resourceID),
	)
}

// EnsureResourceOwner ensures resource owner exists.
func EnsureResourceOwner(resourceID int, subjectType OwnerSubjectType, subjectID int) (okResult bool, errResult error) {
	var filter *gomysql.Filter

	filter = gomysql.NewFilter().
		KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gomysql.OpEqual, resourceID).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID)
	var (
		existing []*ResourceOwner
		err      error
	)

	existing, err = ResourceOwners.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, nil
	}

	return true, ResourceOwners.Insert(&ResourceOwner{
		ResourceID:  resourceID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		CreatedAt:   time.Now().UTC(),
	})
}

// RemoveResourceOwner removes resource owner.
func RemoveResourceOwner(resourceID int, subjectType OwnerSubjectType, subjectID int) (errResult error) {
	var err error

	_, err = ResourceOwners.DeleteWithFilter(gomysql.NewFilter().
		KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gomysql.OpEqual, resourceID).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID))
	return err
}

// EnsureResourceUserAccess ensures resource user access exists.
func EnsureResourceUserAccess(resourceID int, subjectType RoleBindingSubject, subjectID int) (errResult error) {
	var (
		role  *Role
		found bool
		err   error
	)

	role, found, err = GetRoleByName(DefaultResourceUserRoleName)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	var scopeID int

	scopeID = resourceID
	_, err = ensureRoleBinding(role.ID, subjectType, subjectID, RoleBindingScopeResource, &scopeID, time.Now().UTC())
	return err
}

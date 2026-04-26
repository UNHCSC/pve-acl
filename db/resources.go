package db

import (
	"time"

	"github.com/z46-dev/gomysql"
)

func ResourceOwnersForResource(resourceID int) ([]*ResourceOwner, error) {
	return ResourceOwners.SelectAllWithFilter(
		gomysql.NewFilter().KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gomysql.OpEqual, resourceID),
	)
}

func EnsureResourceOwner(resourceID int, subjectType OwnerSubjectType, subjectID int) (bool, error) {
	filter := gomysql.NewFilter().
		KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gomysql.OpEqual, resourceID).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID)

	existing, err := ResourceOwners.SelectAllWithFilter(filter.Limit(1))
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

func RemoveResourceOwner(resourceID int, subjectType OwnerSubjectType, subjectID int) error {
	_, err := ResourceOwners.DeleteWithFilter(gomysql.NewFilter().
		KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gomysql.OpEqual, resourceID).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID))
	return err
}

func EnsureResourceUserAccess(resourceID int, subjectType RoleBindingSubject, subjectID int) error {
	role, found, err := GetRoleByName(DefaultResourceUserRoleName)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	scopeID := resourceID
	_, err = ensureRoleBinding(role.ID, subjectType, subjectID, RoleBindingScopeResource, &scopeID, time.Now().UTC())
	return err
}

package db

import (
	"time"

	"github.com/z46-dev/gomysql"
)

func EnsureUser(username, displayName, email, authSource, externalID string) (*User, bool, error) {
	if existing, found, err := findUserByUsername(username); err != nil || found {
		return existing, false, err
	}

	uuid, err := randomUUID()
	if err != nil {
		return nil, false, err
	}

	now := time.Now().UTC()
	user := &User{
		UUID:        uuid,
		Username:    username,
		DisplayName: displayName,
		Email:       email,
		AuthSource:  authSource,
		ExternalID:  externalID,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := Users.Insert(user); err != nil {
		return nil, false, err
	}

	return user, true, nil
}

func ListUsers() ([]*User, error) {
	return Users.SelectAll()
}

func GetUserByID(id int) (*User, bool, error) {
	user, err := Users.Select(id)
	if err != nil {
		return nil, false, err
	}
	if user == nil {
		return nil, false, nil
	}
	return user, true, nil
}

func ListCloudGroups() ([]*CloudGroup, error) {
	return CloudGroups.SelectAll()
}

func UpdateCloudGroup(group *CloudGroup) error {
	if group.SyncSource == "" {
		group.SyncSource = CloudGroupSyncSourceLocal
	}
	group.UpdatedAt = time.Now().UTC()
	return CloudGroups.Update(group)
}

func GetCloudGroupByID(id int) (*CloudGroup, bool, error) {
	group, err := CloudGroups.Select(id)
	if err != nil {
		return nil, false, err
	}
	if group == nil {
		return nil, false, nil
	}
	return group, true, nil
}

func CloudGroupMembershipsForGroup(groupID int) ([]*CloudGroupMembership, error) {
	return CloudGroupMemberships.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("group_id"), gomysql.OpEqual, groupID))
}

func CloudGroupMembershipsForUser(userID int) ([]*CloudGroupMembership, error) {
	return CloudGroupMemberships.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("user_id"), gomysql.OpEqual, userID))
}

func CloudGroupIDsForUser(userID int) ([]int, error) {
	memberships, err := CloudGroupMembershipsForUser(userID)
	if err != nil {
		return nil, err
	}

	groupIDs := make([]int, len(memberships))
	for i, membership := range memberships {
		groupIDs[i] = membership.GroupID
	}

	return groupIDs, nil
}

func CloudGroupsForUser(userID int) ([]*CloudGroup, error) {
	memberships, err := CloudGroupMembershipsForUser(userID)
	if err != nil {
		return nil, err
	}

	groups := make([]*CloudGroup, 0, len(memberships))
	for _, membership := range memberships {
		group, err := CloudGroups.Select(membership.GroupID)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}

	return groups, nil
}

func RemoveCloudGroupMembership(membershipID int) error {
	return CloudGroupMemberships.Delete(membershipID)
}

func UpdateCloudGroupMembershipRole(membershipID int, role MembershipRole) (*CloudGroupMembership, error) {
	membership, err := CloudGroupMemberships.Select(membershipID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, nil
	}
	membership.MembershipRole = role
	if err := CloudGroupMemberships.Update(membership); err != nil {
		return nil, err
	}
	return membership, nil
}

func CloudGroupMembershipForUserAndGroup(userID, groupID int) (*CloudGroupMembership, bool, error) {
	memberships, err := CloudGroupMemberships.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("user_id"), gomysql.OpEqual, userID).
		And().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("group_id"), gomysql.OpEqual, groupID).
		Limit(1))
	if err != nil {
		return nil, false, err
	}
	if len(memberships) == 0 {
		return nil, false, nil
	}
	return memberships[0], true, nil
}

func findUserByUsername(username string) (*User, bool, error) {
	return findOneByStringField(Users, Users.FieldBySQLName("username"), username)
}

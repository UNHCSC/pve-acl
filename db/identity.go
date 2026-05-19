package db

import (
	"strings"
	"time"

	"github.com/z46-dev/gomysql"
)

// EnsureUser ensures user exists.
func EnsureUser(username, displayName, email, authSource, externalID string) (userResult *User, okResult bool, errResult error) {
	{
		var (
			existing *User
			found    bool
			err      error
		)

		if existing, found, err = findUserByUsername(username); err != nil || found {
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
	var now time.Time

	now = time.Now().UTC()
	var user *User

	user = &User{
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
	{
		var err error

		if err = Users.Insert(user); err != nil {
			return nil, false, err
		}
	}

	return user, true, nil
}

// ListUsers lists all local users.
func ListUsers() (itemsResult []*User, errResult error) {
	return Users.SelectAll()
}

// GetUserByID returns a local user by id.
func GetUserByID(id int) (userResult *User, okResult bool, errResult error) {
	var (
		user *User
		err  error
	)

	user, err = Users.Select(id)
	if err != nil {
		return nil, false, err
	}
	if user == nil {
		return nil, false, nil
	}
	return user, true, nil
}

// ListCloudGroups lists cloud groups.
func ListCloudGroups() (itemsResult []*CloudGroup, errResult error) {
	return CloudGroups.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(CloudGroups.FieldBySQLName("archived_at"), gomysql.OpIsNull, nil))
}

// CloudGroupsForOwner returns active cloud groups owned by a scope.
func CloudGroupsForOwner(scopeType RoleBindingScope, scopeID *int) (itemsResult []*CloudGroup, errResult error) {
	var filter *gomysql.Filter

	filter = gomysql.NewFilter().
		KeyCmp(CloudGroups.FieldBySQLName("owner_scope_type"), gomysql.OpEqual, scopeType).
		And()
	if scopeID == nil {
		filter = filter.KeyCmp(CloudGroups.FieldBySQLName("owner_scope_id"), gomysql.OpIsNull, nil)
	} else {
		filter = filter.KeyCmp(CloudGroups.FieldBySQLName("owner_scope_id"), gomysql.OpEqual, *scopeID)
	}
	return CloudGroups.SelectAllWithFilter(filter.
		And().
		KeyCmp(CloudGroups.FieldBySQLName("archived_at"), gomysql.OpIsNull, nil))
}

// UpdateCloudGroup updates cloud group.
func UpdateCloudGroup(group *CloudGroup) (errResult error) {
	if group.SyncSource == "" {
		group.SyncSource = CloudGroupSyncSourceLocal
	}
	group.UpdatedAt = time.Now().UTC()
	return CloudGroups.Update(group)
}

// ArchiveCloudGroup archives cloud group.
func ArchiveCloudGroup(group *CloudGroup) (errResult error) {
	if group.ArchivedAt == nil {
		var now time.Time

		now = time.Now().UTC()
		group.ArchivedAt = &now
	}
	group.UpdatedAt = time.Now().UTC()
	return CloudGroups.Update(group)
}

// GetCloudGroupByID returns a cloud group by id.
func GetCloudGroupByID(id int) (cloudGroupResult *CloudGroup, okResult bool, errResult error) {
	var (
		group *CloudGroup
		err   error
	)

	group, err = CloudGroups.Select(id)
	if err != nil {
		return nil, false, err
	}
	if group == nil {
		return nil, false, nil
	}
	return group, true, nil
}

// GetCloudGroupBySlug returns cloud group by slug.
func GetCloudGroupBySlug(slug string) (cloudGroupResult *CloudGroup, okResult bool, errResult error) {
	return findOneByStringField(CloudGroups, CloudGroups.FieldBySQLName("slug"), strings.TrimSpace(slug))
}

// CloudGroupMembershipsForGroup returns memberships for a cloud group.
func CloudGroupMembershipsForGroup(groupID int) (itemsResult []*CloudGroupMembership, errResult error) {
	return CloudGroupMemberships.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("group_id"), gomysql.OpEqual, groupID))
}

// CloudGroupMembershipsForUser returns cloud group memberships for a user.
func CloudGroupMembershipsForUser(userID int) (itemsResult []*CloudGroupMembership, errResult error) {
	return CloudGroupMemberships.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(CloudGroupMemberships.FieldBySQLName("user_id"), gomysql.OpEqual, userID))
}

// CloudGroupIDsForUser returns cloud group ids for a user.
func CloudGroupIDsForUser(userID int) (itemsResult []int, errResult error) {
	var (
		memberships []*CloudGroupMembership
		err         error
	)

	memberships, err = CloudGroupMembershipsForUser(userID)
	if err != nil {
		return nil, err
	}
	var groupIDs []int

	groupIDs = make([]int, 0, len(memberships))
	for _, membership := range memberships {
		var (
			group *CloudGroup
			err   error
		)

		group, err = CloudGroups.Select(membership.GroupID)
		if err != nil {
			return nil, err
		}
		if group == nil || group.ArchivedAt != nil {
			continue
		}
		groupIDs = append(groupIDs, membership.GroupID)
	}

	return groupIDs, nil
}

// CloudGroupsForUser returns active cloud groups for a user.
func CloudGroupsForUser(userID int) (itemsResult []*CloudGroup, errResult error) {
	var (
		memberships []*CloudGroupMembership
		err         error
	)

	memberships, err = CloudGroupMembershipsForUser(userID)
	if err != nil {
		return nil, err
	}
	var groups []*CloudGroup

	groups = make([]*CloudGroup, 0, len(memberships))
	for _, membership := range memberships {
		var (
			group *CloudGroup
			err   error
		)

		group, err = CloudGroups.Select(membership.GroupID)
		if err != nil {
			return nil, err
		}
		if group == nil || group.ArchivedAt != nil {
			continue
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// RemoveCloudGroupMembership removes cloud group membership.
func RemoveCloudGroupMembership(membershipID int) (errResult error) {
	return CloudGroupMemberships.Delete(membershipID)
}

// UpdateCloudGroupMembershipRole updates cloud group membership role.
func UpdateCloudGroupMembershipRole(membershipID int, role MembershipRole) (cloudGroupMembershipResult *CloudGroupMembership, errResult error) {
	var (
		membership *CloudGroupMembership
		err        error
	)

	membership, err = CloudGroupMemberships.Select(membershipID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, nil
	}
	membership.MembershipRole = role
	{
		var err error

		if err = CloudGroupMemberships.Update(membership); err != nil {
			return nil, err
		}
	}
	return membership, nil
}

// CloudGroupMembershipForUserAndGroup returns a user's membership in a cloud group.
func CloudGroupMembershipForUserAndGroup(userID, groupID int) (cloudGroupMembershipResult *CloudGroupMembership, okResult bool, errResult error) {
	var (
		memberships []*CloudGroupMembership
		err         error
	)

	memberships, err = CloudGroupMemberships.SelectAllWithFilter(gomysql.NewFilter().
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

func findUserByUsername(username string) (userResult *User, okResult bool, errResult error) {
	return findOneByStringField(Users, Users.FieldBySQLName("username"), username)
}

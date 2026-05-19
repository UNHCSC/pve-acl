package db

import "github.com/z46-dev/gomysql"

// GroupsForUser returns local group names for a username.
func GroupsForUser(username string) (groupnames []string, err error) {
	var (
		filter  *gomysql.Filter = gomysql.NewFilter().KeyCmp(LocalGroupMembershipsByUser.FieldBySQLName("username"), gomysql.OpEqual, username)
		members []*LocalGroupMembership
	)

	if members, err = LocalGroupMembershipsByUser.SelectAllWithFilter(filter); err != nil {
		return
	}

	groupnames = make([]string, len(members))
	for index, membership := range members {
		groupnames[index] = membership.Groupname
	}

	return
}

// UsersForGroup returns local usernames for a group name.
func UsersForGroup(groupname string) (usernames []string, err error) {
	var (
		filter  *gomysql.Filter = gomysql.NewFilter().KeyCmp(LocalGroupMembershipsByUser.FieldBySQLName("groupname"), gomysql.OpEqual, groupname)
		members []*LocalGroupMembership
	)

	if members, err = LocalGroupMembershipsByUser.SelectAllWithFilter(filter); err != nil {
		return
	}

	usernames = make([]string, len(members))
	for index, membership := range members {
		usernames[index] = membership.Username
	}

	return
}

// AssetIDsForUser returns Proxmox asset ids assigned directly to a user.
func AssetIDsForUser(username string) (assetIDs []string, err error) {
	var (
		filter      *gomysql.Filter = gomysql.NewFilter().KeyCmp(ProxmoxAssetAssignmentsByUser.FieldBySQLName("username"), gomysql.OpEqual, username)
		assignments []*ProxmoxAssetAssignmentByUser
	)

	if assignments, err = ProxmoxAssetAssignmentsByUser.SelectAllWithFilter(filter); err != nil {
		return
	}

	assetIDs = make([]string, len(assignments))
	for index, assignment := range assignments {
		assetIDs[index] = assignment.AssetID
	}

	return
}

// AssetIDsForGroup returns Proxmox asset ids assigned to a group.
func AssetIDsForGroup(groupname string) (assetIDs []string, err error) {
	var (
		filter      *gomysql.Filter = gomysql.NewFilter().KeyCmp(ProxmoxAssetAssignmentsByGroup.FieldBySQLName("groupname"), gomysql.OpEqual, groupname)
		assignments []*ProxmoxAssetAssignmentByGroup
	)

	if assignments, err = ProxmoxAssetAssignmentsByGroup.SelectAllWithFilter(filter); err != nil {
		return
	}

	assetIDs = make([]string, len(assignments))
	for index, assignment := range assignments {
		assetIDs[index] = assignment.AssetID
	}

	return
}

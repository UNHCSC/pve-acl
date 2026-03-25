package db

import "github.com/z46-dev/gomysql"

func GroupsForUser(username string) (groupnames []string, err error) {
	var (
		filter  *gomysql.Filter = gomysql.NewFilter().KeyCmp(LocalGroupMembershipsByUser.FieldBySQLName("username"), gomysql.OpEqual, username)
		members []*LocalGroupMembership
	)

	if members, err = LocalGroupMembershipsByUser.SelectAllWithFilter(filter); err != nil {
		return
	}

	groupnames = make([]string, len(members))
	for i, m := range members {
		groupnames[i] = m.Groupname
	}

	return
}

func UsersForGroup(groupname string) (usernames []string, err error) {
	var (
		filter  *gomysql.Filter = gomysql.NewFilter().KeyCmp(LocalGroupMembershipsByUser.FieldBySQLName("groupname"), gomysql.OpEqual, groupname)
		members []*LocalGroupMembership
	)

	if members, err = LocalGroupMembershipsByUser.SelectAllWithFilter(filter); err != nil {
		return
	}

	usernames = make([]string, len(members))
	for i, m := range members {
		usernames[i] = m.Username
	}

	return
}

func AssetIDsForUser(username string) (assetIDs []string, err error) {
	var (
		filter      *gomysql.Filter = gomysql.NewFilter().KeyCmp(ProxmoxAssetAssignmentsByUser.FieldBySQLName("username"), gomysql.OpEqual, username)
		assignments []*ProxmoxAssetAssignmentByUser
	)

	if assignments, err = ProxmoxAssetAssignmentsByUser.SelectAllWithFilter(filter); err != nil {
		return
	}

	assetIDs = make([]string, len(assignments))
	for i, a := range assignments {
		assetIDs[i] = a.AssetID
	}

	return
}

func AssetIDsForGroup(groupname string) (assetIDs []string, err error) {
	var (
		filter      *gomysql.Filter = gomysql.NewFilter().KeyCmp(ProxmoxAssetAssignmentsByGroup.FieldBySQLName("groupname"), gomysql.OpEqual, groupname)
		assignments []*ProxmoxAssetAssignmentByGroup
	)

	if assignments, err = ProxmoxAssetAssignmentsByGroup.SelectAllWithFilter(filter); err != nil {
		return
	}

	assetIDs = make([]string, len(assignments))
	for i, a := range assignments {
		assetIDs[i] = a.AssetID
	}

	return
}

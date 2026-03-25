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

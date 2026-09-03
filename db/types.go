package db

import "time"

// Note: Data is imported from FreeIPA and Proxmox. We do not create or destroy any of these
// referenced entities (Users, Groups, Assets), we only reference them and create mappings
// between them. We only care about what we import too. If a user imports the group "group1"
// and "user1" is a member of groups "group1" and "group2", we will only create the membership
// mapping between "user1" and "group1".

type (
	ProxmoxAssetType     uint8
	ManagementPermission uint8
	AssetPermission      uint8

	LocalUser struct {
		Username  string    `gosqlite:"username,primary,unique" json:"username"`
		Name      string    `gosqlite:"name" json:"name"`
		Email     string    `gosqlite:"email" json:"email"`
		Notes     string    `gosqlite:"notes" json:"notes"`
		FirstSeen time.Time `gosqlite:"first_seen" json:"first_seen"`
		LastSeen  time.Time `gosqlite:"last_seen" json:"last_seen"`
	}

	LocalGroup struct {
		Groupname   string `gosqlite:"groupname,primary,unique" json:"groupname"`
		DisplayName string `gosqlite:"display_name" json:"display_name"`
		Notes       string `gosqlite:"notes" json:"notes"`
	}

	ProxmoxAsset struct {
		ID   string           `gosqlite:"id,primary,unique" json:"id"`
		Name string           `gosqlite:"name" json:"name"`
		Type ProxmoxAssetType `gosqlite:"type" json:"type"`
	}

	LocalGroupMembership struct {
		ID        int    `gosqlite:"membership_id,primary,increment" json:"membership_id"`
		Username  string `gosqlite:"username,fkey:LocalUser.username" json:"username"`
		Groupname string `gosqlite:"groupname,fkey:LocalGroup.groupname" json:"groupname"`
	}

	ProxmoxAssetAssignmentByUser struct {
		ID          int             `gosqlite:"ownership_id,primary,increment" json:"ownership_id"`
		AssetID     string          `gosqlite:"asset_id,fkey:ProxmoxAsset.id" json:"asset_id"`
		Username    string          `gosqlite:"username,fkey:LocalUser.username" json:"username"`
		Permissions AssetPermission `gosqlite:"permissions" json:"permissions"`
	}

	ProxmoxAssetAssignmentByGroup struct {
		ID          int             `gosqlite:"ownership_id,primary,increment" json:"ownership_id"`
		AssetID     string          `gosqlite:"asset_id,fkey:ProxmoxAsset.id" json:"asset_id"`
		Groupname   string          `gosqlite:"groupname,fkey:LocalGroup.groupname" json:"groupname"`
		Permissions AssetPermission `gosqlite:"permissions" json:"permissions"`
	}

	LocalGroupManagementByUser struct {
		ID          int                  `gosqlite:"membership_id,primary,increment" json:"membership_id"`
		Member      string               `gosqlite:"member,fkey:LocalUser.username" json:"member"`
		MemberOf    string               `gosqlite:"member_of,fkey:LocalGroup.groupname" json:"member_of"`
		Permissions ManagementPermission `gosqlite:"permissions" json:"permissions"`
	}

	LocalGroupManagementByGroup struct {
		ID          int                  `gosqlite:"membership_id,primary,increment" json:"membership_id"`
		Member      string               `gosqlite:"member,fkey:LocalGroup.groupname" json:"member"`
		MemberOf    string               `gosqlite:"member_of,fkey:LocalGroup.groupname" json:"member_of"`
		Permissions ManagementPermission `gosqlite:"permissions" json:"permissions"`
	}
)

// Note that full site administrators are configured through config.toml
// LDAP group entries. Usually defaults to "admins" for FreeIPA.

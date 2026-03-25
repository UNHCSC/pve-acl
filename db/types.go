package db

import "time"

// Note: Data is imported from FreeIPA and Proxmox. We do not create or destroy any of these
// referenced entities (Users, Groups, Assets), we only reference them and create mappings
// between them. We only care about what we import too. If a user imports the group "group1"
// and "user1" is a member of groups "group1" and "group2", we will only create the membership
// mapping between "user1" and "group1".

type (
	ProxmoxAssetType      uint8
	ManagementPermissions uint8
	AssetPermissions      uint8

	LocalUser struct {
		Username  string    `gomysql:"username,primary,unique" json:"username"`
		Notes     string    `gomysql:"notes" json:"notes"`
		FirstSeen time.Time `gomysql:"first_seen" json:"first_seen"`
		LastSeen  time.Time `gomysql:"last_seen" json:"last_seen"`
	}

	LocalGroup struct {
		Groupname   string `gomysql:"groupname,primary,unique" json:"groupname"`
		DisplayName string `gomysql:"display_name" json:"display_name"`
		Notes       string `gomysql:"notes" json:"notes"`
	}

	ProxmoxAsset struct {
		ID   string           `gomysql:"id,primary,unique" json:"id"`
		Name string           `gomysql:"name" json:"name"`
		Type ProxmoxAssetType `gomysql:"type" json:"type"`
	}

	LocalGroupMembershipByUser struct {
		ID        int    `gomysql:"membership_id,primary,increment" json:"membership_id"`
		Username  string `gomysql:"username,fkey:LocalUser.username" json:"username"`
		Groupname string `gomysql:"groupname,fkey:LocalGroup.groupname" json:"groupname"`
	}

	ProxmoxAssetAssignmentByUser struct {
		ID          int              `gomysql:"ownership_id,primary,increment" json:"ownership_id"`
		AssetID     string           `gomysql:"asset_id,fkey:ProxmoxAsset.id" json:"asset_id"`
		Username    string           `gomysql:"username,fkey:LocalUser.username" json:"username"`
		Permissions AssetPermissions `gomysql:"permissions" json:"permissions"`
	}

	ProxmoxAssetAssignmentByGroup struct {
		ID          int              `gomysql:"ownership_id,primary,increment" json:"ownership_id"`
		AssetID     string           `gomysql:"asset_id,fkey:ProxmoxAsset.id" json:"asset_id"`
		Groupname   string           `gomysql:"groupname,fkey:LocalGroup.groupname" json:"groupname"`
		Permissions AssetPermissions `gomysql:"permissions" json:"permissions"`
	}

	LocalGroupManagementByUser struct {
		ID          int                   `gomysql:"membership_id,primary,increment" json:"membership_id"`
		Member      string                `gomysql:"member,fkey:LocalUser.username" json:"member"`
		MemberOf    string                `gomysql:"member_of,fkey:LocalGroup.groupname" json:"member_of"`
		Permissions ManagementPermissions `gomysql:"permissions" json:"permissions"`
	}

	LocalGroupManagementByGroup struct {
		ID          int                   `gomysql:"membership_id,primary,increment" json:"membership_id"`
		Member      string                `gomysql:"member,fkey:LocalGroup.groupname" json:"member"`
		MemberOf    string                `gomysql:"member_of,fkey:LocalGroup.groupname" json:"member_of"`
		Permissions ManagementPermissions `gomysql:"permissions" json:"permissions"`
	}
)

const (
	ProxmoxAssetTypeVM ProxmoxAssetType = iota
	ProxmoxAssetTypeCT
)

const (
	PermissionsNone         AssetPermissions = 0               // Cannot see or do anything with the asset
	PermissionsView         AssetPermissions = 1 << (iota - 1) // Can view asset details and status, no actions
	PermissionsVNC                                             // Can use VNC console
	PermissionsPowerControl                                    // Can perform power actions (start, stop, reboot)
	PermissionsManage                                          // Can edit basic asset properties supported by system (may be expanded in future)
)

const (
	ManagementPermissionsNone                   ManagementPermissions = 0               // No special perms
	ManagementPermissionsUserAssets             ManagementPermissions = 1 << (iota - 1) // Can manage assets assigned to users in group this is bound to (LocalGroupManagementByUser, LocalGroupManagementByGroup)
	ManagementPermissionsManageLocalPermissions                                         // Can create/edit/delete LocalGroupManagementByUser and LocalGroupManagementByGroup entries for a certain group
)

// Note that full site administrators are configured through config.toml
// LDAP group entries. Usually defaults to "admins" for FreeIPA.

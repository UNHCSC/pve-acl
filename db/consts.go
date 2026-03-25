package db

const (
	ProxmoxAssetTypeVM ProxmoxAssetType = iota
	ProxmoxAssetTypeCT
)

const (
	AssetPermissionsNone         AssetPermission = 0               // Cannot see or do anything with the asset
	AssetPermissionsView         AssetPermission = 1 << (iota - 1) // Can view asset details and status, no actions
	AssetPermissionsVNC                                            // Can use VNC console
	AssetPermissionsPowerControl                                   // Can perform power actions (start, stop, reboot)
	AssetPermissionsManage                                         // Can edit basic asset properties supported by system (may be expanded in future)
)

const (
	ManagementPermissionsNone                   ManagementPermission = 0               // No special perms
	ManagementPermissionsUserAssets             ManagementPermission = 1 << (iota - 1) // Can manage assets assigned to users in group this is bound to (LocalGroupManagementByUser, LocalGroupManagementByGroup)
	ManagementPermissionsModifyAssets                                                  // Can modify asset configurations (e.g. CPU, Memory, Storage, Network) for assets in groups this is bound to (LocalGroupManagementByUser, LocalGroupManagementByGroup)
	ManagementPermissionsManageLocalPermissions                                        // Can create/edit/delete LocalGroupManagementByUser and LocalGroupManagementByGroup entries for a certain group
)

var (
	AssetTypes map[ProxmoxAssetType]string = map[ProxmoxAssetType]string{
		ProxmoxAssetTypeVM: "Virtual Machine",
		ProxmoxAssetTypeCT: "Container",
	}

	AssetPermissions map[AssetPermission]string = map[AssetPermission]string{
		AssetPermissionsNone:         "No Permissions",
		AssetPermissionsView:         "View",
		AssetPermissionsVNC:          "VNC Access",
		AssetPermissionsPowerControl: "Power Control",
		AssetPermissionsManage:       "Manage",
	}

	ManagementPermissions map[ManagementPermission]string = map[ManagementPermission]string{
		ManagementPermissionsNone:                   "No Permissions",
		ManagementPermissionsUserAssets:             "Manage User Assets",
		ManagementPermissionsModifyAssets:           "Modify Assets",
		ManagementPermissionsManageLocalPermissions: "Manage Local Permissions",
	}

	AssetTypesReverse            map[string]ProxmoxAssetType     = make(map[string]ProxmoxAssetType, len(AssetTypes))
	AssetPermissionsReverse      map[string]AssetPermission      = make(map[string]AssetPermission, len(AssetPermissions))
	ManagementPermissionsReverse map[string]ManagementPermission = make(map[string]ManagementPermission, len(ManagementPermissions))
)

func init() {
	for val, key := range AssetTypes {
		AssetTypesReverse[key] = val
	}

	for val, key := range AssetPermissions {
		AssetPermissionsReverse[key] = val
	}

	for val, key := range ManagementPermissions {
		ManagementPermissionsReverse[key] = val
	}
}

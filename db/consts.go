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

const (
	GroupTypeAdmin        GroupType = 0
	GroupTypeClub         GroupType = 1
	GroupTypeCompetition  GroupType = 2
	GroupTypeStudentGroup GroupType = 5
	GroupTypeProject      GroupType = 6
	GroupTypeCustom       GroupType = 7
)

const (
	CloudGroupSyncSourceLocal = "local"
	CloudGroupSyncSourceLDAP  = "ldap"
)

const (
	RoleBindingSubjectUser RoleBindingSubject = iota
	RoleBindingSubjectGroup
)

const (
	RoleBindingScopeGlobal   RoleBindingScope = 0
	RoleBindingScopeOrg      RoleBindingScope = 1
	RoleBindingScopeProject  RoleBindingScope = 3
	RoleBindingScopeGroup    RoleBindingScope = 4
	RoleBindingScopeResource RoleBindingScope = 5
)

const (
	ProjectTypeAdmin       ProjectType = 0
	ProjectTypeClub        ProjectType = 1
	ProjectTypeCompetition ProjectType = 2
	ProjectTypeStudent     ProjectType = 4
	ProjectTypeGroup       ProjectType = 5
	ProjectTypeLab         ProjectType = 6
	ProjectTypeCustom      ProjectType = 7
)

const (
	ProjectMemberSubjectUser ProjectMemberSubject = iota
	ProjectMemberSubjectGroup
)

const (
	OwnerSubjectUser OwnerSubjectType = iota
	OwnerSubjectGroup
)

const (
	ProjectRoleViewer ProjectRole = iota
	ProjectRoleOperator
	ProjectRoleDeveloper
	ProjectRoleManager
	ProjectRoleOwner
)

const (
	OwnerTypeUser         OwnerType = 0
	OwnerTypeGroup        OwnerType = 1
	OwnerTypeProject      OwnerType = 2
	OwnerTypeOrganization OwnerType = 4
)

const (
	ResourceTypeVM ResourceType = iota
	ResourceTypeCT
	ResourceTypeNetwork
	ResourceTypeTemplate
	ResourceTypeVolume
	ResourceTypeSecret
	ResourceTypeTerraformWorkspace
	ResourceTypeAnsibleInventory
)

const (
	ResourceStatusCreating ResourceStatus = iota
	ResourceStatusReady
	ResourceStatusUpdating
	ResourceStatusDeleting
	ResourceStatusDeleted
	ResourceStatusError
	ResourceStatusUnknown
)

const (
	NetworkTypeBridge NetworkType = iota
	NetworkTypeVLAN
	NetworkTypeVXLAN
	NetworkTypeSDNZone
	NetworkTypeIsolated
	NetworkTypeRouted
	NetworkTypeNAT
)

const (
	PowerStateRunning PowerState = iota
	PowerStateStopped
	PowerStatePaused
	PowerStateUnknown
)

const (
	JobTypeProxmox JobType = iota
	JobTypeTerraform
	JobTypeAnsible
	JobTypeLabDeploy
	JobTypeLabDestroy
	JobTypeSync
	JobTypeCleanup
)

const (
	JobStatusQueued JobStatus = iota
	JobStatusRunning
	JobStatusSucceeded
	JobStatusFailed
	JobStatusCancelled
)

const (
	JobLogStreamStdout JobLogStream = iota
	JobLogStreamStderr
	JobLogStreamSystem
)

const (
	SecretTypeProxmoxToken SecretType = iota
	SecretTypeSSHKey
	SecretTypePassword
	SecretTypeAPIToken
	SecretTypeTerraformVar
	SecretTypeAnsibleVar
)

const (
	SecretOwnerTypeSystem SecretOwnerType = iota
	SecretOwnerTypeUser
	SecretOwnerTypeGroup
	SecretOwnerTypeProject
)

const (
	MembershipRoleMember MembershipRole = iota
	MembershipRoleManager
	MembershipRoleOwner
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

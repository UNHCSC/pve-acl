package db

type PermissionKey uint16

const (
	PermissionVMRead PermissionKey = iota
	PermissionVMCreate
	PermissionVMStart
	PermissionVMStop
	PermissionVMReboot
	PermissionVMConsole
	PermissionVMSnapshot
	PermissionVMClone
	PermissionVMResize
	PermissionVMReconfigure
	PermissionVMDelete
	PermissionCTRead
	PermissionCTCreate
	PermissionCTStart
	PermissionCTStop
	PermissionCTConsole
	PermissionCTDelete
	PermissionNetworkRead
	PermissionNetworkCreate
	PermissionNetworkUpdate
	PermissionNetworkDelete
	PermissionNetworkAttach
	PermissionTemplateRead
	PermissionTemplateCreate
	PermissionTemplateUpdate
	PermissionTemplateDelete
	PermissionTemplateClone
	PermissionTerraformPlan
	PermissionTerraformApply
	PermissionTerraformDestroy
	PermissionAnsibleRun
	PermissionQuotaRead
	PermissionQuotaUpdate
	PermissionUserManage
	PermissionGroupManage
	PermissionRoleManage
	PermissionAuditRead
	PermissionOrgManage
	PermissionProjectManage
)

var permissionNames = map[PermissionKey]string{
	PermissionVMRead:           "vm.read",
	PermissionVMCreate:         "vm.create",
	PermissionVMStart:          "vm.start",
	PermissionVMStop:           "vm.stop",
	PermissionVMReboot:         "vm.reboot",
	PermissionVMConsole:        "vm.console",
	PermissionVMSnapshot:       "vm.snapshot",
	PermissionVMClone:          "vm.clone",
	PermissionVMResize:         "vm.resize",
	PermissionVMReconfigure:    "vm.reconfigure",
	PermissionVMDelete:         "vm.delete",
	PermissionCTRead:           "ct.read",
	PermissionCTCreate:         "ct.create",
	PermissionCTStart:          "ct.start",
	PermissionCTStop:           "ct.stop",
	PermissionCTConsole:        "ct.console",
	PermissionCTDelete:         "ct.delete",
	PermissionNetworkRead:      "network.read",
	PermissionNetworkCreate:    "network.create",
	PermissionNetworkUpdate:    "network.update",
	PermissionNetworkDelete:    "network.delete",
	PermissionNetworkAttach:    "network.attach",
	PermissionTemplateRead:     "template.read",
	PermissionTemplateCreate:   "template.create",
	PermissionTemplateUpdate:   "template.update",
	PermissionTemplateDelete:   "template.delete",
	PermissionTemplateClone:    "template.clone",
	PermissionTerraformPlan:    "terraform.plan",
	PermissionTerraformApply:   "terraform.apply",
	PermissionTerraformDestroy: "terraform.destroy",
	PermissionAnsibleRun:       "ansible.run",
	PermissionQuotaRead:        "quota.read",
	PermissionQuotaUpdate:      "quota.update",
	PermissionUserManage:       "user.manage",
	PermissionGroupManage:      "group.manage",
	PermissionRoleManage:       "role.manage",
	PermissionAuditRead:        "audit.read",
	PermissionOrgManage:        "org.manage",
	PermissionProjectManage:    "project.manage",
}

var CorePermissions = []PermissionKey{
	PermissionVMRead,
	PermissionVMCreate,
	PermissionVMStart,
	PermissionVMStop,
	PermissionVMReboot,
	PermissionVMConsole,
	PermissionVMSnapshot,
	PermissionVMClone,
	PermissionVMResize,
	PermissionVMReconfigure,
	PermissionVMDelete,
	PermissionCTRead,
	PermissionCTCreate,
	PermissionCTStart,
	PermissionCTStop,
	PermissionCTConsole,
	PermissionCTDelete,
	PermissionNetworkRead,
	PermissionNetworkCreate,
	PermissionNetworkUpdate,
	PermissionNetworkDelete,
	PermissionNetworkAttach,
	PermissionTemplateRead,
	PermissionTemplateCreate,
	PermissionTemplateUpdate,
	PermissionTemplateDelete,
	PermissionTemplateClone,
	PermissionTerraformPlan,
	PermissionTerraformApply,
	PermissionTerraformDestroy,
	PermissionAnsibleRun,
	PermissionQuotaRead,
	PermissionQuotaUpdate,
	PermissionUserManage,
	PermissionGroupManage,
	PermissionRoleManage,
	PermissionAuditRead,
	PermissionOrgManage,
	PermissionProjectManage,
}

var CorePermissionNames = permissionKeyNames(CorePermissions)

func (key PermissionKey) String() string {
	return permissionNames[key]
}

func PermissionKeyFromName(name string) (PermissionKey, bool) {
	for key, candidate := range permissionNames {
		if candidate == name {
			return key, true
		}
	}
	return 0, false
}

func permissionKeyNames(keys []PermissionKey) []string {
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		names = append(names, key.String())
	}
	return names
}

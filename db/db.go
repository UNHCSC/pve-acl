package db

import (
	"fmt"
	"os"
	"slices"

	"github.com/UNHCSC/proxman/config"
	"github.com/z46-dev/golog"
	"github.com/z46-dev/gomysql"
)

var (
	dbLog                          *golog.Logger
	LocalUsers                     *gomysql.RegisteredStruct[LocalUser]
	LocalGroups                    *gomysql.RegisteredStruct[LocalGroup]
	ProxmoxAssets                  *gomysql.RegisteredStruct[ProxmoxAsset]
	LocalGroupMembershipsByUser    *gomysql.RegisteredStruct[LocalGroupMembership]
	ProxmoxAssetAssignmentsByUser  *gomysql.RegisteredStruct[ProxmoxAssetAssignmentByUser]
	ProxmoxAssetAssignmentsByGroup *gomysql.RegisteredStruct[ProxmoxAssetAssignmentByGroup]
	LocalGroupManagementsByUser    *gomysql.RegisteredStruct[LocalGroupManagementByUser]
	LocalGroupManagementsByGroup   *gomysql.RegisteredStruct[LocalGroupManagementByGroup]
	Users                          *gomysql.RegisteredStruct[User]
	CloudGroups                    *gomysql.RegisteredStruct[CloudGroup]
	CloudGroupMemberships          *gomysql.RegisteredStruct[CloudGroupMembership]
	Organizations                  *gomysql.RegisteredStruct[Organization]
	Courses                        *gomysql.RegisteredStruct[Course]
	Projects                       *gomysql.RegisteredStruct[Project]
	ProjectMemberships             *gomysql.RegisteredStruct[ProjectMembership]
	Roles                          *gomysql.RegisteredStruct[Role]
	Permissions                    *gomysql.RegisteredStruct[Permission]
	RolePermissions                *gomysql.RegisteredStruct[RolePermission]
	RoleBindings                   *gomysql.RegisteredStruct[RoleBinding]
	QuotaPolicies                  *gomysql.RegisteredStruct[QuotaPolicy]
	QuotaBindings                  *gomysql.RegisteredStruct[QuotaBinding]
	Resources                      *gomysql.RegisteredStruct[Resource]
	ProxmoxClusters                *gomysql.RegisteredStruct[ProxmoxCluster]
	ProxmoxNodes                   *gomysql.RegisteredStruct[ProxmoxNode]
	VirtualMachines                *gomysql.RegisteredStruct[VirtualMachine]
	Containers                     *gomysql.RegisteredStruct[Container]
	VirtualNetworks                *gomysql.RegisteredStruct[VirtualNetwork]
	Jobs                           *gomysql.RegisteredStruct[Job]
	JobLogs                        *gomysql.RegisteredStruct[JobLog]
	AuditEvents                    *gomysql.RegisteredStruct[AuditEvent]
	Secrets                        *gomysql.RegisteredStruct[Secret]
)

func Init(parentLog *golog.Logger) (err error) {
	dbLog = parentLog.SpawnChild().Prefix("[DB]", golog.BoldGreen)

	if err = gomysql.Begin(config.Config.Database.File); err != nil {
		dbLog.Errorf("Failed to initialize database: %v\n", err)
		return
	}

	var migrationOpts gomysql.MigrationOptions

	if len(os.Args) > 1 && slices.Contains(os.Args, "--allow-destructive-migrations") {
		migrationOpts.AllowDestructive = true
		dbLog.Warning("Destructive migrations are enabled!")
	}

	if err = registerAndMigrate("LocalUsers", &LocalUsers, LocalUser{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("LocalGroups", &LocalGroups, LocalGroup{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("ProxmoxAssets", &ProxmoxAssets, ProxmoxAsset{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("LocalGroupMembershipsByUser", &LocalGroupMembershipsByUser, LocalGroupMembership{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("ProxmoxAssetAssignmentsByUser", &ProxmoxAssetAssignmentsByUser, ProxmoxAssetAssignmentByUser{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("ProxmoxAssetAssignmentsByGroup", &ProxmoxAssetAssignmentsByGroup, ProxmoxAssetAssignmentByGroup{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("LocalGroupManagementsByUser", &LocalGroupManagementsByUser, LocalGroupManagementByUser{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("LocalGroupManagementsByGroup", &LocalGroupManagementsByGroup, LocalGroupManagementByGroup{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Users", &Users, User{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("CloudGroups", &CloudGroups, CloudGroup{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("CloudGroupMemberships", &CloudGroupMemberships, CloudGroupMembership{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Organizations", &Organizations, Organization{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Courses", &Courses, Course{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Projects", &Projects, Project{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("ProjectMemberships", &ProjectMemberships, ProjectMembership{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Roles", &Roles, Role{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Permissions", &Permissions, Permission{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("RolePermissions", &RolePermissions, RolePermission{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("RoleBindings", &RoleBindings, RoleBinding{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("QuotaPolicies", &QuotaPolicies, QuotaPolicy{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("QuotaBindings", &QuotaBindings, QuotaBinding{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Resources", &Resources, Resource{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("ProxmoxClusters", &ProxmoxClusters, ProxmoxCluster{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("ProxmoxNodes", &ProxmoxNodes, ProxmoxNode{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("VirtualMachines", &VirtualMachines, VirtualMachine{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Containers", &Containers, Container{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("VirtualNetworks", &VirtualNetworks, VirtualNetwork{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Jobs", &Jobs, Job{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("JobLogs", &JobLogs, JobLog{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("AuditEvents", &AuditEvents, AuditEvent{}, migrationOpts); err != nil {
		return
	}
	if err = registerAndMigrate("Secrets", &Secrets, Secret{}, migrationOpts); err != nil {
		return
	}

	dbLog.Info("Database initialized successfully")

	return
}

func registerAndMigrate[T any](name string, target **gomysql.RegisteredStruct[T], model T, opts gomysql.MigrationOptions) (err error) {
	if *target, err = gomysql.Register(model); err != nil {
		dbLog.Errorf("Failed to register %s struct: %v\n", name, err)
		return
	}

	if err = migrate(*target, opts); err != nil {
		dbLog.Errorf("Failed to migrate %s table: %v\n", name, err)
		return
	}

	return
}

func migrate[T any](table *gomysql.RegisteredStruct[T], opts gomysql.MigrationOptions) (err error) {
	var report *gomysql.MigrationReport

	if report, err = table.Migrate(opts); err != nil {
		return
	}

	if report == nil {
		err = fmt.Errorf("migration report is nil")
		return
	}

	if len(report.AddedColumns) > 0 {
		dbLog.Warningf("Added columns to table for %T: %v\n", *new(T), report.AddedColumns)
	}

	if len(report.ChangedColumns) > 0 {
		dbLog.Warningf("Changed columns in table for %T: %v\n", *new(T), report.ChangedColumns)
	}

	if len(report.DroppedColumns) > 0 {
		dbLog.Warningf("Dropped columns from table for %T: %v\n", *new(T), report.DroppedColumns)
	}

	if len(report.RenamedColumns) > 0 {
		dbLog.Warningf("Renamed columns in table for %T: %v\n", *new(T), report.RenamedColumns)
	}

	if report.Rebuilt {
		dbLog.Warningf("Rebuilt table for %T\n", *new(T))
	}

	return
}

package db

import (
	"fmt"
	"os"
	"slices"

	"github.com/UNHCSC/pve-acl/config"
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
)

func Init(parentLog *golog.Logger) (err error) {
	dbLog = parentLog.SpawnChild().Prefix("[DB]", golog.BoldGreen)

	if err = gomysql.Begin(config.Config.Database.File); err != nil {
		dbLog.Errorf("Failed to initialize database: %v\n", err)
		return
	}

	if LocalUsers, err = gomysql.Register(LocalUser{}); err != nil {
		dbLog.Errorf("Failed to register LocalUser struct: %v\n", err)
		return
	}

	if LocalGroups, err = gomysql.Register(LocalGroup{}); err != nil {
		dbLog.Errorf("Failed to register LocalGroup struct: %v\n", err)
		return
	}

	if ProxmoxAssets, err = gomysql.Register(ProxmoxAsset{}); err != nil {
		dbLog.Errorf("Failed to register ProxmoxAsset struct: %v\n", err)
		return
	}

	if LocalGroupMembershipsByUser, err = gomysql.Register(LocalGroupMembership{}); err != nil {
		dbLog.Errorf("Failed to register LocalGroupMembershipByUser struct: %v\n", err)
		return
	}

	if ProxmoxAssetAssignmentsByUser, err = gomysql.Register(ProxmoxAssetAssignmentByUser{}); err != nil {
		dbLog.Errorf("Failed to register ProxmoxAssetAssignmentByUser struct: %v\n", err)
		return
	}

	if ProxmoxAssetAssignmentsByGroup, err = gomysql.Register(ProxmoxAssetAssignmentByGroup{}); err != nil {
		dbLog.Errorf("Failed to register ProxmoxAssetAssignmentByGroup struct: %v\n", err)
		return
	}

	if LocalGroupManagementsByUser, err = gomysql.Register(LocalGroupManagementByUser{}); err != nil {
		dbLog.Errorf("Failed to register LocalGroupManagementByUser struct: %v\n", err)
		return
	}

	if LocalGroupManagementsByGroup, err = gomysql.Register(LocalGroupManagementByGroup{}); err != nil {
		dbLog.Errorf("Failed to register LocalGroupManagementByGroup struct: %v\n", err)
		return
	}

	// Migrations
	var migrationOpts gomysql.MigrationOptions

	if len(os.Args) > 1 && slices.Contains(os.Args, "--allow-destructive-migrations") {
		migrationOpts.AllowDestructive = true
		dbLog.Warning("Destructive migrations are enabled!")
	}

	if err = migrate(LocalUsers, migrationOpts); err != nil {
		dbLog.Errorf("Failed to migrate LocalUsers table: %v\n", err)
		return
	}

	if err = migrate(LocalGroups, migrationOpts); err != nil {
		dbLog.Errorf("Failed to migrate LocalGroups table: %v\n", err)
		return
	}

	if err = migrate(ProxmoxAssets, migrationOpts); err != nil {
		dbLog.Errorf("Failed to migrate ProxmoxAssets table: %v\n", err)
		return
	}

	if err = migrate(LocalGroupMembershipsByUser, migrationOpts); err != nil {
		dbLog.Errorf("Failed to migrate LocalGroupMembershipsByUser table: %v\n", err)
		return
	}

	if err = migrate(ProxmoxAssetAssignmentsByUser, migrationOpts); err != nil {
		dbLog.Errorf("Failed to migrate ProxmoxAssetAssignmentsByUser table: %v\n", err)
		return
	}

	if err = migrate(ProxmoxAssetAssignmentsByGroup, migrationOpts); err != nil {
		dbLog.Errorf("Failed to migrate ProxmoxAssetAssignmentsByGroup table: %v\n", err)
		return
	}

	if err = migrate(LocalGroupManagementsByUser, migrationOpts); err != nil {
		dbLog.Errorf("Failed to migrate LocalGroupManagementsByUser table: %v\n", err)
		return
	}

	if err = migrate(LocalGroupManagementsByGroup, migrationOpts); err != nil {
		dbLog.Errorf("Failed to migrate LocalGroupManagementsByGroup table: %v\n", err)
		return
	}

	dbLog.Info("Database initialized successfully")

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

package app

import (
	"os"
	"path/filepath"

	"github.com/UNHCSC/organesson/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	"github.com/z46-dev/golog"
)

var appLog *golog.Logger = golog.New().Prefix("[Please call app.InitAndListen() with the main logger]", golog.BoldRed)

func InitAndListen(parentLog *golog.Logger) (app *fiber.App, err error) {
	appLog = parentLog.SpawnChild().Prefix("[APP]", golog.BoldPurple)

	var templateEngine *html.Engine = html.New(clientPath("views"), ".html")
	templateEngine.Reload(config.Config.WebServer.ReloadTemplatesOnEachRender)

	if err = initPersistentJWTSigningKey(); err != nil {
		return
	}

	app = fiber.New(fiber.Config{
		Views:   templateEngine,
		Network: "tcp",
	})
	app.Use(securityHeaders)

	// Statics
	app.Static("/static", clientPath("static"))

	// Pages
	app.Get("/", getHome)
	app.Get("/dashboard", getDashboard)
	app.Get("/login", getLogin)

	// API
	var (
		api   fiber.Router = app.Group("/api")
		apiV1 fiber.Router = api.Group("/v1")
	)

	// API v1
	var (
		apiV1Auth     fiber.Router = apiV1.Group("/auth")
		apiV1Enums    fiber.Router = apiV1.Group("/enums")
		apiV1System   fiber.Router = apiV1.Group("/system")
		apiV1Users    fiber.Router = apiV1.Group("/users")
		apiV1Groups   fiber.Router = apiV1.Group("/groups")
		apiV1Orgs     fiber.Router = apiV1.Group("/organizations")
		apiV1Roles    fiber.Router = apiV1.Group("/roles")
		apiV1Projects fiber.Router = apiV1.Group("/projects")
		apiV1Assets   fiber.Router = apiV1.Group("/assets")
		apiV1ACL      fiber.Router = apiV1.Group("/acl")
	)

	// API v1 auth
	apiV1Auth.Post("/login", postLogin)
	apiV1Auth.Post("/logout", postLogout)
	apiV1Auth.Get("/status", getStatus)

	apiV1.Use(requireAPIAuth)

	// API v1 enums
	apiV1Enums.Get("/asset-types", getAssetTypes)
	apiV1Enums.Get("/asset-types/reverse", getAssetTypesReverse)
	apiV1Enums.Get("/asset-permissions", getAssetPermissions)
	apiV1Enums.Get("/asset-permissions/reverse", getAssetPermissionsReverse)
	apiV1Enums.Get("/management-permissions", getManagementPermissions)
	apiV1Enums.Get("/management-permissions/reverse", getManagementPermissionsReverse)

	// API v1 system
	apiV1System.Get("/summary", getSystemSummary)
	apiV1System.Get("/access", getAccessData)

	// API v1 access grants
	apiV1.Get("/role-bindings", getRoleBindings)
	apiV1.Post("/role-bindings", postCreateRoleBinding)
	apiV1.Delete("/role-bindings/:bindingID", deleteRoleBinding)

	// API v1 users
	apiV1.Get("/users", getUsers)
	apiV1.Post("/users", postCreateUser)
	apiV1.Post("/users/import", postImportUsers)
	apiV1Users.Get("/me", getCurrentUser)
	apiV1Users.Get("/me/access", getCurrentUserAccess)
	apiV1Users.Get("/resolve", getResolveUser)
	apiV1Users.Get("/", getUsers)
	apiV1Users.Post("/", postCreateUser)
	apiV1Users.Post("/import", postImportUsers)
	apiV1Users.Get("/search", _noop)
	apiV1Users.Get("/some/:usernames", _noop)
	apiV1Users.Post("/update/:usernames", _noop)

	// API v1 groups
	apiV1.Get("/groups", getCloudGroups)
	apiV1.Post("/groups", postCreateCloudGroup)
	apiV1Groups.Get("/", getCloudGroups)
	apiV1Groups.Post("/", postCreateCloudGroup)
	apiV1Groups.Get("/:id/memberships", getGroupMemberships)
	apiV1Groups.Post("/:id/memberships", postCreateGroupMembership)
	apiV1Groups.Patch("/:id/memberships/:membershipID", patchGroupMembership)
	apiV1Groups.Delete("/:id/memberships/:membershipID", deleteGroupMembership)
	apiV1Groups.Get("/:id/role-bindings", getGroupRoleBindings)
	apiV1Groups.Post("/:id/role-bindings", postCreateGroupRoleBinding)
	apiV1Groups.Delete("/:id/role-bindings/:bindingID", deleteGroupRoleBinding)
	apiV1Groups.Get("/search", _noop)
	apiV1Groups.Get("/some/:groupnames", _noop)
	apiV1Groups.Post("/update/:groupnames", _noop)
	apiV1Groups.Get("/:id", getCloudGroupByID)

	// API v1 organizations
	apiV1.Get("/organizations", getProjectTree)
	apiV1.Post("/organizations", postCreateOrganization)
	apiV1Orgs.Post("/", postCreateOrganization)
	apiV1Orgs.Patch("/:id", patchOrganization)
	apiV1Orgs.Delete("/:id", deleteOrganization)

	// API v1 roles
	apiV1.Get("/roles", getRoles)
	apiV1.Post("/roles", postCreateRole)
	apiV1Roles.Get("/", getRoles)
	apiV1Roles.Post("/", postCreateRole)
	apiV1Roles.Get("/:id/permissions", getRolePermissions)
	apiV1Roles.Post("/:id/permissions", postCreateRolePermission)
	apiV1Roles.Delete("/:id/permissions/:permissionID", deleteRolePermission)

	// API v1 projects
	apiV1.Get("/projects", getProjects)
	apiV1.Post("/projects", postCreateProject)
	apiV1Projects.Get("/tree", getProjectTree)
	apiV1Projects.Get("/", getProjects)
	apiV1Projects.Post("/", postCreateProject)
	apiV1Projects.Get("/:id/memberships", getProjectMemberships)
	apiV1Projects.Post("/:id/memberships", postCreateProjectMembership)
	apiV1Projects.Patch("/:id/memberships/:membershipID", patchProjectMembership)
	apiV1Projects.Delete("/:id/memberships/:membershipID", deleteProjectMembership)
	apiV1Projects.Patch("/:id", patchProject)
	apiV1Projects.Get("/:slug", getProjectBySlug)
	apiV1Projects.Delete("/:slug", deleteProjectBySlug)

	// API v1 assets
	apiV1Assets.Get("/search", _noop)
	apiV1Assets.Get("/some/:assetnames", _noop)
	apiV1Assets.Post("/update/:assetnames", _noop)
	apiV1Assets.Post("/create", _noop)

	// API v1 ACL
	apiV1ACL.Get("/groupsForUser/:username", getGroupsForUser)
	apiV1ACL.Get("/usersForGroup/:groupname", getUsersForGroup)
	apiV1ACL.Get("/assetsForUser/:username", _noop)
	apiV1ACL.Get("/assetsForGroup/:groupname", _noop)
	apiV1ACL.Get("/assignmentsForAsset/:assetid", _noop)
	apiV1ACL.Post("/updateAssetAssignments", _noop) // body will have sets of additions, changes, and deletions to apply
	apiV1ACL.Post("/updateGroupManagement", _noop)

	return
}

func clientPath(part string) string {
	path := filepath.Join("client", part)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return filepath.Join("..", "client", part)
}

func _noop(*fiber.Ctx) (err error) {
	return
}

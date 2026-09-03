package app

import (
	"fmt"
	"strings"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

type (
	blueprintCreateRequest struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}

	blueprintVersionRequest struct {
		Document db.BlueprintDocument `json:"document"`
	}

	deploymentPreviewRequest struct {
		BlueprintVersionID int    `json:"blueprintVersionID"`
		GroupIDs           []int  `json:"groupIDs"`
		NamePrefix         string `json:"namePrefix"`
	}
)

// getProjectBlueprints lists generic blueprints and their immutable versions.
func getProjectBlueprints(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	if project, errResult = requireBlueprintProject(c, false); errResult != nil || project == nil {
		return
	}
	var blueprints []*db.Blueprint
	if blueprints, errResult = db.BlueprintsForProject(project.ID); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load blueprints"})
	}
	var response []fiber.Map = make([]fiber.Map, 0, len(blueprints))
	for _, blueprint := range blueprints {
		var versions []*db.BlueprintVersion
		if versions, errResult = db.BlueprintVersionsForBlueprint(blueprint.ID); errResult != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load blueprint versions"})
		}
		var projected []fiber.Map = make([]fiber.Map, 0, len(versions))
		for _, version := range versions {
			var document db.BlueprintDocument
			if document, errResult = db.BlueprintDocumentForVersion(version); errResult != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "stored blueprint document is invalid"})
			}
			projected = append(projected, fiber.Map{"id": version.ID, "uuid": version.UUID, "version": version.Version, "document_digest": version.DocumentDigest, "document": document, "created_at": version.CreatedAt})
		}
		response = append(response, fiber.Map{"id": blueprint.ID, "uuid": blueprint.UUID, "project_id": blueprint.ProjectID, "name": blueprint.Name, "slug": blueprint.Slug, "description": blueprint.Description, "versions": projected, "created_at": blueprint.CreatedAt})
	}
	return c.JSON(response)
}

// postProjectBlueprint creates reusable blueprint metadata in a project.
func postProjectBlueprint(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	if project, errResult = requireBlueprintProject(c, true); errResult != nil || project == nil {
		return
	}
	var request blueprintCreateRequest
	if errResult = c.BodyParser(&request); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	var blueprint *db.Blueprint
	if blueprint, errResult = db.CreateBlueprint(project.ID, request.Name, request.Slug, request.Description); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "blueprint.create", "blueprint", &blueprint.ID, &project.ID, map[string]any{"slug": blueprint.Slug})
	return c.Status(fiber.StatusCreated).JSON(blueprint)
}

// postBlueprintVersion publishes a validated immutable runner document.
func postBlueprintVersion(c *fiber.Ctx) (errResult error) {
	var blueprintID int
	if blueprintID, errResult = c.ParamsInt("id"); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid blueprint id"})
	}
	var blueprint *db.Blueprint
	if blueprint, errResult = db.Blueprints.Select(blueprintID); errResult != nil || blueprint == nil || blueprint.ArchivedAt != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "blueprint not found"})
	}
	var allowed bool
	if allowed, errResult = currentUserCanManageProjectID(c, blueprint.ProjectID); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var request blueprintVersionRequest
	if errResult = c.BodyParser(&request); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	var actorID *int
	if currentDBUser(c) != nil {
		actorID = &currentDBUser(c).ID
	}
	var version *db.BlueprintVersion
	if version, errResult = db.PublishBlueprintVersion(blueprint.ID, request.Document, actorID); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "blueprint.version.publish", "blueprint_version", &version.ID, &blueprint.ProjectID, map[string]any{"blueprint_id": blueprint.ID, "version": version.Version, "digest": version.DocumentDigest})
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": version.ID, "uuid": version.UUID, "version": version.Version, "document_digest": version.DocumentDigest, "document": request.Document, "created_at": version.CreatedAt})
}

// postProjectDeploymentPreview expands a version for groups without reserving or mutating infrastructure.
func postProjectDeploymentPreview(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	if project, errResult = requireBlueprintProject(c, true); errResult != nil || project == nil {
		return
	}
	var request deploymentPreviewRequest
	if errResult = c.BodyParser(&request); errResult != nil || request.BlueprintVersionID <= 0 || len(request.GroupIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "blueprintVersionID and groupIDs are required"})
	}
	var version *db.BlueprintVersion
	if version, errResult = db.BlueprintVersions.Select(request.BlueprintVersionID); errResult != nil || version == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "blueprint version not found"})
	}
	var blueprint *db.Blueprint
	if blueprint, errResult = db.Blueprints.Select(version.BlueprintID); errResult != nil || blueprint == nil || blueprint.ProjectID != project.ID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "blueprint version not found in project"})
	}
	var document db.BlueprintDocument
	if document, errResult = db.BlueprintDocumentForVersion(version); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "stored blueprint document is invalid"})
	}
	var groups []*db.CloudGroup
	if groups, errResult = db.CloudGroupsForOwner(db.RoleBindingScopeProject, &project.ID); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project groups"})
	}
	var groupsByID map[int]*db.CloudGroup = make(map[int]*db.CloudGroup, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = group
	}
	var prefix string = strings.Trim(strings.ToLower(strings.TrimSpace(request.NamePrefix)), "-")
	if prefix == "" {
		prefix = project.Slug
	}
	var deployments []fiber.Map = make([]fiber.Map, 0, len(request.GroupIDs))
	var totals db.QuotaDimensions
	var seen map[int]bool = make(map[int]bool, len(request.GroupIDs))
	for index, groupID := range request.GroupIDs {
		var group *db.CloudGroup = groupsByID[groupID]
		if group == nil || seen[groupID] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "groupIDs must be unique project groups"})
		}
		seen[groupID] = true
		var deploymentName string = fmt.Sprintf("%s-g%02d", prefix, index+1)
		var resources []fiber.Map = make([]fiber.Map, 0, len(document.Resources))
		for _, resource := range document.Resources {
			var name string = strings.ReplaceAll(document.NamePattern, "{{deployment}}", deploymentName)
			name = strings.ReplaceAll(name, "{{resource}}", resource.Key)
			resources = append(resources, fiber.Map{"key": resource.Key, "name": name, "kind": resource.Kind, "template": resource.Template, "vcpu": resource.VCPU, "memory_mb": resource.MemoryMB, "disk_gb": resource.DiskGB, "networks": resource.Networks, "configuration_role": resource.ConfigurationRole})
			totals.VMs++
			totals.VCPU += resource.VCPU
			totals.MemoryMB += resource.MemoryMB
			totals.StorageGB += resource.DiskGB
		}
		totals.Networks += len(document.Networks)
		deployments = append(deployments, fiber.Map{"name": deploymentName, "group": fiber.Map{"id": group.ID, "name": group.Name, "slug": group.Slug}, "resources": resources, "networks": document.Networks})
	}
	return c.JSON(fiber.Map{"blueprint": fiber.Map{"id": blueprint.ID, "name": blueprint.Name, "version": version.Version, "digest": version.DocumentDigest}, "runner": fiber.Map{"opentofu_module": document.OpenTofuModule, "ansible_project": document.AnsibleProject}, "deployments": deployments, "totals": totals, "mutates": false})
}

func requireBlueprintProject(c *fiber.Ctx, manage bool) (projectResult *db.Project, errResult error) {
	var projectID int
	if projectID, errResult = c.ParamsInt("id"); errResult != nil {
		errResult = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
		return
	}
	var found bool
	if projectResult, found, errResult = db.GetProjectByID(projectID); errResult != nil || !found || !projectResult.IsActive {
		errResult = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
		return
	}
	var allowed bool
	if manage {
		allowed, errResult = currentUserCanManageProject(c, projectResult)
	} else {
		allowed, errResult = currentUserCanViewProject(c, projectResult)
	}
	if errResult != nil {
		errResult = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	} else if !allowed {
		errResult = c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
		projectResult = nil
	}
	return
}

func currentUserCanManageProjectID(c *fiber.Ctx, projectID int) (allowedResult bool, errResult error) {
	var project *db.Project
	var found bool
	if project, found, errResult = db.GetProjectByID(projectID); errResult != nil || !found {
		return false, errResult
	}
	return currentUserCanManageProject(c, project)
}

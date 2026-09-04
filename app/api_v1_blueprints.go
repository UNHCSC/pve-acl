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
		BlueprintVersionID int            `json:"blueprintVersionID"`
		GroupIDs           []int          `json:"groupIDs"`
		NamePrefix         string         `json:"namePrefix"`
		AllocationPoolIDs  map[string]int `json:"allocationPoolIDs"`
	}

	allocationPoolRequest struct {
		Name  string `json:"name"`
		Kind  string `json:"kind"`
		Start int    `json:"start"`
		End   int    `json:"end"`
		CIDR  string `json:"cidr"`
	}
)

// getProjectDeployments lists durable desired-state plans without implying provisioning.
func getProjectDeployments(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	if project, errResult = requireBlueprintProject(c, false); errResult != nil || project == nil {
		return
	}
	var deployments []*db.Deployment
	if deployments, errResult = db.DeploymentsForProject(project.ID); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load deployments"})
	}
	return c.JSON(deployments)
}

// postProjectDeployment commits a validated plan and reserves its quota and pool values.
func postProjectDeployment(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	if project, errResult = requireBlueprintProject(c, true); errResult != nil || project == nil {
		return
	}
	var request deploymentPreviewRequest
	if errResult = c.BodyParser(&request); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	var result db.DeploymentPlanResult
	if result, errResult = db.CreateDeploymentPlan(db.DeploymentPlanInput{ProjectID: project.ID, BlueprintVersionID: request.BlueprintVersionID, GroupIDs: request.GroupIDs, NamePrefix: request.NamePrefix, AllocationPoolIDs: request.AllocationPoolIDs}); errResult != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": errResult.Error()})
	}
	var deploymentIDs []int
	for _, deployment := range result.Deployments {
		deploymentIDs = append(deploymentIDs, deployment.ID)
	}
	_ = auditRequest(c, "deployment.plan.create", "deployment", nil, &project.ID, map[string]any{"deployment_ids": deploymentIDs, "blueprint_version_id": request.BlueprintVersionID, "allocation_count": len(result.Allocations), "quota_reservation_id": result.Quota.ID})
	return c.Status(fiber.StatusCreated).JSON(result)
}

// getProjectAllocationPools lists capacity available for deployment planning.
func getProjectAllocationPools(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	if project, errResult = requireBlueprintProject(c, true); errResult != nil || project == nil {
		return
	}
	var pools []*db.AllocationPool
	if pools, errResult = db.AllocationPoolsForProject(project.ID); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load allocation pools"})
	}
	var response []fiber.Map = make([]fiber.Map, 0, len(pools))
	for _, pool := range pools {
		var available int
		if available, errResult = db.AllocationPoolAvailable(pool); errResult != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to calculate allocation capacity"})
		}
		response = append(response, fiber.Map{"id": pool.ID, "uuid": pool.UUID, "project_id": pool.ProjectID, "name": pool.Name, "kind": pool.Kind, "start": pool.Start, "end": pool.End, "cidr": pool.CIDR, "available": available})
	}
	return c.JSON(response)
}

// postProjectAllocationPool creates a project-scoped allocation boundary.
func postProjectAllocationPool(c *fiber.Ctx) (errResult error) {
	var project *db.Project
	if project, errResult = requireBlueprintProject(c, true); errResult != nil || project == nil {
		return
	}
	var request allocationPoolRequest
	if errResult = c.BodyParser(&request); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	var pool *db.AllocationPool
	if pool, errResult = db.CreateAllocationPool(project.ID, request.Name, request.Kind, request.Start, request.End, request.CIDR); errResult != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": errResult.Error()})
	}
	_ = auditRequest(c, "allocation_pool.create", "allocation_pool", &pool.ID, &project.ID, map[string]any{"kind": pool.Kind, "start": pool.Start, "end": pool.End, "cidr": pool.CIDR})
	return c.Status(fiber.StatusCreated).JSON(pool)
}

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
			if resource.Kind == "ct" {
				totals.Containers++
			} else {
				totals.VMs++
			}
			totals.VCPU += resource.VCPU
			totals.MemoryMB += resource.MemoryMB
			totals.StorageGB += resource.DiskGB
		}
		totals.Networks += len(document.Networks)
		for _, network := range document.Networks {
			if network.Public {
				totals.PublicIPs++
			}
		}
		deployments = append(deployments, fiber.Map{"name": deploymentName, "group": fiber.Map{"id": group.ID, "name": group.Name, "slug": group.Slug}, "resources": resources, "networks": document.Networks})
	}
	var allocationNeeds map[string]int = map[string]int{"vmid": totals.VMs + totals.Containers, "vlan": totals.Networks}
	for _, network := range document.Networks {
		if network.Public {
			allocationNeeds["ipv4"] += len(request.GroupIDs)
			allocationNeeds["external_port"] += len(request.GroupIDs)
		}
	}
	if errResult = db.CheckProjectQuota(project.ID, totals); errResult != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": errResult.Error()})
	}
	var capacity []fiber.Map
	for kind, needed := range allocationNeeds {
		var poolID int = request.AllocationPoolIDs[kind]
		if poolID == 0 {
			continue
		}
		var pool *db.AllocationPool
		if pool, errResult = db.AllocationPools.Select(poolID); errResult != nil || pool == nil || pool.ProjectID != project.ID || pool.Kind != kind || pool.ArchivedAt != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "allocation pool does not match project or kind " + kind})
		}
		var available int
		if available, errResult = db.AllocationPoolAvailable(pool); errResult != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to calculate allocation capacity"})
		}
		if available < needed {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": fmt.Sprintf("%s pool has %d available but %d are required", kind, available, needed)})
		}
		capacity = append(capacity, fiber.Map{"kind": kind, "pool_id": pool.ID, "needed": needed, "available": available})
	}
	return c.JSON(fiber.Map{"blueprint": fiber.Map{"id": blueprint.ID, "name": blueprint.Name, "version": version.Version, "digest": version.DocumentDigest}, "runner": fiber.Map{"opentofu_module": document.OpenTofuModule, "ansible_project": document.AnsibleProject}, "deployments": deployments, "totals": totals, "allocation_needs": allocationNeeds, "allocation_capacity": capacity, "mutates": false})
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

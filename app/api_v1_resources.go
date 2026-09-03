package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
	"github.com/UNHCSC/organesson/proxmox"
	"github.com/gofiber/fiber/v2"
)

type (
	projectResourceRequest struct {
		Name                    string `json:"name"`
		Slug                    string `json:"slug"`
		ResourceType            string `json:"resourceType"`
		Status                  string `json:"status"`
		ProxmoxInventoryGuestID *int   `json:"proxmoxInventoryGuestID"`
	}

	assetGroupRequest struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}

	assetAssignmentRequest struct {
		TargetType   string `json:"targetType"`
		ResourceID   *int   `json:"resourceID"`
		AssetGroupID *int   `json:"assetGroupID"`
		SubjectType  string `json:"subjectType"`
		SubjectID    int    `json:"subjectID"`
		SubjectRef   string `json:"subjectRef"`
		RoleID       int    `json:"roleID"`
	}
)

// getProjectResources lists visible local inventory resources for a project.
func getProjectResources(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		err     error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	var resources []*db.Resource

	resources, err = db.ResourcesForProject(project.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load resources"})
	}
	var (
		projectVisible bool
		allowErr       error
	)
	projectVisible, allowErr = currentUserCanViewProject(c, project)
	if allowErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	var visible []*db.Resource

	visible = make([]*db.Resource, 0, len(resources))
	for _, resource := range resources {
		var (
			allowed bool
			err     error
		)
		allowed, err = currentUserCanViewResource(c, project, resource)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		}
		if allowed {
			visible = append(visible, resource)
		}
	}
	if !projectVisible && len(visible) == 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var items []fiber.Map

	items, err = projectResourceResponse(visible)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load resource metadata"})
	}
	if err = addResourceCapabilities(c, project, visible, items); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	return c.JSON(items)
}

func addResourceCapabilities(c *fiber.Ctx, project *db.Project, resources []*db.Resource, items []fiber.Map) (errResult error) {
	var permissions []db.PermissionKey = []db.PermissionKey{db.PermissionVMStart, db.PermissionVMStop, db.PermissionVMReboot, db.PermissionVMConsole}
	var names []string = []string{"can_start", "can_stop", "can_reboot", "can_console"}
	for index, resource := range resources {
		for permissionIndex, permission := range permissions {
			var allowed bool
			if allowed, errResult = currentUserCan(c, permission, db.RoleBindingScopeResource, &resource.ID); errResult != nil {
				return
			}
			if !allowed {
				if allowed, errResult = currentUserCan(c, permission, db.RoleBindingScopeProject, &project.ID); errResult != nil {
					return
				}
			}
			items[index][names[permissionIndex]] = allowed
		}
	}
	return
}

// postCreateProjectResource creates a local inventory resource.
func postCreateProjectResource(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		err     error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	var req projectResourceRequest

	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid resource request"})
	}
	var (
		resourceType db.ResourceType
		ok           bool
	)
	resourceType, ok = parseLocalResourceType(req.ResourceType)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported resource type"})
	}
	if strings.TrimSpace(req.Status) != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "resource status is server-managed"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		var permission db.PermissionKey

		permission = resourceCreatePermission(resourceType)
		allowed, err = currentUserCan(c, permission, db.RoleBindingScopeProject, &project.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		}
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var createdByUserID *int
	var current *db.User

	current = currentDBUser(c)
	if current != nil {
		createdByUserID = &current.ID
	}
	var resource *db.Resource
	if req.ProxmoxInventoryGuestID != nil {
		resource, err = importProxmoxProjectResource(c, project, req, createdByUserID)
	} else {
		resource, err = db.CreateResource(db.ResourceCreateInput{
			ProjectID:       project.ID,
			Name:            req.Name,
			Slug:            req.Slug,
			ResourceType:    resourceType,
			Status:          db.ResourceStatusReady,
			CreatedByUserID: createdByUserID,
		})
	}
	if err != nil {
		if fiberError, ok := err.(*fiber.Error); ok {
			return c.Status(fiberError.Code).JSON(fiber.Map{"error": fiberError.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var items []fiber.Map

	items, err = projectResourceResponse([]*db.Resource{resource})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load resource metadata"})
	}
	if err = auditRequest(c, "resource.create", "resource", &resource.ID, &project.ID, map[string]any{"name": resource.Name, "resourceType": resourceTypeLabel(resource.ResourceType)}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to record audit event"})
	}
	return c.Status(fiber.StatusCreated).JSON(items[0])
}

func importProxmoxProjectResource(c *fiber.Ctx, project *db.Project, req projectResourceRequest, createdByUserID *int) (resourceResult *db.Resource, errResult error) {
	if !proxmoxIntegration.enabled || proxmoxIntegration.service == nil {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "Proxmox integration is disabled")
	}
	if req.ResourceType != "vm" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "a Proxmox guest must be imported as a virtual machine")
	}
	var inventory *db.ProxmoxInventoryGuest
	if inventory, errResult = db.ProxmoxInventoryGuests.Select(*req.ProxmoxInventoryGuestID); errResult != nil || inventory == nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "managed Proxmox guest was not found")
	}
	if inventory.ClusterIdentity != proxmoxIntegration.clusterIdentity || inventory.ResourceID != nil || inventory.MissingSince != nil || inventory.DriftState == db.ProxmoxDriftAmbiguous || inventory.DriftState == db.ProxmoxDriftError {
		return nil, fiber.NewError(fiber.StatusConflict, "managed Proxmox guest is not available for import")
	}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(c.UserContext(), 20*time.Second)
	defer cancel()
	var guest proxmox.Guest
	if guest, errResult = proxmoxIntegration.service.GetGuest(ctx, inventory.Node, inventory.ProxmoxVMID); errResult != nil {
		return nil, fiber.NewError(fiber.StatusBadGateway, "failed to verify managed Proxmox guest")
	}
	if !proxmox.HasTag(guest, proxmoxIntegration.managedTag) {
		return nil, fiber.NewError(fiber.StatusConflict, "guest no longer has the required managed tag")
	}
	var baseURL string
	if baseURL, errResult = proxmoxBaseURL(config.Config.Proxmox.Hostname, config.Config.Proxmox.Port); errResult != nil {
		return
	}
	return db.ImportProxmoxGuest(db.ProxmoxGuestImportInput{ProjectID: project.ID, Name: req.Name, Slug: req.Slug, ClusterIdentity: proxmoxIntegration.clusterIdentity, APIURL: baseURL, VerifyTLS: config.Config.Proxmox.VerifyTLS, Guest: guest, CreatedByUserID: createdByUserID})
}

// patchProjectResource updates a local inventory resource.
func patchProjectResource(c *fiber.Ctx) (errResult error) {
	var (
		project  *db.Project
		resource *db.Resource
		err      error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	resource, err = projectResourceFromParam(c, project.ID)
	if err != nil {
		return resourceParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var req projectResourceRequest

	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid resource request"})
	}
	if strings.TrimSpace(req.Status) != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "resource status is server-managed"})
	}
	if err = db.UpdateResource(resource, db.ResourceUpdateInput{
		Name: req.Name,
		Slug: req.Slug,
	}); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var items []fiber.Map

	items, err = projectResourceResponse([]*db.Resource{resource})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load resource metadata"})
	}
	if err = auditRequest(c, "resource.update", "resource", &resource.ID, &project.ID, map[string]any{"name": resource.Name}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to record audit event"})
	}
	return c.JSON(items[0])
}

// deleteProjectResource deletes a local inventory resource.
func deleteProjectResource(c *fiber.Ctx) (errResult error) {
	var (
		project  *db.Project
		resource *db.Resource
		err      error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	resource, err = projectResourceFromParam(c, project.ID)
	if err != nil {
		return resourceParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		allowed, err = currentUserCanDeleteResource(c, project, resource)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		}
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	if err = db.ArchiveAssetAssignmentsForResource(resource.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove resource assignments"})
	}
	if err = db.ArchiveResource(resource); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete resource"})
	}
	if err = auditRequest(c, "resource.archive", "resource", &resource.ID, &project.ID, nil); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to record audit event"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// getProjectAssetGroups lists project asset groups.
func getProjectAssetGroups(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		err     error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanViewProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var groups []*db.AssetGroup

	groups, err = db.AssetGroupsForProject(project.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset groups"})
	}
	var items []fiber.Map

	items, err = assetGroupResponse(groups)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset group metadata"})
	}
	return c.JSON(items)
}

// postCreateProjectAssetGroup creates a project asset group.
func postCreateProjectAssetGroup(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		err     error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var req assetGroupRequest

	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid asset group request"})
	}
	var group *db.AssetGroup

	group, err = db.CreateAssetGroup(db.AssetGroupCreateInput{
		ProjectID:   project.ID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var items []fiber.Map

	items, err = assetGroupResponse([]*db.AssetGroup{group})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset group metadata"})
	}
	return c.Status(fiber.StatusCreated).JSON(items[0])
}

// patchProjectAssetGroup updates an asset group's editable fields.
func patchProjectAssetGroup(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		group   *db.AssetGroup
		err     error
	)

	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	group, err = projectAssetGroupFromParam(c, project.ID)
	if err != nil {
		return assetGroupParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var req assetGroupRequest

	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid asset group request"})
	}
	if err = db.UpdateAssetGroup(group, db.AssetGroupUpdateInput{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
	}); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var items []fiber.Map

	items, err = assetGroupResponse([]*db.AssetGroup{group})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset group metadata"})
	}
	return c.JSON(items[0])
}

// deleteProjectAssetGroup archives an asset group and removes its assignment grants.
func deleteProjectAssetGroup(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		group   *db.AssetGroup
		err     error
	)

	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	group, err = projectAssetGroupFromParam(c, project.ID)
	if err != nil {
		return assetGroupParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	if err = db.ArchiveAssetGroup(group); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to archive asset group"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// postProjectAssetGroupResource adds a resource to an asset group.
func postProjectAssetGroupResource(c *fiber.Ctx) (errResult error) {
	var (
		project  *db.Project
		group    *db.AssetGroup
		resource *db.Resource
		err      error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	group, err = projectAssetGroupFromParam(c, project.ID)
	if err != nil {
		return assetGroupParamError(c, err)
	}
	resource, err = projectResourceFromParam(c, project.ID)
	if err != nil {
		return resourceParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	if _, err = db.EnsureAssetGroupResource(group.ID, resource.ID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var items []fiber.Map

	items, err = assetGroupResponse([]*db.AssetGroup{group})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset group metadata"})
	}
	return c.Status(fiber.StatusCreated).JSON(items[0])
}

// deleteProjectAssetGroupResource removes a resource from an asset group.
func deleteProjectAssetGroupResource(c *fiber.Ctx) (errResult error) {
	var (
		project  *db.Project
		group    *db.AssetGroup
		resource *db.Resource
		err      error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	group, err = projectAssetGroupFromParam(c, project.ID)
	if err != nil {
		return assetGroupParamError(c, err)
	}
	resource, err = projectResourceFromParam(c, project.ID)
	if err != nil {
		return resourceParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	if err = db.RemoveAssetGroupResource(group.ID, resource.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove asset group resource"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// getProjectAssetAssignments lists project asset assignments.
func getProjectAssetAssignments(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		err     error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanViewProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var assignments []*db.AssetAssignment

	assignments, err = db.AssetAssignmentsForProject(project.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset assignments"})
	}
	var items []fiber.Map

	items, err = assetAssignmentResponse(assignments)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset assignment metadata"})
	}
	return c.JSON(items)
}

// postCreateProjectAssetAssignment creates a resource or asset group assignment.
func postCreateProjectAssetAssignment(c *fiber.Ctx) (errResult error) {
	var (
		project *db.Project
		err     error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var req assetAssignmentRequest

	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid asset assignment request"})
	}
	var (
		subjectType db.RoleBindingSubject
		ok          bool
	)
	subjectType, ok = parseAssetAssignmentSubjectType(req.SubjectType)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported subject type"})
	}
	var subjectID int

	subjectID, err = resolveLocalAssetAssignmentSubject(subjectType, req.SubjectID, req.SubjectRef)
	if err != nil {
		return requestError(c, err)
	}
	var input db.AssetAssignmentInput

	input = db.AssetAssignmentInput{
		ProjectID:   project.ID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      req.RoleID,
	}
	var scopeID *int
	var scopeType db.RoleBindingScope

	if strings.EqualFold(strings.TrimSpace(req.TargetType), "assetGroup") || strings.EqualFold(strings.TrimSpace(req.TargetType), "asset_group") {
		if req.AssetGroupID == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "asset group is required"})
		}
		var group *db.AssetGroup

		group, err = projectAssetGroupByID(project.ID, *req.AssetGroupID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		input.AssetGroupID = &group.ID
		scopeType = db.RoleBindingScopeProject
		scopeID = &project.ID
	} else if strings.EqualFold(strings.TrimSpace(req.TargetType), "resource") {
		if req.ResourceID == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "resource is required"})
		}
		var resource *db.Resource

		resource, err = projectResourceByID(project.ID, *req.ResourceID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		input.ResourceID = &resource.ID
		scopeType = db.RoleBindingScopeResource
		scopeID = &resource.ID
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported assignment target"})
	}
	allowed, err = currentUserCanAssignRoleAtScope(c, req.RoleID, scopeType, scopeID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "role is outside your scoped access"})
	}
	var current *db.User

	current = currentDBUser(c)
	if current != nil {
		input.CreatedByUserID = &current.ID
	}
	var assignment *db.AssetAssignment

	assignment, _, err = db.EnsureAssetAssignment(input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err = auditRequest(c, "access.assignment.create", "asset_assignment", &assignment.ID, &project.ID, map[string]any{"subjectType": req.SubjectType, "subjectID": subjectID}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to record audit event"})
	}
	var items []fiber.Map

	items, err = assetAssignmentResponse([]*db.AssetAssignment{assignment})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset assignment metadata"})
	}
	return c.Status(fiber.StatusCreated).JSON(items[0])
}

// deleteProjectAssetAssignment archives an assignment and removes derived grants.
func deleteProjectAssetAssignment(c *fiber.Ctx) (errResult error) {
	var (
		project    *db.Project
		assignment *db.AssetAssignment
		err        error
	)
	project, err = projectFromIDParam(c)
	if err != nil {
		return projectParamError(c, err)
	}
	var assignmentID int

	assignmentID, err = strconv.Atoi(c.Params("assignmentID"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid assignment id"})
	}
	assignment, err = db.AssetAssignments.Select(assignmentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset assignment"})
	}
	if assignment == nil || assignment.ProjectID != project.ID || assignment.ArchivedAt != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "asset assignment not found"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	if err = db.ArchiveAssetAssignment(assignment.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove asset assignment"})
	}
	if err = auditRequest(c, "access.assignment.archive", "asset_assignment", &assignment.ID, &project.ID, nil); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to record audit event"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func projectResourceResponse(resources []*db.Resource) (itemsResult []fiber.Map, errResult error) {
	var items []fiber.Map

	items = make([]fiber.Map, 0, len(resources))
	for _, resource := range resources {
		var (
			assetGroupLinks []*db.AssetGroupResource
			assignments     []*db.AssetAssignment
			err             error
		)
		assetGroupLinks, err = db.AssetGroupResourcesForResource(resource.ID)
		if err != nil {
			return nil, err
		}
		assignments, err = db.AssetAssignmentsForResource(resource.ID)
		if err != nil {
			return nil, err
		}
		var assignmentCount int

		assignmentCount = len(assignments)
		for _, link := range assetGroupLinks {
			var groupAssignments []*db.AssetAssignment

			groupAssignments, err = db.AssetAssignmentsForAssetGroup(link.AssetGroupID)
			if err != nil {
				return nil, err
			}
			assignmentCount += len(groupAssignments)
		}
		items = append(items, fiber.Map{
			"id":                  resource.ID,
			"uuid":                resource.UUID,
			"project_id":          resource.ProjectID,
			"name":                resource.Name,
			"slug":                resource.Slug,
			"resource_type":       resource.ResourceType,
			"resource_type_label": resourceTypeLabel(resource.ResourceType),
			"status":              resource.Status,
			"status_label":        resourceStatusLabel(resource.Status),
			"asset_group_count":   len(assetGroupLinks),
			"assignment_count":    assignmentCount,
			"created_at":          resource.CreatedAt,
			"updated_at":          resource.UpdatedAt,
			"deleted_at":          resource.DeletedAt,
		})
		if resource.ResourceType == db.ResourceTypeVM {
			var machine *db.VirtualMachine
			var found bool
			if machine, found, err = db.VirtualMachineForResource(resource.ID); err != nil {
				return nil, err
			}
			if found {
				if proxmoxIntegration.enabled && proxmoxIntegration.service != nil {
					var refreshContext context.Context
					var cancel context.CancelFunc
					refreshContext, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					var guest proxmox.Guest
					if guest, err = managedMachineGuest(refreshContext, machine); err == nil {
						if err = db.UpdateVirtualMachinePower(machine, db.PowerStateFromProxmox(guest.Status)); err != nil {
							cancel()
							return nil, err
						}
					}
					cancel()
				}
				items[len(items)-1]["power_state"] = machine.PowerState
				items[len(items)-1]["power_updated_at"] = machine.UpdatedAt
				items[len(items)-1]["proxmox_vmid"] = machine.ProxmoxVMID
				items[len(items)-1]["proxmox_node"] = machineNode(machine)
				items[len(items)-1]["cpu_cores"] = machine.CPUCores
				items[len(items)-1]["memory_mb"] = machine.MemoryMB
				items[len(items)-1]["disk_gb"] = machine.DiskGB
				items[len(items)-1]["os_type"] = machine.OSType
			}
		}
	}
	return items, nil
}

func assetGroupResponse(groups []*db.AssetGroup) (itemsResult []fiber.Map, errResult error) {
	var items []fiber.Map

	items = make([]fiber.Map, 0, len(groups))
	for _, group := range groups {
		var (
			links       []*db.AssetGroupResource
			assignments []*db.AssetAssignment
			resources   []fiber.Map
			err         error
		)
		links, err = db.AssetGroupResourcesForGroup(group.ID)
		if err != nil {
			return nil, err
		}
		assignments, err = db.AssetAssignmentsForAssetGroup(group.ID)
		if err != nil {
			return nil, err
		}
		resources = make([]fiber.Map, 0, len(links))
		for _, link := range links {
			var resource *db.Resource

			resource, err = db.Resources.Select(link.ResourceID)
			if err != nil {
				return nil, err
			}
			if resource == nil || resource.DeletedAt != nil {
				continue
			}
			resources = append(resources, fiber.Map{
				"id":                  resource.ID,
				"name":                resource.Name,
				"slug":                resource.Slug,
				"resource_type":       resource.ResourceType,
				"resource_type_label": resourceTypeLabel(resource.ResourceType),
				"status_label":        resourceStatusLabel(resource.Status),
			})
		}
		items = append(items, fiber.Map{
			"id":               group.ID,
			"uuid":             group.UUID,
			"project_id":       group.ProjectID,
			"name":             group.Name,
			"slug":             group.Slug,
			"description":      group.Description,
			"resource_count":   len(resources),
			"assignment_count": len(assignments),
			"resources":        resources,
			"created_at":       group.CreatedAt,
			"updated_at":       group.UpdatedAt,
			"archived_at":      group.ArchivedAt,
		})
	}
	return items, nil
}

func assetAssignmentResponse(assignments []*db.AssetAssignment) (itemsResult []fiber.Map, errResult error) {
	var items []fiber.Map

	items = make([]fiber.Map, 0, len(assignments))
	for _, assignment := range assignments {
		var (
			role    *db.Role
			subject fiber.Map
			err     error
		)
		role, err = db.Roles.Select(assignment.RoleID)
		if err != nil {
			return nil, err
		}
		subject, err = roleBindingSubjectResponse(assignment.SubjectType, assignment.SubjectID)
		if err != nil {
			return nil, err
		}
		var item fiber.Map

		item = fiber.Map{
			"id":                 assignment.ID,
			"project_id":         assignment.ProjectID,
			"resource_id":        assignment.ResourceID,
			"asset_group_id":     assignment.AssetGroupID,
			"subject_type":       assignment.SubjectType,
			"subject_type_label": roleBindingSubjectLabel(assignment.SubjectType),
			"subject_id":         assignment.SubjectID,
			"subject":            subject,
			"role_id":            assignment.RoleID,
			"created_at":         assignment.CreatedAt,
			"archived_at":        assignment.ArchivedAt,
		}
		if role != nil {
			item["role"] = fiber.Map{
				"id":          role.ID,
				"name":        role.Name,
				"description": role.Description,
			}
		}
		if assignment.ResourceID != nil {
			var resource *db.Resource

			resource, err = db.Resources.Select(*assignment.ResourceID)
			if err != nil {
				return nil, err
			}
			item["target_type"] = "resource"
			if resource != nil {
				item["target"] = fiber.Map{
					"id":                  resource.ID,
					"name":                resource.Name,
					"slug":                resource.Slug,
					"resource_type":       resource.ResourceType,
					"resource_type_label": resourceTypeLabel(resource.ResourceType),
					"label":               resource.Name,
				}
			}
		}
		if assignment.AssetGroupID != nil {
			var group *db.AssetGroup

			group, err = db.AssetGroups.Select(*assignment.AssetGroupID)
			if err != nil {
				return nil, err
			}
			item["target_type"] = "assetGroup"
			if group != nil {
				item["target"] = fiber.Map{
					"id":    group.ID,
					"name":  group.Name,
					"slug":  group.Slug,
					"label": group.Name,
				}
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func currentUserCanViewResource(c *fiber.Ctx, project *db.Project, resource *db.Resource) (okResult bool, errResult error) {
	var (
		allowed bool
		err     error
	)
	allowed, err = currentUserCanManageProject(c, project)
	if err != nil || allowed {
		return allowed, err
	}
	var permission db.PermissionKey

	permission = resourceReadPermission(resource.ResourceType)
	allowed, err = currentUserCan(c, permission, db.RoleBindingScopeProject, &project.ID)
	if err != nil || allowed {
		return allowed, err
	}
	return currentUserCan(c, permission, db.RoleBindingScopeResource, &resource.ID)
}

func resourceReadPermission(resourceType db.ResourceType) (permissionResult db.PermissionKey) {
	switch resourceType {
	case db.ResourceTypeCT:
		return db.PermissionCTRead
	case db.ResourceTypeNetwork:
		return db.PermissionNetworkRead
	default:
		return db.PermissionVMRead
	}
}

func resourceCreatePermission(resourceType db.ResourceType) (permissionResult db.PermissionKey) {
	switch resourceType {
	case db.ResourceTypeCT:
		return db.PermissionCTCreate
	case db.ResourceTypeNetwork:
		return db.PermissionNetworkCreate
	default:
		return db.PermissionVMCreate
	}
}

func currentUserCanDeleteResource(c *fiber.Ctx, project *db.Project, resource *db.Resource) (okResult bool, errResult error) {
	var (
		allowed    bool
		permission db.PermissionKey
		err        error
	)

	permission = resourceDeletePermission(resource.ResourceType)
	allowed, err = currentUserCan(c, permission, db.RoleBindingScopeProject, &project.ID)
	if err != nil || allowed {
		return allowed, err
	}
	return currentUserCan(c, permission, db.RoleBindingScopeResource, &resource.ID)
}

func resourceDeletePermission(resourceType db.ResourceType) (permissionResult db.PermissionKey) {
	switch resourceType {
	case db.ResourceTypeCT:
		return db.PermissionCTDelete
	case db.ResourceTypeNetwork:
		return db.PermissionNetworkDelete
	default:
		return db.PermissionVMDelete
	}
}

func parseLocalResourceType(value string) (resourceTypeResult db.ResourceType, okResult bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "vm":
		return db.ResourceTypeVM, true
	case "container", "ct":
		return db.ResourceTypeCT, true
	case "network":
		return db.ResourceTypeNetwork, true
	default:
		return db.ResourceTypeVM, false
	}
}

func resourceTypeLabel(value db.ResourceType) (valueResult string) {
	switch value {
	case db.ResourceTypeCT:
		return "container"
	case db.ResourceTypeNetwork:
		return "network"
	default:
		return "vm"
	}
}

func resourceStatusLabel(value db.ResourceStatus) (valueResult string) {
	switch value {
	case db.ResourceStatusCreating:
		return "creating"
	case db.ResourceStatusUpdating:
		return "updating"
	case db.ResourceStatusDeleting:
		return "deleting"
	case db.ResourceStatusDeleted:
		return "deleted"
	case db.ResourceStatusError:
		return "error"
	case db.ResourceStatusUnknown:
		return "unknown"
	default:
		return "ready"
	}
}

func parseAssetAssignmentSubjectType(value string) (subjectTypeResult db.RoleBindingSubject, okResult bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user":
		return db.RoleBindingSubjectUser, true
	case "group":
		return db.RoleBindingSubjectGroup, true
	default:
		return db.RoleBindingSubjectUser, false
	}
}

func resolveLocalAssetAssignmentSubject(subjectType db.RoleBindingSubject, subjectID int, subjectRef string) (countResult int, errResult error) {
	if subjectID > 0 {
		return subjectID, nil
	}
	subjectRef = strings.TrimSpace(subjectRef)
	if subjectRef == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "subject is required")
	}
	if subjectType == db.RoleBindingSubjectGroup {
		var (
			group *db.CloudGroup
			found bool
			err   error
		)
		group, found, err = findCloudGroupByRef(subjectRef)
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, fiber.NewError(fiber.StatusBadRequest, "group was not found")
		}
		return group.ID, nil
	}
	var (
		users []*db.User
		err   error
	)

	users, err = db.ListUsers()
	if err != nil {
		return 0, err
	}
	for _, user := range users {
		if strings.EqualFold(user.Username, subjectRef) || strings.EqualFold(user.Email, subjectRef) || strings.EqualFold(user.DisplayName, subjectRef) {
			return user.ID, nil
		}
	}
	return 0, fiber.NewError(fiber.StatusBadRequest, "user was not found")
}

func projectFromIDParam(c *fiber.Ctx) (projectResult *db.Project, errResult error) {
	var (
		projectID int
		err       error
	)
	projectID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid project id")
	}
	var (
		project *db.Project
		found   bool
	)
	project, found, err = db.GetProjectByID(projectID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to load project")
	}
	if !found || !project.IsActive {
		return nil, fiber.NewError(fiber.StatusNotFound, "project not found")
	}
	return project, nil
}

func projectParamError(c *fiber.Ctx, err error) (errResult error) {
	var (
		fiberErr *fiber.Error
		ok       bool
	)
	if fiberErr, ok = err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
}

func projectResourceFromParam(c *fiber.Ctx, projectID int) (resourceResult *db.Resource, errResult error) {
	var (
		resourceID int
		err        error
	)
	resourceID, err = strconv.Atoi(c.Params("resourceID"))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid resource id")
	}
	return projectResourceByID(projectID, resourceID)
}

func projectResourceByID(projectID int, resourceID int) (resourceResult *db.Resource, errResult error) {
	var (
		resource *db.Resource
		found    bool
		err      error
	)
	resource, found, err = db.GetResourceByID(resourceID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to load resource")
	}
	if !found || resource.ProjectID != projectID {
		return nil, fiber.NewError(fiber.StatusNotFound, "resource not found")
	}
	return resource, nil
}

func resourceParamError(c *fiber.Ctx, err error) (errResult error) {
	var (
		fiberErr *fiber.Error
		ok       bool
	)
	if fiberErr, ok = err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load resource"})
}

func projectAssetGroupFromParam(c *fiber.Ctx, projectID int) (assetGroupResult *db.AssetGroup, errResult error) {
	var (
		groupID int
		err     error
	)
	groupID, err = strconv.Atoi(c.Params("groupID"))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid asset group id")
	}
	return projectAssetGroupByID(projectID, groupID)
}

func projectAssetGroupByID(projectID int, groupID int) (assetGroupResult *db.AssetGroup, errResult error) {
	var (
		group *db.AssetGroup
		err   error
	)
	group, err = db.AssetGroups.Select(groupID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to load asset group")
	}
	if group == nil || group.ProjectID != projectID || group.ArchivedAt != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "asset group not found")
	}
	return group, nil
}

func assetGroupParamError(c *fiber.Ctx, err error) (errResult error) {
	var (
		fiberErr *fiber.Error
		ok       bool
	)
	if fiberErr, ok = err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load asset group"})
}

func requestError(c *fiber.Ctx, err error) (errResult error) {
	var (
		fiberErr *fiber.Error
		ok       bool
	)
	if fiberErr, ok = err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "request failed"})
}

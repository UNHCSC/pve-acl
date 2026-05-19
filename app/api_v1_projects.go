package app

import (
	"strconv"
	"strings"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

type (
	projectCreateRequest struct {
		Name           string `json:"name"`
		Slug           string `json:"slug"`
		Description    string `json:"description"`
		OrganizationID int    `json:"organizationID"`
	}

	projectUpdateRequest struct {
		OrganizationID int `json:"organizationID"`
	}

	organizationCreateRequest struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		ParentOrgID *int   `json:"parentOrgID"`
	}

	organizationUpdateRequest struct {
		ParentOrgID *int `json:"parentOrgID"`
	}
)

// getProjects lists projects the current user can view.
func getProjects(c *fiber.Ctx) (errResult error) {
	var (
		projects []*db.Project
		err      error
	)
	projects, err = db.ListProjects()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load projects",
		})
	}
	var visible []*db.Project

	visible = make([]*db.Project, 0, len(projects))
	for _, project := range projects {
		var (
			allowed  bool
			allowErr error
		)
		allowed, allowErr = currentUserCanViewProject(c, project)
		if allowErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "permission check failed",
			})
		}
		if allowed {
			visible = append(visible, project)
		}
	}

	return c.JSON(visible)
}

// getProjectTree returns visible organizations and projects for navigation.
func getProjectTree(c *fiber.Ctx) (errResult error) {
	var (
		orgs []*db.Organization
		err  error
	)
	orgs, err = db.ListOrganizations()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load organizations",
		})
	}
	var projects []*db.Project

	projects, err = db.ListProjects()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load projects",
		})
	}
	var visibleProjects []fiber.Map

	visibleProjects = make([]fiber.Map, 0, len(projects))
	for _, project := range projects {
		var (
			allowed  bool
			allowErr error
		)
		allowed, allowErr = currentUserCanViewProject(c, project)
		if allowErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "permission check failed",
			})
		}
		if !allowed {
			continue
		}
		var item fiber.Map

		item = fiber.Map{
			"id":              project.ID,
			"uuid":            project.UUID,
			"organization_id": project.OrganizationID,
			"name":            project.Name,
			"slug":            project.Slug,
			"project_type":    project.ProjectType,
			"description":     project.Description,
			"is_active":       project.IsActive,
			"created_at":      project.CreatedAt,
			"updated_at":      project.UpdatedAt,
		}
		var (
			org    *db.Organization
			found  bool
			orgErr error
		)
		if org, found, orgErr = db.GetOrganizationByID(project.OrganizationID); orgErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to load project organization",
			})
		} else if found {
			item["organization"] = organizationResponse(org)
		}
		visibleProjects = append(visibleProjects, item)
	}
	var visibleOrgIDs map[int]bool

	visibleOrgIDs, err = visibleOrganizationIDs(c, orgs, visibleProjects)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "permission check failed",
		})
	}
	var orgItems []fiber.Map

	orgItems = make([]fiber.Map, 0, len(visibleOrgIDs))
	for _, org := range orgs {
		if !visibleOrgIDs[org.ID] {
			continue
		}
		orgItems = append(orgItems, organizationResponse(org))
	}

	return c.JSON(fiber.Map{
		"organizations": orgItems,
		"projects":      visibleProjects,
	})
}

// visibleOrganizationIDs collects organizations visible through project access or org management rights.
func visibleOrganizationIDs(c *fiber.Ctx, orgs []*db.Organization, visibleProjects []fiber.Map) (mapResult map[int]bool, errResult error) {
	var visible map[int]bool
	visible = map[int]bool{}
	if currentUserIsSiteAdmin(c) {
		for _, org := range orgs {
			visible[org.ID] = true
		}
		return visible, nil
	}

	for _, project := range visibleProjects {
		var orgID int
		orgID, _ = project["organization_id"].(int)
		if orgID == 0 {
			continue
		}
		var err error
		if err = addOrganizationAncestors(visible, orgID); err != nil {
			return nil, err
		}
	}

	for _, org := range orgs {
		var (
			allowed bool
			err     error
		)
		allowed, err = currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeOrg, &org.ID)
		if err != nil {
			return nil, err
		}
		if allowed {
			if err = addOrganizationAncestors(visible, org.ID); err != nil {
				return nil, err
			}
		}
	}

	return visible, nil
}

// addOrganizationAncestors marks an organization and all of its ancestors as visible.
func addOrganizationAncestors(visible map[int]bool, orgID int) (errResult error) {
	var (
		ancestors []int
		err       error
	)
	ancestors, err = db.OrganizationAncestorIDs(orgID)
	if err != nil {
		return err
	}
	for _, ancestorID := range ancestors {
		visible[ancestorID] = true
	}
	return nil
}

// postCreateProject creates a custom project in an accessible organization.
func postCreateProject(c *fiber.Ctx) (errResult error) {
	var req projectCreateRequest
	var err error
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid project request",
		})
	}
	var organizationID int

	organizationID, err = resolveProjectOrganizationID(req.OrganizationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	var allowed bool

	allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeGlobal, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "permission check failed",
		})
	}
	if !allowed {
		allowed, err = currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeOrg, &organizationID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "permission check failed",
			})
		}
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "permission denied",
		})
	}
	var project *db.Project

	project, err = db.CreateProject(db.ProjectCreateInput{
		Name:           strings.TrimSpace(req.Name),
		Slug:           strings.TrimSpace(req.Slug),
		Description:    strings.TrimSpace(req.Description),
		OrganizationID: organizationID,
		ProjectType:    db.ProjectTypeCustom,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	var dbUser *db.User

	dbUser = currentDBUser(c)
	if dbUser != nil {
		if _, err = db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, dbUser.ID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to assign project owner",
			})
		}
		if err = db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, dbUser.ID, db.ProjectRoleOwner); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to assign project owner",
			})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(project)
}

// postCreateOrganization creates an organization under an allowed parent.
func postCreateOrganization(c *fiber.Ctx) (errResult error) {
	var req organizationCreateRequest
	var err error
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid organization request"})
	}

	if req.ParentOrgID == nil {
		var (
			rootExists bool
			rootErr    error
		)
		rootExists, rootErr = db.ActiveRootOrganizationExists()
		if rootErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to validate root organization"})
		}
		if rootExists {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "parent organization is required"})
		}
	}
	if req.ParentOrgID != nil {
		var (
			parent *db.Organization
			found  bool
		)
		if parent, found, err = db.GetOrganizationByID(*req.ParentOrgID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load parent organization"})
		} else if !found || parent.ArchivedAt != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "parent organization was not found"})
		}
	}
	var allowed bool

	allowed, err = currentUserCanCreateOrganization(c, req.ParentOrgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var org *db.Organization

	org, err = db.CreateOrganization(db.OrganizationCreateInput{
		Name:        strings.TrimSpace(req.Name),
		Slug:        strings.TrimSpace(req.Slug),
		Description: strings.TrimSpace(req.Description),
		ParentOrgID: req.ParentOrgID,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(organizationResponse(org))
}

// patchOrganization updates organization parent placement.
func patchOrganization(c *fiber.Ctx) (errResult error) {
	var (
		org *db.Organization
		err error
	)
	org, err = organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	if org.ArchivedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "archived organizations cannot be updated"})
	}

	var req organizationUpdateRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid organization request"})
	}
	var allowed bool

	allowed, err = currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	if req.ParentOrgID == nil && org.Slug != db.DefaultRootOrganizationSlug {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization must remain under the root organization"})
	}
	if req.ParentOrgID != nil && org.Slug == db.DefaultRootOrganizationSlug {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "root organization must remain at the root"})
	}
	if req.ParentOrgID != nil {
		if *req.ParentOrgID == org.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization cannot be its own parent"})
		}
		var (
			parent  *db.Organization
			found   bool
			loadErr error
		)
		if parent, found, loadErr = db.GetOrganizationByID(*req.ParentOrgID); loadErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load parent organization"})
		} else if !found || parent.ArchivedAt != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "parent organization was not found"})
		}
		var (
			ancestors   []int
			ancestorErr error
		)
		ancestors, ancestorErr = db.OrganizationAncestorIDs(*req.ParentOrgID)
		if ancestorErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to validate organization tree"})
		}
		for _, ancestorID := range ancestors {
			if ancestorID == org.ID {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization cannot move below one of its descendants"})
			}
		}
		allowed, err = currentUserCanManageOrganization(c, *req.ParentOrgID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
		}
		if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied for target organization"})
		}
	}

	org.ParentOrgID = req.ParentOrgID
	if err = db.UpdateOrganization(org); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(organizationResponse(org))
}

// deleteOrganization archives an empty non-root organization.
func deleteOrganization(c *fiber.Ctx) (errResult error) {
	var (
		org *db.Organization
		err error
	)
	org, err = organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	if org.Slug == db.DefaultRootOrganizationSlug {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "default organization cannot be deleted"})
	}
	var allowed bool

	allowed, err = currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	var orgs []*db.Organization

	orgs, err = db.ListOrganizations()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load organizations"})
	}
	for _, child := range orgs {
		if child.ParentOrgID != nil && *child.ParentOrgID == org.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization has child organizations"})
		}
	}
	var projects []*db.Project
	projects, err = db.ListProjects()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load projects"})
	}
	for _, project := range projects {
		if project.OrganizationID == org.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization has projects"})
		}
	}

	if err = db.ArchiveOrganization(org); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to archive organization"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// patchProject moves a project to a new organization.
func patchProject(c *fiber.Ctx) (errResult error) {
	var (
		projectID int
		err       error
	)
	projectID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	var (
		project *db.Project
		found   bool
	)
	project, found, err = db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}
	var allowed bool

	allowed, err = currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var req projectUpdateRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project request"})
	}
	if req.OrganizationID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization is required"})
	}
	var org *db.Organization
	if org, found, err = db.GetOrganizationByID(req.OrganizationID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load organization"})
	} else if !found || org.ArchivedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization was not found"})
	}
	allowed, err = currentUserCanManageOrganization(c, req.OrganizationID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied for target organization"})
	}

	project.OrganizationID = req.OrganizationID
	if err = db.UpdateProject(project); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(project)
}

// organizationResponse serializes an organization for API responses.
func organizationResponse(org *db.Organization) (mapResult fiber.Map) {
	return fiber.Map{
		"id":            org.ID,
		"uuid":          org.UUID,
		"name":          org.Name,
		"slug":          org.Slug,
		"description":   org.Description,
		"parent_org_id": org.ParentOrgID,
		"archived_at":   org.ArchivedAt,
		"created_at":    org.CreatedAt,
		"updated_at":    org.UpdatedAt,
	}
}

// resolveProjectOrganizationID finds a valid target organization or the default root.
func resolveProjectOrganizationID(id int) (countResult int, errResult error) {
	if id > 0 {
		var (
			org   *db.Organization
			found bool
			err   error
		)
		if org, found, err = db.GetOrganizationByID(id); err != nil {
			return id, err
		} else if found && org.ArchivedAt == nil {
			return id, nil
		}
		return 0, fiber.NewError(fiber.StatusBadRequest, "organization was not found")
	}
	var (
		org   *db.Organization
		found bool
		err   error
	)

	org, found, err = db.GetOrganizationBySlug(db.DefaultRootOrganizationSlug)
	if err != nil {
		return 0, err
	}
	if !found || org.ArchivedAt != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "default organization was not found")
	}
	return org.ID, nil
}

// organizationFromParam loads an organization from the route id parameter.
func organizationFromParam(c *fiber.Ctx) (organizationResult *db.Organization, errResult error) {
	var (
		orgID int
		err   error
	)
	orgID, err = strconv.Atoi(c.Params("id"))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid organization id")
	}
	var (
		org   *db.Organization
		found bool
	)
	org, found, err = db.GetOrganizationByID(orgID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to load organization")
	}
	if !found {
		return nil, fiber.NewError(fiber.StatusNotFound, "organization not found")
	}
	return org, nil
}

// organizationParamError writes a JSON response for organization lookup errors.
func organizationParamError(c *fiber.Ctx, err error) (errResult error) {
	var (
		fiberErr *fiber.Error
		ok       bool
	)
	if fiberErr, ok = err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load organization"})
}

// currentUserCanCreateOrganization reports whether the current user can create under a parent.
func currentUserCanCreateOrganization(c *fiber.Ctx, parentOrgID *int) (okResult bool, errResult error) {
	if parentOrgID == nil {
		return currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeGlobal, nil)
	}
	return currentUserCanManageOrganization(c, *parentOrgID)
}

// currentUserCanManageOrganization reports whether the current user can manage an organization.
func currentUserCanManageOrganization(c *fiber.Ctx, orgID int) (okResult bool, errResult error) {
	var (
		allowed bool
		err     error
	)
	allowed, err = currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || allowed {
		return allowed, err
	}
	return currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeOrg, &orgID)
}

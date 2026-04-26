package app

import (
	"strconv"
	"strings"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

type projectCreateRequest struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description"`
	OrganizationID int    `json:"organizationID"`
}

type projectUpdateRequest struct {
	OrganizationID int `json:"organizationID"`
}

type organizationCreateRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ParentOrgID *int   `json:"parentOrgID"`
}

type organizationUpdateRequest struct {
	ParentOrgID *int `json:"parentOrgID"`
}

func getProjects(c *fiber.Ctx) error {
	projects, err := db.ListProjects()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load projects",
		})
	}

	visible := make([]*db.Project, 0, len(projects))
	for _, project := range projects {
		allowed, allowErr := currentUserCanViewProject(c, project)
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

func getProjectTree(c *fiber.Ctx) error {
	orgs, err := db.ListOrganizations()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load organizations",
		})
	}

	projects, err := db.ListProjects()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load projects",
		})
	}

	visibleProjects := make([]fiber.Map, 0, len(projects))
	for _, project := range projects {
		allowed, allowErr := currentUserCanViewProject(c, project)
		if allowErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "permission check failed",
			})
		}
		if !allowed {
			continue
		}

		item := fiber.Map{
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
		if org, found, orgErr := db.GetOrganizationByID(project.OrganizationID); orgErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to load project organization",
			})
		} else if found {
			item["organization"] = organizationResponse(org)
		}
		visibleProjects = append(visibleProjects, item)
	}

	visibleOrgIDs, err := visibleOrganizationIDs(c, orgs, visibleProjects)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "permission check failed",
		})
	}

	orgItems := make([]fiber.Map, 0, len(visibleOrgIDs))
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

func visibleOrganizationIDs(c *fiber.Ctx, orgs []*db.Organization, visibleProjects []fiber.Map) (map[int]bool, error) {
	visible := map[int]bool{}
	if currentUserIsSiteAdmin(c) {
		for _, org := range orgs {
			visible[org.ID] = true
		}
		return visible, nil
	}

	for _, project := range visibleProjects {
		orgID, _ := project["organization_id"].(int)
		if orgID == 0 {
			continue
		}
		if err := addOrganizationAncestors(visible, orgID); err != nil {
			return nil, err
		}
	}

	for _, org := range orgs {
		allowed, err := currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeOrg, &org.ID)
		if err != nil {
			return nil, err
		}
		if allowed {
			if err := addOrganizationAncestors(visible, org.ID); err != nil {
				return nil, err
			}
		}
	}

	return visible, nil
}

func addOrganizationAncestors(visible map[int]bool, orgID int) error {
	ancestors, err := db.OrganizationAncestorIDs(orgID)
	if err != nil {
		return err
	}
	for _, ancestorID := range ancestors {
		visible[ancestorID] = true
	}
	return nil
}

func postCreateProject(c *fiber.Ctx) error {
	var req projectCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid project request",
		})
	}

	organizationID, err := resolveProjectOrganizationID(req.OrganizationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	allowed, err := currentUserCan(c, db.PermissionProjectManage, db.RoleBindingScopeGlobal, nil)
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

	project, err := db.CreateProject(db.ProjectCreateInput{
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

	dbUser := currentDBUser(c)
	if dbUser != nil {
		if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, dbUser.ID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to assign project owner",
			})
		}
		if err := db.EnsureProjectMemberAccessRole(project.ID, db.ProjectMemberSubjectUser, dbUser.ID, db.ProjectRoleOwner); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to assign project owner",
			})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(project)
}

func postCreateOrganization(c *fiber.Ctx) error {
	var req organizationCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid organization request"})
	}

	if req.ParentOrgID != nil {
		if _, found, err := db.GetOrganizationByID(*req.ParentOrgID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load parent organization"})
		} else if !found {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "parent organization was not found"})
		}
	}

	allowed, err := currentUserCanCreateOrganization(c, req.ParentOrgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	org, err := db.CreateOrganization(db.OrganizationCreateInput{
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

func patchOrganization(c *fiber.Ctx) error {
	org, err := organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}

	var req organizationUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid organization request"})
	}

	allowed, err := currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}
	if req.ParentOrgID != nil {
		if *req.ParentOrgID == org.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization cannot be its own parent"})
		}
		if _, found, loadErr := db.GetOrganizationByID(*req.ParentOrgID); loadErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load parent organization"})
		} else if !found {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "parent organization was not found"})
		}
		ancestors, ancestorErr := db.OrganizationAncestorIDs(*req.ParentOrgID)
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
	if err := db.UpdateOrganization(org); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(organizationResponse(org))
}

func deleteOrganization(c *fiber.Ctx) error {
	org, err := organizationFromParam(c)
	if err != nil {
		return organizationParamError(c, err)
	}
	if org.Slug == db.DefaultRootOrganizationSlug {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "default organization cannot be deleted"})
	}

	allowed, err := currentUserCanManageOrganization(c, org.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	orgs, err := db.ListOrganizations()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load organizations"})
	}
	for _, child := range orgs {
		if child.ParentOrgID != nil && *child.ParentOrgID == org.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization has child organizations"})
		}
	}
	projects, err := db.ListProjects()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load projects"})
	}
	for _, project := range projects {
		if project.OrganizationID == org.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization has projects"})
		}
	}

	if err := db.DeleteOrganization(org.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete organization"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func patchProject(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project id"})
	}
	project, found, err := db.GetProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load project"})
	}
	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	}

	allowed, err := currentUserCanManageProject(c, project)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "permission check failed"})
	}
	if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
	}

	var req projectUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid project request"})
	}
	if req.OrganizationID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "organization is required"})
	}
	if _, found, err := db.GetOrganizationByID(req.OrganizationID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load organization"})
	} else if !found {
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
	if err := db.UpdateProject(project); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(project)
}

func organizationResponse(org *db.Organization) fiber.Map {
	return fiber.Map{
		"id":            org.ID,
		"uuid":          org.UUID,
		"name":          org.Name,
		"slug":          org.Slug,
		"description":   org.Description,
		"parent_org_id": org.ParentOrgID,
		"created_at":    org.CreatedAt,
		"updated_at":    org.UpdatedAt,
	}
}

func resolveProjectOrganizationID(id int) (int, error) {
	if id > 0 {
		if _, found, err := db.GetOrganizationByID(id); err != nil || found {
			return id, err
		}
		return 0, fiber.NewError(fiber.StatusBadRequest, "organization was not found")
	}

	org, found, err := db.GetOrganizationBySlug(db.DefaultRootOrganizationSlug)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fiber.NewError(fiber.StatusBadRequest, "default organization was not found")
	}
	return org.ID, nil
}

func organizationFromParam(c *fiber.Ctx) (*db.Organization, error) {
	orgID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid organization id")
	}
	org, found, err := db.GetOrganizationByID(orgID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to load organization")
	}
	if !found {
		return nil, fiber.NewError(fiber.StatusNotFound, "organization not found")
	}
	return org, nil
}

func organizationParamError(c *fiber.Ctx, err error) error {
	if fiberErr, ok := err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load organization"})
}

func currentUserCanCreateOrganization(c *fiber.Ctx, parentOrgID *int) (bool, error) {
	if parentOrgID == nil {
		return currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeGlobal, nil)
	}
	return currentUserCanManageOrganization(c, *parentOrgID)
}

func currentUserCanManageOrganization(c *fiber.Ctx, orgID int) (bool, error) {
	allowed, err := currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeGlobal, nil)
	if err != nil || allowed {
		return allowed, err
	}
	return currentUserCan(c, db.PermissionOrgManage, db.RoleBindingScopeOrg, &orgID)
}

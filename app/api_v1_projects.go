package app

import (
	"strings"

	"github.com/UNHCSC/proxman/db"
	"github.com/gofiber/fiber/v2"
)

type projectCreateRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func getProjects(c *fiber.Ctx) error {
	projects, err := db.ListProjects()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to load projects",
		})
	}

	return c.JSON(projects)
}

func postCreateProject(c *fiber.Ctx) error {
	allowed, err := requirePermission(c, "project.manage", db.RoleBindingScopeGlobal, nil)
	if err != nil || !allowed {
		return err
	}

	var req projectCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid project request",
		})
	}

	project, err := db.CreateProject(db.ProjectCreateInput{
		Name:        strings.TrimSpace(req.Name),
		Slug:        strings.TrimSpace(req.Slug),
		Description: strings.TrimSpace(req.Description),
		ProjectType: db.ProjectTypeCustom,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	dbUser := currentDBUser(c)
	if dbUser != nil {
		if _, err := db.EnsureProjectMembership(project.ID, db.ProjectMemberSubjectUser, dbUser.ID, db.ProjectRoleOwner); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to assign project owner",
			})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(project)
}

package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/z46-dev/gomysql"
)

type ProjectCreateInput struct {
	Name           string
	Slug           string
	Description    string
	OrganizationID int
	ProjectType    ProjectType
}

func CreateProject(input ProjectCreateInput) (*Project, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = slugify(input.Slug)
	if input.Slug == "" {
		input.Slug = slugify(input.Name)
	}

	if input.Name == "" {
		return nil, fmt.Errorf("project name is required")
	}
	if input.Slug == "" {
		return nil, fmt.Errorf("project slug is required")
	}

	if input.OrganizationID == 0 {
		org, found, err := findOrganizationBySlug(DefaultRootOrganizationSlug)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("default organization %q was not found", DefaultRootOrganizationSlug)
		}
		input.OrganizationID = org.ID
	}

	if existing, found, err := findProjectBySlug(input.Slug); err != nil || found {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("project slug %q already exists", existing.Slug)
	}

	uuid, err := randomUUID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	project := &Project{
		UUID:           uuid,
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Slug:           input.Slug,
		ProjectType:    input.ProjectType,
		Description:    strings.TrimSpace(input.Description),
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := Projects.Insert(project); err != nil {
		return nil, err
	}

	return project, nil
}

func ListProjects() ([]*Project, error) {
	return Projects.SelectAll()
}

func GetProjectBySlug(slug string) (*Project, bool, error) {
	return findProjectBySlug(slug)
}

func GetProjectByID(id int) (*Project, bool, error) {
	project, err := Projects.Select(id)
	if err != nil {
		return nil, false, err
	}
	if project == nil {
		return nil, false, nil
	}
	return project, true, nil
}

func ProjectMembershipsForProject(projectID int) ([]*ProjectMembership, error) {
	return ProjectMemberships.SelectAllWithFilter(
		gomysql.NewFilter().KeyCmp(ProjectMemberships.FieldBySQLName("project_id"), gomysql.OpEqual, projectID),
	)
}

func EnsureProjectMembership(projectID int, subjectType ProjectMemberSubject, subjectID int, role ProjectRole) (bool, error) {
	filter := gomysql.NewFilter().
		KeyCmp(ProjectMemberships.FieldBySQLName("project_id"), gomysql.OpEqual, projectID).
		And().
		KeyCmp(ProjectMemberships.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(ProjectMemberships.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID)

	existing, err := ProjectMemberships.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		membership := existing[0]
		if membership.ProjectRole != role {
			membership.ProjectRole = role
			return false, ProjectMemberships.Update(membership)
		}
		return false, nil
	}

	return true, ProjectMemberships.Insert(&ProjectMembership{
		ProjectID:   projectID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		ProjectRole: role,
		CreatedAt:   time.Now().UTC(),
	})
}

func RemoveProjectMembership(membershipID int) error {
	return ProjectMemberships.Delete(membershipID)
}

func UpdateProjectMembershipRole(membershipID int, role ProjectRole) (*ProjectMembership, error) {
	membership, err := ProjectMemberships.Select(membershipID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, nil
	}
	membership.ProjectRole = role
	if err := ProjectMemberships.Update(membership); err != nil {
		return nil, err
	}
	return membership, nil
}

func UpdateProject(project *Project) error {
	project.Name = strings.TrimSpace(project.Name)
	project.Slug = slugify(project.Slug)
	if project.Name == "" {
		return fmt.Errorf("project name is required")
	}
	if project.Slug == "" {
		project.Slug = slugify(project.Name)
	}
	if project.Slug == "" {
		return fmt.Errorf("project slug is required")
	}
	project.Description = strings.TrimSpace(project.Description)
	project.UpdatedAt = time.Now().UTC()
	return Projects.Update(project)
}

func DeleteProject(projectID int) error {
	if _, err := ProjectMemberships.DeleteWithFilter(
		gomysql.NewFilter().KeyCmp(ProjectMemberships.FieldBySQLName("project_id"), gomysql.OpEqual, projectID),
	); err != nil {
		return err
	}

	return Projects.Delete(projectID)
}

func findProjectBySlug(slug string) (*Project, bool, error) {
	return findOneByStringField(Projects, Projects.FieldBySQLName("slug"), slug)
}

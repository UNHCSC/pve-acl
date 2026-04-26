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

func EnsureProjectMembership(projectID int, subjectType ProjectMemberSubject, subjectID int) (bool, error) {
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
		return false, nil
	}

	return true, ProjectMemberships.Insert(&ProjectMembership{
		ProjectID:   projectID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		CreatedAt:   time.Now().UTC(),
	})
}

func RemoveProjectMembership(membershipID int) error {
	return ProjectMemberships.Delete(membershipID)
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

func ProjectRoleName(role ProjectRole) string {
	switch role {
	case ProjectRoleOperator:
		return DefaultProjectOperatorRoleName
	case ProjectRoleDeveloper:
		return DefaultProjectDeveloperRoleName
	case ProjectRoleManager:
		return DefaultProjectManagerRoleName
	case ProjectRoleOwner:
		return DefaultProjectOwnerRoleName
	default:
		return DefaultProjectViewerRoleName
	}
}

func ProjectRoleForRoleName(name string) (ProjectRole, bool) {
	switch name {
	case DefaultProjectViewerRoleName:
		return ProjectRoleViewer, true
	case DefaultProjectOperatorRoleName:
		return ProjectRoleOperator, true
	case DefaultProjectDeveloperRoleName:
		return ProjectRoleDeveloper, true
	case DefaultProjectManagerRoleName:
		return ProjectRoleManager, true
	case DefaultProjectOwnerRoleName:
		return ProjectRoleOwner, true
	default:
		return ProjectRoleViewer, false
	}
}

func EnsureProjectMemberAccessRole(projectID int, subjectType ProjectMemberSubject, subjectID int, projectRole ProjectRole) error {
	if err := RemoveProjectMemberAccessRoles(projectID, subjectType, subjectID); err != nil {
		return err
	}

	role, found, err := GetRoleByName(ProjectRoleName(projectRole))
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("project access role %q was not found", ProjectRoleName(projectRole))
	}

	scopeID := projectID
	_, err = ensureRoleBinding(role.ID, RoleBindingSubject(subjectType), subjectID, RoleBindingScopeProject, &scopeID, time.Now().UTC())
	return err
}

func RemoveProjectMemberAccessRoles(projectID int, subjectType ProjectMemberSubject, subjectID int) error {
	roles, err := Roles.SelectAll()
	if err != nil {
		return err
	}
	roleIDs := map[int]bool{}
	for _, role := range roles {
		if _, ok := ProjectRoleForRoleName(role.Name); ok {
			roleIDs[role.ID] = true
		}
	}
	if len(roleIDs) == 0 {
		return nil
	}

	bindings, err := roleBindingsForSubject(RoleBindingSubject(subjectType), subjectID)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.ScopeType != RoleBindingScopeProject || binding.ScopeID == nil || *binding.ScopeID != projectID || !roleIDs[binding.RoleID] {
			continue
		}
		if err := RoleBindings.Delete(binding.ID); err != nil {
			return err
		}
	}
	return nil
}

func ProjectMemberAccessRole(projectID int, subjectType ProjectMemberSubject, subjectID int) (ProjectRole, bool, error) {
	bindings, err := roleBindingsForSubject(RoleBindingSubject(subjectType), subjectID)
	if err != nil {
		return ProjectRoleViewer, false, err
	}

	best := ProjectRoleViewer
	found := false
	for _, binding := range bindings {
		if binding.ScopeType != RoleBindingScopeProject || binding.ScopeID == nil || *binding.ScopeID != projectID {
			continue
		}
		role, err := Roles.Select(binding.RoleID)
		if err != nil {
			return ProjectRoleViewer, false, err
		}
		if role == nil {
			continue
		}
		projectRole, ok := ProjectRoleForRoleName(role.Name)
		if !ok {
			continue
		}
		if !found || projectRole > best {
			best = projectRole
			found = true
		}
	}
	return best, found, nil
}

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

// CreateProject creates a project from input.
func CreateProject(input ProjectCreateInput) (projectResult *Project, errResult error) {
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
		var (
			org   *Organization
			found bool
			err   error
		)

		org, found, err = findOrganizationBySlug(DefaultRootOrganizationSlug)
		if err != nil {
			return nil, err
		}
		if !found || org.ArchivedAt != nil {
			return nil, fmt.Errorf("default organization %q was not found", DefaultRootOrganizationSlug)
		}
		input.OrganizationID = org.ID
	}
	{
		var (
			org   *Organization
			found bool
			err   error
		)

		if org, found, err = GetOrganizationByID(input.OrganizationID); err != nil {
			return nil, err
		} else if !found || org.ArchivedAt != nil {
			return nil, fmt.Errorf("organization was not found")
		}
	}
	{
		var (
			existing *Project
			found    bool
			err      error
		)

		if existing, found, err = findProjectBySlug(input.Slug); err != nil || found {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("project slug %q already exists", existing.Slug)
		}
	}
	var (
		uuid string
		err  error
	)

	uuid, err = randomUUID()
	if err != nil {
		return nil, err
	}
	var now time.Time

	now = time.Now().UTC()
	var project *Project

	project = &Project{
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
	{
		var err error

		if err = Projects.Insert(project); err != nil {
			return nil, err
		}
	}

	return project, nil
}

// ListProjects lists all projects.
func ListProjects() (itemsResult []*Project, errResult error) {
	return Projects.SelectAll()
}

// GetProjectBySlug returns project by slug.
func GetProjectBySlug(slug string) (projectResult *Project, okResult bool, errResult error) {
	return findProjectBySlug(slug)
}

// GetProjectByID returns a project by id.
func GetProjectByID(id int) (projectResult *Project, okResult bool, errResult error) {
	var (
		project *Project
		err     error
	)

	project, err = Projects.Select(id)
	if err != nil {
		return nil, false, err
	}
	if project == nil {
		return nil, false, nil
	}
	return project, true, nil
}

// ProjectMembershipsForProject returns all memberships for a project.
func ProjectMembershipsForProject(projectID int) (itemsResult []*ProjectMembership, errResult error) {
	return ProjectMemberships.SelectAllWithFilter(
		gomysql.NewFilter().KeyCmp(ProjectMemberships.FieldBySQLName("project_id"), gomysql.OpEqual, projectID),
	)
}

// EnsureProjectMembership ensures project membership exists.
func EnsureProjectMembership(projectID int, subjectType ProjectMemberSubject, subjectID int) (okResult bool, errResult error) {
	var filter *gomysql.Filter

	filter = gomysql.NewFilter().
		KeyCmp(ProjectMemberships.FieldBySQLName("project_id"), gomysql.OpEqual, projectID).
		And().
		KeyCmp(ProjectMemberships.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(ProjectMemberships.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID)
	var (
		existing []*ProjectMembership
		err      error
	)

	existing, err = ProjectMemberships.SelectAllWithFilter(filter.Limit(1))
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

// RemoveProjectMembership removes project membership.
func RemoveProjectMembership(membershipID int) (errResult error) {
	return ProjectMemberships.Delete(membershipID)
}

// UpdateProject updates a project.
func UpdateProject(project *Project) (errResult error) {
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

// DeleteProject deletes a project and its memberships.
func DeleteProject(projectID int) (errResult error) {
	{
		var err error

		if _, err = ProjectMemberships.DeleteWithFilter(
			gomysql.NewFilter().KeyCmp(ProjectMemberships.FieldBySQLName("project_id"), gomysql.OpEqual, projectID),
		); err != nil {
			return err
		}
	}

	return Projects.Delete(projectID)
}

func findProjectBySlug(slug string) (projectResult *Project, okResult bool, errResult error) {
	return findOneByStringField(Projects, Projects.FieldBySQLName("slug"), slug)
}

// ProjectRoleName returns the system role name for a project role.
func ProjectRoleName(role ProjectRole) (valueResult string) {
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

// ProjectRoleForRoleName resolves a project role from a role name.
func ProjectRoleForRoleName(name string) (projectRoleResult ProjectRole, okResult bool) {
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

// EnsureProjectMemberAccessRole ensures project member access role exists.
func EnsureProjectMemberAccessRole(projectID int, subjectType ProjectMemberSubject, subjectID int, projectRole ProjectRole) (errResult error) {
	var (
		role  *Role
		found bool
		err   error
	)

	role, found, err = GetRoleByName(ProjectRoleName(projectRole))
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("project access role %q was not found", ProjectRoleName(projectRole))
	}
	return EnsureProjectMemberRoleBinding(projectID, subjectType, subjectID, role.ID)
}

// EnsureProjectMemberRoleBinding ensures project member role binding exists.
func EnsureProjectMemberRoleBinding(projectID int, subjectType ProjectMemberSubject, subjectID int, roleID int) (errResult error) {
	{
		var err error

		if err = RemoveProjectMemberAccessRoles(projectID, subjectType, subjectID); err != nil {
			return err
		}
	}
	var scopeID int

	scopeID = projectID
	var err error

	_, err = ensureRoleBinding(roleID, RoleBindingSubject(subjectType), subjectID, RoleBindingScopeProject, &scopeID, time.Now().UTC())
	return err
}

// RemoveProjectMemberAccessRoles removes project member access roles.
func RemoveProjectMemberAccessRoles(projectID int, subjectType ProjectMemberSubject, subjectID int) (errResult error) {
	var (
		bindings []*RoleBinding
		err      error
	)

	bindings, err = roleBindingsForSubject(RoleBindingSubject(subjectType), subjectID)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.ScopeType != RoleBindingScopeProject || binding.ScopeID == nil || *binding.ScopeID != projectID {
			continue
		}
		{
			var err error

			if err = RoleBindings.Delete(binding.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ProjectMemberAccessRole returns the best access role assigned to a project member.
func ProjectMemberAccessRole(projectID int, subjectType ProjectMemberSubject, subjectID int) (projectRoleResult ProjectRole, okResult bool, errResult error) {
	var (
		bindings []*RoleBinding
		err      error
	)

	bindings, err = roleBindingsForSubject(RoleBindingSubject(subjectType), subjectID)
	if err != nil {
		return ProjectRoleViewer, false, err
	}
	var best ProjectRole

	best = ProjectRoleViewer
	var found bool

	found = false
	for _, binding := range bindings {
		if binding.ScopeType != RoleBindingScopeProject || binding.ScopeID == nil || *binding.ScopeID != projectID {
			continue
		}
		var (
			role *Role
			err  error
		)

		role, err = Roles.Select(binding.RoleID)
		if err != nil {
			return ProjectRoleViewer, false, err
		}
		if role == nil {
			continue
		}
		var (
			projectRole ProjectRole
			ok          bool
		)

		projectRole, ok = ProjectRoleForRoleName(role.Name)
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

package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/z46-dev/gomysql"
)

type OrganizationCreateInput struct {
	Name        string
	Slug        string
	Description string
	ParentOrgID *int
}

// CreateOrganization creates an organization from input.
func CreateOrganization(input OrganizationCreateInput) (organizationResult *Organization, errResult error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = slugify(input.Slug)
	if input.Slug == "" {
		input.Slug = slugify(input.Name)
	}

	if input.Name == "" {
		return nil, fmt.Errorf("organization name is required")
	}
	if input.Slug == "" {
		return nil, fmt.Errorf("organization slug is required")
	}
	if input.ParentOrgID == nil {
		var (
			roots []*Organization
			err   error
		)

		roots, err = rootOrganizations()
		if err != nil {
			return nil, err
		}
		if len(roots) > 0 {
			return nil, fmt.Errorf("root organization already exists")
		}
	}
	{
		var (
			existing *Organization
			found    bool
			err      error
		)

		if existing, found, err = findOrganizationBySlug(input.Slug); err != nil || found {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("organization slug %q already exists", existing.Slug)
		}
	}
	if input.ParentOrgID != nil {
		var (
			parent *Organization
			found  bool
			err    error
		)

		parent, found, err = GetOrganizationByID(*input.ParentOrgID)
		if err != nil {
			return nil, err
		}
		if !found || parent.ArchivedAt != nil {
			return nil, fmt.Errorf("parent organization was not found")
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
	var org *Organization

	org = &Organization{
		UUID:        uuid,
		Name:        input.Name,
		Slug:        input.Slug,
		Description: strings.TrimSpace(input.Description),
		ParentOrgID: input.ParentOrgID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	{
		var err error

		if err = Organizations.Insert(org); err != nil {
			return nil, err
		}
	}

	return org, nil
}

// ListOrganizations lists active organizations.
func ListOrganizations() (itemsResult []*Organization, errResult error) {
	return Organizations.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(Organizations.FieldBySQLName("archived_at"), gomysql.OpIsNull, nil))
}

// ActiveRootOrganizationExists reports whether an active root organization exists.
func ActiveRootOrganizationExists() (okResult bool, errResult error) {
	var (
		roots []*Organization
		err   error
	)

	roots, err = rootOrganizations()
	if err != nil {
		return false, err
	}
	return len(roots) > 0, nil
}

func rootOrganizations() (itemsResult []*Organization, errResult error) {
	return Organizations.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(Organizations.FieldBySQLName("parent_org_id"), gomysql.OpIsNull, nil).
		And().
		KeyCmp(Organizations.FieldBySQLName("archived_at"), gomysql.OpIsNull, nil))
}

// GetOrganizationByID returns an organization by id.
func GetOrganizationByID(id int) (organizationResult *Organization, okResult bool, errResult error) {
	var (
		org *Organization
		err error
	)

	org, err = Organizations.Select(id)
	if err != nil {
		return nil, false, err
	}
	if org == nil {
		return nil, false, nil
	}
	return org, true, nil
}

// GetOrganizationBySlug returns organization by slug.
func GetOrganizationBySlug(slug string) (organizationResult *Organization, okResult bool, errResult error) {
	return findOrganizationBySlug(strings.TrimSpace(slug))
}

// UpdateOrganization updates an organization.
func UpdateOrganization(org *Organization) (errResult error) {
	org.Name = strings.TrimSpace(org.Name)
	org.Slug = slugify(org.Slug)
	if org.Name == "" {
		return fmt.Errorf("organization name is required")
	}
	if org.Slug == "" {
		org.Slug = slugify(org.Name)
	}
	if org.Slug == "" {
		return fmt.Errorf("organization slug is required")
	}
	org.Description = strings.TrimSpace(org.Description)
	org.UpdatedAt = time.Now().UTC()
	return Organizations.Update(org)
}

// DeleteOrganization deletes an empty organization.
func DeleteOrganization(orgID int) (errResult error) {
	var (
		org   *Organization
		found bool
		err   error
	)

	org, found, err = GetOrganizationByID(orgID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return ArchiveOrganization(org)
}

// ArchiveOrganization archives an organization.
func ArchiveOrganization(org *Organization) (errResult error) {
	if org.ArchivedAt == nil {
		var now time.Time

		now = time.Now().UTC()
		org.ArchivedAt = &now
	}
	org.UpdatedAt = time.Now().UTC()
	return Organizations.Update(org)
}

// OrganizationMembershipsForOrganization returns memberships for an organization.
func OrganizationMembershipsForOrganization(orgID int) (itemsResult []*OrganizationMembership, errResult error) {
	return OrganizationMemberships.SelectAllWithFilter(
		gomysql.NewFilter().KeyCmp(OrganizationMemberships.FieldBySQLName("organization_id"), gomysql.OpEqual, orgID),
	)
}

// EnsureOrganizationMembership ensures organization membership exists.
func EnsureOrganizationMembership(orgID int, subjectType ProjectMemberSubject, subjectID int, role MembershipRole) (okResult bool, errResult error) {
	var filter *gomysql.Filter

	filter = gomysql.NewFilter().
		KeyCmp(OrganizationMemberships.FieldBySQLName("organization_id"), gomysql.OpEqual, orgID).
		And().
		KeyCmp(OrganizationMemberships.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(OrganizationMemberships.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID)
	var (
		existing []*OrganizationMembership
		err      error
	)

	existing, err = OrganizationMemberships.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		var membership *OrganizationMembership

		membership = existing[0]
		if membership.MembershipRole != role {
			membership.MembershipRole = role
			return false, OrganizationMemberships.Update(membership)
		}
		return false, nil
	}

	return true, OrganizationMemberships.Insert(&OrganizationMembership{
		OrganizationID: orgID,
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		MembershipRole: role,
		CreatedAt:      time.Now().UTC(),
	})
}

// RemoveOrganizationMembership removes organization membership.
func RemoveOrganizationMembership(membershipID int) (errResult error) {
	return OrganizationMemberships.Delete(membershipID)
}

// EnsureOrganizationMemberRoleBinding ensures organization member role binding exists.
func EnsureOrganizationMemberRoleBinding(orgID int, subjectType ProjectMemberSubject, subjectID int, roleID int) (errResult error) {
	{
		var err error

		if err = RemoveOrganizationMemberAccessRoles(orgID, subjectType, subjectID); err != nil {
			return err
		}
	}
	var scopeID int

	scopeID = orgID
	var err error

	_, err = ensureRoleBinding(roleID, RoleBindingSubject(subjectType), subjectID, RoleBindingScopeOrg, &scopeID, time.Now().UTC())
	return err
}

// RemoveOrganizationMemberAccessRoles removes organization member access roles.
func RemoveOrganizationMemberAccessRoles(orgID int, subjectType ProjectMemberSubject, subjectID int) (errResult error) {
	var (
		bindings []*RoleBinding
		err      error
	)

	bindings, err = roleBindingsForSubject(RoleBindingSubject(subjectType), subjectID)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.ScopeType != RoleBindingScopeOrg || binding.ScopeID == nil || *binding.ScopeID != orgID {
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

// SubjectInOrganizationOrAncestor reports whether a subject belongs to an organization or ancestor.
func SubjectInOrganizationOrAncestor(orgID int, subjectType ProjectMemberSubject, subjectID int) (okResult bool, errResult error) {
	var (
		ancestorIDs []int
		err         error
	)

	ancestorIDs, err = OrganizationAncestorIDs(orgID)
	if err != nil {
		return false, err
	}
	for _, ancestorID := range ancestorIDs {
		var (
			memberships []*OrganizationMembership
			err         error
		)

		memberships, err = OrganizationMembershipsForOrganization(ancestorID)
		if err != nil {
			return false, err
		}
		for _, membership := range memberships {
			if membership.SubjectType == subjectType && membership.SubjectID == subjectID {
				return true, nil
			}
		}
	}
	return false, nil
}

// SubjectInProjectOrAncestor reports whether a subject belongs to a project or its organization ancestors.
func SubjectInProjectOrAncestor(projectID int, subjectType ProjectMemberSubject, subjectID int) (okResult bool, errResult error) {
	var (
		project *Project
		found   bool
		err     error
	)

	project, found, err = GetProjectByID(projectID)
	if err != nil || !found {
		return false, err
	}
	var memberships []*ProjectMembership

	memberships, err = ProjectMembershipsForProject(projectID)
	if err != nil {
		return false, err
	}
	for _, membership := range memberships {
		if membership.SubjectType == subjectType && membership.SubjectID == subjectID {
			return true, nil
		}
	}
	return SubjectInOrganizationOrAncestor(project.OrganizationID, subjectType, subjectID)
}

// OrganizationAncestorIDs returns an organization id and its ancestor ids.
func OrganizationAncestorIDs(orgID int) (itemsResult []int, errResult error) {
	var ancestors []int

	ancestors = []int{}
	var seen map[int]bool

	seen = map[int]bool{}
	var currentID int

	currentID = orgID

	for currentID > 0 {
		if seen[currentID] {
			return nil, fmt.Errorf("organization cycle detected at id %d", currentID)
		}
		seen[currentID] = true
		ancestors = append(ancestors, currentID)
		var (
			org *Organization
			err error
		)

		org, err = Organizations.Select(currentID)
		if err != nil {
			return nil, err
		}
		if org == nil || org.ParentOrgID == nil {
			break
		}
		currentID = *org.ParentOrgID
	}

	return ancestors, nil
}

// ProjectOrganizationAncestorIDs returns ancestor organization ids for a project.
func ProjectOrganizationAncestorIDs(projectID int) (itemsResult []int, errResult error) {
	var (
		project *Project
		found   bool
		err     error
	)

	project, found, err = GetProjectByID(projectID)
	if err != nil || !found {
		return nil, err
	}
	return OrganizationAncestorIDs(project.OrganizationID)
}

// ResourceOrganizationAncestorIDs returns ancestor organization ids for a resource.
func ResourceOrganizationAncestorIDs(resourceID int) (itemsResult []int, errResult error) {
	var (
		resource *Resource
		err      error
	)

	resource, err = Resources.Select(resourceID)
	if err != nil || resource == nil {
		return nil, err
	}
	return ProjectOrganizationAncestorIDs(resource.ProjectID)
}

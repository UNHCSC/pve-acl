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

func CreateOrganization(input OrganizationCreateInput) (*Organization, error) {
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
		roots, err := rootOrganizations()
		if err != nil {
			return nil, err
		}
		if len(roots) > 0 {
			return nil, fmt.Errorf("root organization already exists")
		}
	}

	if existing, found, err := findOrganizationBySlug(input.Slug); err != nil || found {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("organization slug %q already exists", existing.Slug)
	}
	if input.ParentOrgID != nil {
		parent, found, err := GetOrganizationByID(*input.ParentOrgID)
		if err != nil {
			return nil, err
		}
		if !found || parent.ArchivedAt != nil {
			return nil, fmt.Errorf("parent organization was not found")
		}
	}

	uuid, err := randomUUID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	org := &Organization{
		UUID:        uuid,
		Name:        input.Name,
		Slug:        input.Slug,
		Description: strings.TrimSpace(input.Description),
		ParentOrgID: input.ParentOrgID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := Organizations.Insert(org); err != nil {
		return nil, err
	}

	return org, nil
}

func ListOrganizations() ([]*Organization, error) {
	return Organizations.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(Organizations.FieldBySQLName("archived_at"), gomysql.OpIsNull, nil))
}

func ActiveRootOrganizationExists() (bool, error) {
	roots, err := rootOrganizations()
	if err != nil {
		return false, err
	}
	return len(roots) > 0, nil
}

func rootOrganizations() ([]*Organization, error) {
	return Organizations.SelectAllWithFilter(gomysql.NewFilter().
		KeyCmp(Organizations.FieldBySQLName("parent_org_id"), gomysql.OpIsNull, nil).
		And().
		KeyCmp(Organizations.FieldBySQLName("archived_at"), gomysql.OpIsNull, nil))
}

func GetOrganizationByID(id int) (*Organization, bool, error) {
	org, err := Organizations.Select(id)
	if err != nil {
		return nil, false, err
	}
	if org == nil {
		return nil, false, nil
	}
	return org, true, nil
}

func GetOrganizationBySlug(slug string) (*Organization, bool, error) {
	return findOrganizationBySlug(strings.TrimSpace(slug))
}

func UpdateOrganization(org *Organization) error {
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

func DeleteOrganization(orgID int) error {
	org, found, err := GetOrganizationByID(orgID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return ArchiveOrganization(org)
}

func ArchiveOrganization(org *Organization) error {
	if org.ArchivedAt == nil {
		now := time.Now().UTC()
		org.ArchivedAt = &now
	}
	org.UpdatedAt = time.Now().UTC()
	return Organizations.Update(org)
}

func OrganizationMembershipsForOrganization(orgID int) ([]*OrganizationMembership, error) {
	return OrganizationMemberships.SelectAllWithFilter(
		gomysql.NewFilter().KeyCmp(OrganizationMemberships.FieldBySQLName("organization_id"), gomysql.OpEqual, orgID),
	)
}

func EnsureOrganizationMembership(orgID int, subjectType ProjectMemberSubject, subjectID int, role MembershipRole) (bool, error) {
	filter := gomysql.NewFilter().
		KeyCmp(OrganizationMemberships.FieldBySQLName("organization_id"), gomysql.OpEqual, orgID).
		And().
		KeyCmp(OrganizationMemberships.FieldBySQLName("subject_type"), gomysql.OpEqual, subjectType).
		And().
		KeyCmp(OrganizationMemberships.FieldBySQLName("subject_id"), gomysql.OpEqual, subjectID)

	existing, err := OrganizationMemberships.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		membership := existing[0]
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

func RemoveOrganizationMembership(membershipID int) error {
	return OrganizationMemberships.Delete(membershipID)
}

func EnsureOrganizationMemberRoleBinding(orgID int, subjectType ProjectMemberSubject, subjectID int, roleID int) error {
	if err := RemoveOrganizationMemberAccessRoles(orgID, subjectType, subjectID); err != nil {
		return err
	}
	scopeID := orgID
	_, err := ensureRoleBinding(roleID, RoleBindingSubject(subjectType), subjectID, RoleBindingScopeOrg, &scopeID, time.Now().UTC())
	return err
}

func RemoveOrganizationMemberAccessRoles(orgID int, subjectType ProjectMemberSubject, subjectID int) error {
	bindings, err := roleBindingsForSubject(RoleBindingSubject(subjectType), subjectID)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.ScopeType != RoleBindingScopeOrg || binding.ScopeID == nil || *binding.ScopeID != orgID {
			continue
		}
		if err := RoleBindings.Delete(binding.ID); err != nil {
			return err
		}
	}
	return nil
}

func SubjectInOrganizationOrAncestor(orgID int, subjectType ProjectMemberSubject, subjectID int) (bool, error) {
	ancestorIDs, err := OrganizationAncestorIDs(orgID)
	if err != nil {
		return false, err
	}
	for _, ancestorID := range ancestorIDs {
		memberships, err := OrganizationMembershipsForOrganization(ancestorID)
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

func SubjectInProjectOrAncestor(projectID int, subjectType ProjectMemberSubject, subjectID int) (bool, error) {
	project, found, err := GetProjectByID(projectID)
	if err != nil || !found {
		return false, err
	}
	memberships, err := ProjectMembershipsForProject(projectID)
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

func OrganizationAncestorIDs(orgID int) ([]int, error) {
	ancestors := []int{}
	seen := map[int]bool{}
	currentID := orgID

	for currentID > 0 {
		if seen[currentID] {
			return nil, fmt.Errorf("organization cycle detected at id %d", currentID)
		}
		seen[currentID] = true
		ancestors = append(ancestors, currentID)

		org, err := Organizations.Select(currentID)
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

func ProjectOrganizationAncestorIDs(projectID int) ([]int, error) {
	project, found, err := GetProjectByID(projectID)
	if err != nil || !found {
		return nil, err
	}
	return OrganizationAncestorIDs(project.OrganizationID)
}

func ResourceOrganizationAncestorIDs(resourceID int) ([]int, error) {
	resource, err := Resources.Select(resourceID)
	if err != nil || resource == nil {
		return nil, err
	}
	return ProjectOrganizationAncestorIDs(resource.ProjectID)
}

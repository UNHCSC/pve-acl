package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/z46-dev/gosqlite"
)

type (
	ResourceCreateInput struct {
		ProjectID       int
		Name            string
		Slug            string
		ResourceType    ResourceType
		Status          ResourceStatus
		CreatedByUserID *int
	}

	ResourceUpdateInput struct {
		Name string
		Slug string
	}
)

// CreateResource creates a local project-owned inventory resource.
func CreateResource(input ResourceCreateInput) (resourceResult *Resource, errResult error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = slugify(input.Slug)
	if input.Slug == "" {
		input.Slug = slugify(input.Name)
	}
	if input.Status == 0 {
		input.Status = ResourceStatusReady
	}
	if input.ProjectID <= 0 {
		return nil, fmt.Errorf("project is required")
	}
	if input.Name == "" {
		return nil, fmt.Errorf("resource name is required")
	}
	if input.Slug == "" {
		return nil, fmt.Errorf("resource slug is required")
	}
	if !LocalInventoryResourceType(input.ResourceType) {
		return nil, fmt.Errorf("unsupported resource type")
	}
	{
		var (
			project *Project
			found   bool
			err     error
		)

		if project, found, err = GetProjectByID(input.ProjectID); err != nil {
			return nil, err
		} else if !found || !project.IsActive {
			return nil, fmt.Errorf("project was not found")
		}
	}
	var (
		existing []*Resource
		err      error
	)

	existing, err = ResourcesForProject(input.ProjectID)
	if err != nil {
		return nil, err
	}
	for _, resource := range existing {
		if strings.EqualFold(resource.Slug, input.Slug) {
			return nil, fmt.Errorf("resource slug %q already exists in this project", resource.Slug)
		}
	}
	var uuid string

	uuid, err = randomUUID()
	if err != nil {
		return nil, err
	}
	var now time.Time

	now = time.Now().UTC()
	var resource *Resource

	resource = &Resource{
		UUID:            uuid,
		ProjectID:       input.ProjectID,
		OwnerType:       OwnerTypeProject,
		OwnerID:         input.ProjectID,
		ResourceType:    input.ResourceType,
		Name:            input.Name,
		Slug:            input.Slug,
		Status:          input.Status,
		CreatedByUserID: input.CreatedByUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err = Resources.Insert(resource); err != nil {
		return nil, err
	}
	return resource, nil
}

// ResourcesForProject returns active local inventory resources for a project.
func ResourcesForProject(projectID int) (itemsResult []*Resource, errResult error) {
	return Resources.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(Resources.FieldBySQLName("project_id"), gosqlite.OpEqual, projectID).
		And().
		KeyCmp(Resources.FieldBySQLName("deleted_at"), gosqlite.OpIsNull, nil))
}

// GetResourceByID returns an active local inventory resource by id.
func GetResourceByID(resourceID int) (resourceResult *Resource, okResult bool, errResult error) {
	var (
		resource *Resource
		err      error
	)

	resource, err = Resources.Select(resourceID)
	if err != nil {
		return nil, false, err
	}
	if resource == nil || resource.DeletedAt != nil {
		return nil, false, nil
	}
	return resource, true, nil
}

// UpdateResource updates editable local inventory resource fields.
func UpdateResource(resource *Resource, input ResourceUpdateInput) (errResult error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = slugify(input.Slug)
	if input.Slug == "" {
		input.Slug = slugify(input.Name)
	}
	if input.Name == "" {
		return fmt.Errorf("resource name is required")
	}
	if input.Slug == "" {
		return fmt.Errorf("resource slug is required")
	}
	var (
		existing []*Resource
		err      error
	)

	existing, err = ResourcesForProject(resource.ProjectID)
	if err != nil {
		return err
	}
	for _, candidate := range existing {
		if candidate.ID != resource.ID && strings.EqualFold(candidate.Slug, input.Slug) {
			return fmt.Errorf("resource slug %q already exists in this project", candidate.Slug)
		}
	}
	resource.Name = input.Name
	resource.Slug = input.Slug
	resource.UpdatedAt = time.Now().UTC()
	return Resources.Update(resource)
}

// ArchiveResource marks a resource deleted.
func ArchiveResource(resource *Resource) (errResult error) {
	if resource.DeletedAt == nil {
		var now time.Time

		now = time.Now().UTC()
		resource.DeletedAt = &now
	}
	resource.Status = ResourceStatusDeleted
	resource.UpdatedAt = time.Now().UTC()
	return Resources.Update(resource)
}

// LocalInventoryResourceType reports whether a resource type is user-manageable here.
func LocalInventoryResourceType(value ResourceType) (okResult bool) {
	return value == ResourceTypeVM || value == ResourceTypeCT || value == ResourceTypeNetwork
}

// ResourceOwnersForResource returns ownership records for a resource.
func ResourceOwnersForResource(resourceID int) (itemsResult []*ResourceOwner, errResult error) {
	return ResourceOwners.SelectAllWithFilter(
		gosqlite.NewFilter().KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gosqlite.OpEqual, resourceID),
	)
}

// EnsureResourceOwner ensures resource owner exists.
func EnsureResourceOwner(resourceID int, subjectType OwnerSubjectType, subjectID int) (okResult bool, errResult error) {
	var filter *gosqlite.Filter

	filter = gosqlite.NewFilter().
		KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gosqlite.OpEqual, resourceID).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_type"), gosqlite.OpEqual, subjectType).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_id"), gosqlite.OpEqual, subjectID)
	var (
		existing []*ResourceOwner
		err      error
	)

	existing, err = ResourceOwners.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, nil
	}

	return true, ResourceOwners.Insert(&ResourceOwner{
		ResourceID:  resourceID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		CreatedAt:   time.Now().UTC(),
	})
}

// RemoveResourceOwner removes resource owner.
func RemoveResourceOwner(resourceID int, subjectType OwnerSubjectType, subjectID int) (errResult error) {
	var err error

	_, err = ResourceOwners.DeleteWithFilter(gosqlite.NewFilter().
		KeyCmp(ResourceOwners.FieldBySQLName("resource_id"), gosqlite.OpEqual, resourceID).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_type"), gosqlite.OpEqual, subjectType).
		And().
		KeyCmp(ResourceOwners.FieldBySQLName("subject_id"), gosqlite.OpEqual, subjectID))
	return err
}

// EnsureResourceUserAccess ensures resource user access exists.
func EnsureResourceUserAccess(resourceID int, subjectType RoleBindingSubject, subjectID int) (errResult error) {
	var (
		role  *Role
		found bool
		err   error
	)

	role, found, err = GetRoleByName(DefaultResourceUserRoleName)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	var scopeID int

	scopeID = resourceID
	_, err = ensureRoleBinding(role.ID, subjectType, subjectID, RoleBindingScopeResource, &scopeID, time.Now().UTC())
	return err
}

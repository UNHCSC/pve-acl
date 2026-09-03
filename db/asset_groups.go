package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/z46-dev/gosqlite"
)

type AssetGroupCreateInput struct {
	ProjectID   int
	Name        string
	Slug        string
	Description string
}

// CreateAssetGroup creates asset group.
func CreateAssetGroup(input AssetGroupCreateInput) (assetGroupResult *AssetGroup, errResult error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = slugify(input.Slug)
	if input.Slug == "" {
		input.Slug = slugify(input.Name)
	}
	if input.ProjectID <= 0 {
		return nil, fmt.Errorf("project is required")
	}
	if input.Name == "" {
		return nil, fmt.Errorf("asset group name is required")
	}
	if input.Slug == "" {
		return nil, fmt.Errorf("asset group slug is required")
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
		existing []*AssetGroup
		err      error
	)

	existing, err = AssetGroupsForProject(input.ProjectID)
	if err != nil {
		return nil, err
	}
	for _, group := range existing {
		if strings.EqualFold(group.Slug, input.Slug) {
			return nil, fmt.Errorf("asset group slug %q already exists in this project", group.Slug)
		}
	}
	var uuid string

	uuid, err = randomUUID()
	if err != nil {
		return nil, err
	}
	var now time.Time

	now = time.Now().UTC()
	var group *AssetGroup

	group = &AssetGroup{
		UUID:        uuid,
		ProjectID:   input.ProjectID,
		Name:        input.Name,
		Slug:        input.Slug,
		Description: strings.TrimSpace(input.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	{
		var err error

		if err = AssetGroups.Insert(group); err != nil {
			return nil, err
		}
	}
	return group, nil
}

// AssetGroupsForProject returns active asset groups for a project.
func AssetGroupsForProject(projectID int) (itemsResult []*AssetGroup, errResult error) {
	return AssetGroups.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(AssetGroups.FieldBySQLName("project_id"), gosqlite.OpEqual, projectID).
		And().
		KeyCmp(AssetGroups.FieldBySQLName("archived_at"), gosqlite.OpIsNull, nil))
}

// AssetGroupResourcesForGroup returns resources attached to an asset group.
func AssetGroupResourcesForGroup(assetGroupID int) (itemsResult []*AssetGroupResource, errResult error) {
	return AssetGroupResources.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(AssetGroupResources.FieldBySQLName("asset_group_id"), gosqlite.OpEqual, assetGroupID))
}

// EnsureAssetGroupResource ensures asset group resource exists.
func EnsureAssetGroupResource(assetGroupID, resourceID int) (okResult bool, errResult error) {
	var (
		assetGroup *AssetGroup
		err        error
	)

	assetGroup, err = AssetGroups.Select(assetGroupID)
	if err != nil || assetGroup == nil {
		return false, err
	}
	var resource *Resource

	resource, err = Resources.Select(resourceID)
	if err != nil || resource == nil || resource.DeletedAt != nil {
		return false, err
	}
	if assetGroup.ProjectID != resource.ProjectID {
		return false, fmt.Errorf("asset group and resource must belong to the same project")
	}
	var filter *gosqlite.Filter

	filter = gosqlite.NewFilter().
		KeyCmp(AssetGroupResources.FieldBySQLName("asset_group_id"), gosqlite.OpEqual, assetGroupID).
		And().
		KeyCmp(AssetGroupResources.FieldBySQLName("resource_id"), gosqlite.OpEqual, resourceID)
	var existing []*AssetGroupResource

	existing, err = AssetGroupResources.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, ensureAssetGroupAssignmentRoleBindingsForResource(assetGroupID, resourceID)
	}
	{
		var err error

		if err = AssetGroupResources.Insert(&AssetGroupResource{
			AssetGroupID: assetGroupID,
			ResourceID:   resourceID,
			CreatedAt:    time.Now().UTC(),
		}); err != nil {
			return false, err
		}
	}
	{
		var err error

		if err = ensureAssetGroupAssignmentRoleBindingsForResource(assetGroupID, resourceID); err != nil {
			return false, err
		}
	}
	return true, nil
}

// RemoveAssetGroupResource removes a resource from an asset group.
func RemoveAssetGroupResource(assetGroupID, resourceID int) (errResult error) {
	var (
		assignments []*AssetAssignment
		err         error
	)

	assignments, err = AssetAssignmentsForAssetGroup(assetGroupID)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		{
			var err error

			if err = RemoveResourceRoleBindingsForSource(resourceID, RoleBindingSourceAssetAssignment, assignment.ID); err != nil {
				return err
			}
		}
	}
	_, err = AssetGroupResources.DeleteWithFilter(gosqlite.NewFilter().
		KeyCmp(AssetGroupResources.FieldBySQLName("asset_group_id"), gosqlite.OpEqual, assetGroupID).
		And().
		KeyCmp(AssetGroupResources.FieldBySQLName("resource_id"), gosqlite.OpEqual, resourceID))
	return err
}

// ArchiveAssetGroup archives an asset group and its active assignments.
func ArchiveAssetGroup(assetGroup *AssetGroup) (errResult error) {
	var (
		assignments []*AssetAssignment
		err         error
	)

	assignments, err = AssetAssignmentsForAssetGroup(assetGroup.ID)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		{
			var err error

			if err = ArchiveAssetAssignment(assignment.ID); err != nil {
				return err
			}
		}
	}
	if assetGroup.ArchivedAt == nil {
		var now time.Time

		now = time.Now().UTC()
		assetGroup.ArchivedAt = &now
	}
	assetGroup.UpdatedAt = time.Now().UTC()
	return AssetGroups.Update(assetGroup)
}

type AssetAssignmentInput struct {
	ProjectID       int
	ResourceID      *int
	AssetGroupID    *int
	SubjectType     RoleBindingSubject
	SubjectID       int
	RoleID          int
	CreatedByUserID *int
}

// EnsureAssetAssignment ensures asset assignment exists.
func EnsureAssetAssignment(input AssetAssignmentInput) (assetAssignmentResult *AssetAssignment, okResult bool, errResult error) {
	if input.ProjectID <= 0 {
		return nil, false, fmt.Errorf("project is required")
	}
	if (input.ResourceID == nil) == (input.AssetGroupID == nil) {
		return nil, false, fmt.Errorf("assign exactly one resource or asset group")
	}
	if input.SubjectID <= 0 {
		return nil, false, fmt.Errorf("assignment subject is required")
	}
	if input.RoleID <= 0 {
		return nil, false, fmt.Errorf("assignment role is required")
	}
	if input.ResourceID != nil {
		var (
			resource *Resource
			err      error
		)

		resource, err = Resources.Select(*input.ResourceID)
		if err != nil || resource == nil || resource.DeletedAt != nil {
			return nil, false, err
		}
		if resource.ProjectID != input.ProjectID {
			return nil, false, fmt.Errorf("resource is not owned by the assignment project")
		}
	}
	if input.AssetGroupID != nil {
		var (
			assetGroup *AssetGroup
			err        error
		)

		assetGroup, err = AssetGroups.Select(*input.AssetGroupID)
		if err != nil || assetGroup == nil || assetGroup.ArchivedAt != nil {
			return nil, false, err
		}
		if assetGroup.ProjectID != input.ProjectID {
			return nil, false, fmt.Errorf("asset group is not owned by the assignment project")
		}
	}
	var filter *gosqlite.Filter

	filter = gosqlite.NewFilter().
		KeyCmp(AssetAssignments.FieldBySQLName("project_id"), gosqlite.OpEqual, input.ProjectID).
		And().
		KeyCmp(AssetAssignments.FieldBySQLName("subject_type"), gosqlite.OpEqual, input.SubjectType).
		And().
		KeyCmp(AssetAssignments.FieldBySQLName("subject_id"), gosqlite.OpEqual, input.SubjectID).
		And().
		KeyCmp(AssetAssignments.FieldBySQLName("role_id"), gosqlite.OpEqual, input.RoleID).
		And().
		KeyCmp(AssetAssignments.FieldBySQLName("archived_at"), gosqlite.OpIsNull, nil).
		And()
	if input.ResourceID != nil {
		filter = filter.KeyCmp(AssetAssignments.FieldBySQLName("resource_id"), gosqlite.OpEqual, *input.ResourceID).
			And().
			KeyCmp(AssetAssignments.FieldBySQLName("asset_group_id"), gosqlite.OpIsNull, nil)
	} else {
		filter = filter.KeyCmp(AssetAssignments.FieldBySQLName("asset_group_id"), gosqlite.OpEqual, *input.AssetGroupID).
			And().
			KeyCmp(AssetAssignments.FieldBySQLName("resource_id"), gosqlite.OpIsNull, nil)
	}
	var (
		existing []*AssetAssignment
		err      error
	)

	existing, err = AssetAssignments.SelectAllWithFilter(filter.Limit(1))
	if err != nil {
		return nil, false, err
	}
	if len(existing) > 0 {
		if input.ResourceID != nil {
			var scopeID int

			scopeID = *input.ResourceID
			var sourceID int

			sourceID = existing[0].ID
			{
				var err error

				if _, err = ensureRoleBindingWithSource(input.RoleID, input.SubjectType, input.SubjectID, RoleBindingScopeResource, &scopeID, RoleBindingSourceAssetAssignment, &sourceID, time.Now().UTC()); err != nil {
					return nil, false, err
				}
			}
		} else if input.AssetGroupID != nil {
			{
				var err error

				if err = ensureAssetGroupAssignmentRoleBindings(*input.AssetGroupID, existing[0]); err != nil {
					return nil, false, err
				}
			}
		}
		return existing[0], false, nil
	}
	var assignment *AssetAssignment

	assignment = &AssetAssignment{
		ProjectID:       input.ProjectID,
		ResourceID:      input.ResourceID,
		AssetGroupID:    input.AssetGroupID,
		SubjectType:     input.SubjectType,
		SubjectID:       input.SubjectID,
		RoleID:          input.RoleID,
		CreatedByUserID: input.CreatedByUserID,
		CreatedAt:       time.Now().UTC(),
	}
	{
		var err error

		if err = AssetAssignments.Insert(assignment); err != nil {
			return nil, false, err
		}
	}
	if input.ResourceID != nil {
		var scopeID int

		scopeID = *input.ResourceID
		var sourceID int

		sourceID = assignment.ID
		{
			var err error

			if _, err = ensureRoleBindingWithSource(input.RoleID, input.SubjectType, input.SubjectID, RoleBindingScopeResource, &scopeID, RoleBindingSourceAssetAssignment, &sourceID, time.Now().UTC()); err != nil {
				return nil, false, err
			}
		}
	} else if input.AssetGroupID != nil {
		{
			var err error

			if err = ensureAssetGroupAssignmentRoleBindings(*input.AssetGroupID, assignment); err != nil {
				return nil, false, err
			}
		}
	}
	return assignment, true, nil
}

// AssetAssignmentsForProject returns active asset assignments for a project.
func AssetAssignmentsForProject(projectID int) (itemsResult []*AssetAssignment, errResult error) {
	return AssetAssignments.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(AssetAssignments.FieldBySQLName("project_id"), gosqlite.OpEqual, projectID).
		And().
		KeyCmp(AssetAssignments.FieldBySQLName("archived_at"), gosqlite.OpIsNull, nil))
}

// AssetAssignmentsForResource returns active direct asset assignments for a resource.
func AssetAssignmentsForResource(resourceID int) (itemsResult []*AssetAssignment, errResult error) {
	return AssetAssignments.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(AssetAssignments.FieldBySQLName("resource_id"), gosqlite.OpEqual, resourceID).
		And().
		KeyCmp(AssetAssignments.FieldBySQLName("archived_at"), gosqlite.OpIsNull, nil))
}

// AssetAssignmentsForAssetGroup returns active asset assignments for an asset group.
func AssetAssignmentsForAssetGroup(assetGroupID int) (itemsResult []*AssetAssignment, errResult error) {
	return AssetAssignments.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(AssetAssignments.FieldBySQLName("asset_group_id"), gosqlite.OpEqual, assetGroupID).
		And().
		KeyCmp(AssetAssignments.FieldBySQLName("archived_at"), gosqlite.OpIsNull, nil))
}

// ArchiveAssetAssignment archives an assignment and removes only its derived role bindings.
func ArchiveAssetAssignment(assignmentID int) (errResult error) {
	var (
		assignment *AssetAssignment
		err        error
	)

	assignment, err = AssetAssignments.Select(assignmentID)
	if err != nil || assignment == nil {
		return err
	}
	if assignment.ArchivedAt == nil {
		var now time.Time

		now = time.Now().UTC()
		assignment.ArchivedAt = &now
		if err = AssetAssignments.Update(assignment); err != nil {
			return err
		}
	}
	return RemoveRoleBindingsForSource(RoleBindingSourceAssetAssignment, assignment.ID)
}

// ArchiveAssetAssignmentsForResource archives direct assignments for a resource.
func ArchiveAssetAssignmentsForResource(resourceID int) (errResult error) {
	var (
		assignments []*AssetAssignment
		err         error
	)

	assignments, err = AssetAssignmentsForResource(resourceID)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		{
			var err error

			if err = ArchiveAssetAssignment(assignment.ID); err != nil {
				return err
			}
		}
	}
	var links []*AssetGroupResource

	links, err = AssetGroupResourcesForResource(resourceID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if err = RemoveAssetGroupResource(link.AssetGroupID, resourceID); err != nil {
			return err
		}
	}
	return nil
}

// AssetGroupResourcesForResource returns asset group links for a resource.
func AssetGroupResourcesForResource(resourceID int) (itemsResult []*AssetGroupResource, errResult error) {
	return AssetGroupResources.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(AssetGroupResources.FieldBySQLName("resource_id"), gosqlite.OpEqual, resourceID))
}

func ensureAssetGroupAssignmentRoleBindings(assetGroupID int, assignment *AssetAssignment) (errResult error) {
	var (
		resources []*AssetGroupResource
		err       error
	)

	resources, err = AssetGroupResourcesForGroup(assetGroupID)
	if err != nil {
		return err
	}
	var now time.Time

	now = time.Now().UTC()
	for _, resource := range resources {
		var scopeID int

		scopeID = resource.ResourceID
		var sourceID int

		sourceID = assignment.ID
		{
			var err error

			if _, err = ensureRoleBindingWithSource(assignment.RoleID, assignment.SubjectType, assignment.SubjectID, RoleBindingScopeResource, &scopeID, RoleBindingSourceAssetAssignment, &sourceID, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureAssetGroupAssignmentRoleBindingsForResource(assetGroupID, resourceID int) (errResult error) {
	var (
		assignments []*AssetAssignment
		err         error
	)

	assignments, err = AssetAssignments.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(AssetAssignments.FieldBySQLName("asset_group_id"), gosqlite.OpEqual, assetGroupID).
		And().
		KeyCmp(AssetAssignments.FieldBySQLName("archived_at"), gosqlite.OpIsNull, nil))
	if err != nil {
		return err
	}
	var now time.Time

	now = time.Now().UTC()
	for _, assignment := range assignments {
		var scopeID int

		scopeID = resourceID
		var sourceID int

		sourceID = assignment.ID
		{
			var err error

			if _, err = ensureRoleBindingWithSource(assignment.RoleID, assignment.SubjectType, assignment.SubjectID, RoleBindingScopeResource, &scopeID, RoleBindingSourceAssetAssignment, &sourceID, now); err != nil {
				return err
			}
		}
	}
	return nil
}

package db

import (
	"fmt"
	"strings"
	"time"
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

	if existing, found, err := findOrganizationBySlug(input.Slug); err != nil || found {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("organization slug %q already exists", existing.Slug)
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
	return Organizations.SelectAll()
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
	return Organizations.Delete(orgID)
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

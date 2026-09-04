package db

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/z46-dev/gosqlite"
)

var allocationMu sync.Mutex

type (
	Blueprint struct {
		ID          int        `gosqlite:"id,primary,increment" json:"id"`
		UUID        string     `gosqlite:"uuid,unique,notnull" json:"uuid"`
		ProjectID   int        `gosqlite:"project_id,fkey:Project.id,notnull" json:"project_id"`
		Name        string     `gosqlite:"name,notnull" json:"name"`
		Slug        string     `gosqlite:"slug,notnull" json:"slug"`
		Description string     `gosqlite:"description" json:"description"`
		CreatedAt   time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt   time.Time  `gosqlite:"updated_at,notnull" json:"updated_at"`
		ArchivedAt  *time.Time `gosqlite:"archived_at" json:"archived_at,omitempty"`
	}

	BlueprintVersion struct {
		ID             int       `gosqlite:"id,primary,increment" json:"id"`
		UUID           string    `gosqlite:"uuid,unique,notnull" json:"uuid"`
		BlueprintID    int       `gosqlite:"blueprint_id,fkey:Blueprint.id,notnull" json:"blueprint_id"`
		Version        int       `gosqlite:"version,notnull" json:"version"`
		DocumentJSON   string    `gosqlite:"document_json,notnull" json:"-"`
		DocumentDigest string    `gosqlite:"document_digest,notnull" json:"document_digest"`
		CreatedByID    *int      `gosqlite:"created_by_id" json:"created_by_id,omitempty"`
		CreatedAt      time.Time `gosqlite:"created_at,notnull" json:"created_at"`
	}

	Deployment struct {
		ID                 int       `gosqlite:"id,primary,increment" json:"id"`
		UUID               string    `gosqlite:"uuid,unique,notnull" json:"uuid"`
		ProjectID          int       `gosqlite:"project_id,fkey:Project.id,notnull" json:"project_id"`
		BlueprintVersionID int       `gosqlite:"blueprint_version_id,fkey:BlueprintVersion.id,notnull" json:"blueprint_version_id"`
		GroupID            int       `gosqlite:"group_id,fkey:CloudGroup.id,notnull" json:"group_id"`
		QuotaReservationID *int      `gosqlite:"quota_reservation_id,fkey:QuotaReservation.id" json:"quota_reservation_id,omitempty"`
		Name               string    `gosqlite:"name,unique,notnull" json:"name"`
		Status             string    `gosqlite:"status,notnull" json:"status"`
		CreatedAt          time.Time `gosqlite:"created_at,notnull" json:"created_at"`
		UpdatedAt          time.Time `gosqlite:"updated_at,notnull" json:"updated_at"`
	}

	DeploymentResource struct {
		ID           int       `gosqlite:"id,primary,increment" json:"id"`
		DeploymentID int       `gosqlite:"deployment_id,fkey:Deployment.id,notnull" json:"deployment_id"`
		ResourceKey  string    `gosqlite:"resource_key,notnull" json:"resource_key"`
		Kind         string    `gosqlite:"kind,notnull" json:"kind"`
		DesiredJSON  string    `gosqlite:"desired_json,notnull" json:"desired_json"`
		CreatedAt    time.Time `gosqlite:"created_at,notnull" json:"created_at"`
	}

	AllocationPool struct {
		ID         int        `gosqlite:"id,primary,increment" json:"id"`
		UUID       string     `gosqlite:"uuid,unique,notnull" json:"uuid"`
		ProjectID  int        `gosqlite:"project_id,fkey:Project.id,notnull" json:"project_id"`
		Name       string     `gosqlite:"name,notnull" json:"name"`
		Kind       string     `gosqlite:"kind,notnull" json:"kind"`
		Start      int        `gosqlite:"range_start,notnull" json:"start"`
		End        int        `gosqlite:"range_end,notnull" json:"end"`
		CIDR       string     `gosqlite:"cidr" json:"cidr,omitempty"`
		CreatedAt  time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		ArchivedAt *time.Time `gosqlite:"archived_at" json:"archived_at,omitempty"`
	}

	Allocation struct {
		ID            int        `gosqlite:"id,primary,increment" json:"id"`
		PoolID        int        `gosqlite:"pool_id,fkey:AllocationPool.id,notnull" json:"pool_id"`
		DeploymentID  int        `gosqlite:"deployment_id,fkey:Deployment.id,notnull" json:"deployment_id"`
		AllocationKey string     `gosqlite:"allocation_key,unique,notnull" json:"allocation_key"`
		Purpose       string     `gosqlite:"purpose,notnull" json:"purpose"`
		Value         string     `gosqlite:"value,notnull" json:"value"`
		CreatedAt     time.Time  `gosqlite:"created_at,notnull" json:"created_at"`
		ReleasedAt    *time.Time `gosqlite:"released_at" json:"released_at,omitempty"`
	}

	BlueprintResourceSpec struct {
		Key               string   `json:"key"`
		Kind              string   `json:"kind"`
		Template          string   `json:"template"`
		VCPU              int      `json:"vcpu"`
		MemoryMB          int      `json:"memory_mb"`
		DiskGB            int      `json:"disk_gb"`
		Networks          []string `json:"networks"`
		ConfigurationRole string   `json:"configuration_role,omitempty"`
	}

	BlueprintNetworkSpec struct {
		Key      string `json:"key"`
		Kind     string `json:"kind"`
		IPv4CIDR string `json:"ipv4_cidr,omitempty"`
		IPv6CIDR string `json:"ipv6_cidr,omitempty"`
		Public   bool   `json:"public,omitempty"`
	}

	BlueprintDocument struct {
		FormatVersion  int                     `json:"format_version"`
		OpenTofuModule string                  `json:"opentofu_module"`
		AnsibleProject string                  `json:"ansible_project"`
		NamePattern    string                  `json:"name_pattern"`
		Resources      []BlueprintResourceSpec `json:"resources"`
		Networks       []BlueprintNetworkSpec  `json:"networks"`
	}
)

// CreateBlueprint creates project-owned blueprint metadata.
func CreateBlueprint(projectID int, name, slug, description string) (blueprintResult *Blueprint, errResult error) {
	name = strings.TrimSpace(name)
	slug = slugify(slug)
	if projectID <= 0 || name == "" || slug == "" {
		return nil, fmt.Errorf("project, name, and slug are required")
	}
	var uuid string
	if uuid, errResult = randomUUID(); errResult != nil {
		return
	}
	var now time.Time = time.Now().UTC()
	blueprintResult = &Blueprint{UUID: uuid, ProjectID: projectID, Name: name, Slug: slug, Description: strings.TrimSpace(description), CreatedAt: now, UpdatedAt: now}
	errResult = Blueprints.Insert(blueprintResult)
	return
}

// BlueprintsForProject lists active project blueprints.
func BlueprintsForProject(projectID int) (results []*Blueprint, errResult error) {
	results, errResult = Blueprints.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(Blueprints.FieldBySQLName("project_id"), gosqlite.OpEqual, projectID))
	if errResult != nil {
		return
	}
	var active []*Blueprint
	for _, blueprint := range results {
		if blueprint.ArchivedAt == nil {
			active = append(active, blueprint)
		}
	}
	return active, nil
}

// PublishBlueprintVersion validates and stores an immutable blueprint document.
func PublishBlueprintVersion(blueprintID int, document BlueprintDocument, createdByID *int) (versionResult *BlueprintVersion, errResult error) {
	if errResult = ValidateBlueprintDocument(document); errResult != nil {
		return
	}
	var versions []*BlueprintVersion
	if versions, errResult = BlueprintVersions.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(BlueprintVersions.FieldBySQLName("blueprint_id"), gosqlite.OpEqual, blueprintID)); errResult != nil {
		return
	}
	var body []byte
	if body, errResult = json.Marshal(document); errResult != nil {
		return
	}
	var digest string
	digest = contentDigest(body)
	for _, version := range versions {
		if version.DocumentDigest == digest {
			return nil, fmt.Errorf("this blueprint document is already published as version %d", version.Version)
		}
	}
	var uuid string
	if uuid, errResult = randomUUID(); errResult != nil {
		return
	}
	versionResult = &BlueprintVersion{UUID: uuid, BlueprintID: blueprintID, Version: len(versions) + 1, DocumentJSON: string(body), DocumentDigest: digest, CreatedByID: createdByID, CreatedAt: time.Now().UTC()}
	errResult = BlueprintVersions.Insert(versionResult)
	return
}

// BlueprintVersionsForBlueprint lists immutable versions in publication order.
func BlueprintVersionsForBlueprint(blueprintID int) (results []*BlueprintVersion, errResult error) {
	return BlueprintVersions.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(BlueprintVersions.FieldBySQLName("blueprint_id"), gosqlite.OpEqual, blueprintID))
}

// BlueprintDocumentForVersion decodes a stored immutable document.
func BlueprintDocumentForVersion(version *BlueprintVersion) (documentResult BlueprintDocument, errResult error) {
	if version == nil {
		return documentResult, fmt.Errorf("blueprint version was not found")
	}
	errResult = json.Unmarshal([]byte(version.DocumentJSON), &documentResult)
	return
}

// ValidateBlueprintDocument checks the generic runner-facing blueprint contract.
func ValidateBlueprintDocument(document BlueprintDocument) (errResult error) {
	if document.FormatVersion != 1 {
		return fmt.Errorf("format_version must be 1")
	}
	if strings.TrimSpace(document.OpenTofuModule) == "" || strings.TrimSpace(document.AnsibleProject) == "" {
		return fmt.Errorf("opentofu_module and ansible_project are required")
	}
	if !immutableRunnerReference(document.OpenTofuModule) || !immutableRunnerReference(document.AnsibleProject) {
		return fmt.Errorf("opentofu_module and ansible_project must contain a pinned ref or digest")
	}
	if !strings.Contains(document.NamePattern, "{{deployment}}") {
		return fmt.Errorf("name_pattern must contain {{deployment}}")
	}
	var networkKeys map[string]bool = make(map[string]bool, len(document.Networks))
	for _, network := range document.Networks {
		if network.Key == "" || networkKeys[network.Key] {
			return fmt.Errorf("network keys must be present and unique")
		}
		networkKeys[network.Key] = true
		if network.IPv4CIDR != "" {
			if _, errResult = netip.ParsePrefix(network.IPv4CIDR); errResult != nil {
				return fmt.Errorf("network %s has invalid IPv4 CIDR", network.Key)
			}
		}
		if network.IPv6CIDR != "" {
			if _, errResult = netip.ParsePrefix(network.IPv6CIDR); errResult != nil {
				return fmt.Errorf("network %s has invalid IPv6 CIDR", network.Key)
			}
		}
	}
	if len(document.Resources) == 0 {
		return fmt.Errorf("at least one resource is required")
	}
	var resourceKeys map[string]bool = make(map[string]bool, len(document.Resources))
	for _, resource := range document.Resources {
		if resource.Key == "" || resourceKeys[resource.Key] {
			return fmt.Errorf("resource keys must be present and unique")
		}
		resourceKeys[resource.Key] = true
		if resource.Kind != "vm" && resource.Kind != "ct" {
			return fmt.Errorf("resource %s has unsupported kind %q", resource.Key, resource.Kind)
		}
		if resource.Template == "" || resource.VCPU <= 0 || resource.MemoryMB <= 0 || resource.DiskGB <= 0 {
			return fmt.Errorf("resource %s requires a template and positive sizing", resource.Key)
		}
		for _, network := range resource.Networks {
			if !networkKeys[network] {
				return fmt.Errorf("resource %s references unknown network %s", resource.Key, network)
			}
		}
	}
	return
}

func contentDigest(body []byte) (valueResult string) {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(body))
}

// CreateAllocationPool creates a project-scoped numeric or address allocation pool.
func CreateAllocationPool(projectID int, name, kind string, start, end int, cidr string) (poolResult *AllocationPool, errResult error) {
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	cidr = strings.TrimSpace(cidr)
	if projectID <= 0 || name == "" || !validAllocationKind(kind) || start < 0 || end < start {
		return nil, fmt.Errorf("project, name, supported kind, and valid range are required")
	}
	if kind == "ipv4" || kind == "ipv6" {
		var prefix netip.Prefix
		if prefix, errResult = netip.ParsePrefix(cidr); errResult != nil || (kind == "ipv4") != prefix.Addr().Is4() {
			return nil, fmt.Errorf("pool CIDR does not match kind %s", kind)
		}
	}
	var uuid string
	if uuid, errResult = randomUUID(); errResult != nil {
		return
	}
	poolResult = &AllocationPool{UUID: uuid, ProjectID: projectID, Name: name, Kind: kind, Start: start, End: end, CIDR: cidr, CreatedAt: time.Now().UTC()}
	errResult = AllocationPools.Insert(poolResult)
	return
}

// AllocationPoolsForProject lists active pools and their available capacity.
func AllocationPoolsForProject(projectID int) (results []*AllocationPool, errResult error) {
	results, errResult = AllocationPools.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(AllocationPools.FieldBySQLName("project_id"), gosqlite.OpEqual, projectID))
	if errResult != nil {
		return
	}
	var active []*AllocationPool
	for _, pool := range results {
		if pool.ArchivedAt == nil {
			active = append(active, pool)
		}
	}
	return active, nil
}

// AllocationPoolAvailable returns unallocated values remaining in a pool.
func AllocationPoolAvailable(pool *AllocationPool) (availableResult int, errResult error) {
	if pool == nil || pool.ArchivedAt != nil {
		return 0, fmt.Errorf("allocation pool was not found")
	}
	allocationMu.Lock()
	defer allocationMu.Unlock()
	var allocations []*Allocation
	if allocations, errResult = Allocations.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(Allocations.FieldBySQLName("pool_id"), gosqlite.OpEqual, pool.ID)); errResult != nil {
		return
	}
	availableResult = pool.End - pool.Start + 1
	for _, allocation := range allocations {
		if allocation.ReleasedAt == nil {
			availableResult--
		}
	}
	if availableResult < 0 {
		availableResult = 0
	}
	return
}

func validAllocationKind(kind string) bool {
	return kind == "vmid" || kind == "vlan" || kind == "vxlan" || kind == "external_port" || kind == "ipv4" || kind == "ipv6"
}

func immutableRunnerReference(value string) bool {
	return strings.Contains(value, "ref=") || strings.Contains(value, "sha256:")
}

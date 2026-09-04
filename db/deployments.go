package db

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/z46-dev/gosqlite"
)

type (
	DeploymentPlanInput struct {
		ProjectID          int
		BlueprintVersionID int
		GroupIDs           []int
		NamePrefix         string
		AllocationPoolIDs  map[string]int
	}

	DeploymentPlanResult struct {
		Deployments []*Deployment         `json:"deployments"`
		Resources   []*DeploymentResource `json:"resources"`
		Allocations []*Allocation         `json:"allocations"`
		Quota       *QuotaReservation     `json:"quota"`
	}
)

var deploymentPlanMu sync.Mutex

// CreateDeploymentPlan persists desired state and uniquely reserves all required capacity.
func CreateDeploymentPlan(input DeploymentPlanInput) (result DeploymentPlanResult, errResult error) {
	deploymentPlanMu.Lock()
	defer deploymentPlanMu.Unlock()
	var (
		version   *BlueprintVersion
		blueprint *Blueprint
		document  BlueprintDocument
		groups    map[int]*CloudGroup
		totals    QuotaDimensions
		pools     map[string]*AllocationPool
	)
	if version, errResult = BlueprintVersions.Select(input.BlueprintVersionID); errResult != nil || version == nil {
		return result, fmt.Errorf("blueprint version was not found")
	}
	if blueprint, errResult = Blueprints.Select(version.BlueprintID); errResult != nil || blueprint == nil || blueprint.ProjectID != input.ProjectID || blueprint.ArchivedAt != nil {
		return result, fmt.Errorf("blueprint version does not belong to the project")
	}
	if document, errResult = BlueprintDocumentForVersion(version); errResult != nil {
		return
	}
	if groups, errResult = deploymentGroups(input.ProjectID, input.GroupIDs); errResult != nil {
		return
	}
	totals = blueprintQuota(document, len(input.GroupIDs))
	if pools, errResult = deploymentPools(input.ProjectID, input.AllocationPoolIDs, blueprintAllocationNeeds(document, len(input.GroupIDs))); errResult != nil {
		return
	}
	var prefix string = deploymentSlug(input.NamePrefix)
	if prefix == "" {
		var project *Project
		if project, _, errResult = GetProjectByID(input.ProjectID); errResult != nil || project == nil {
			return result, fmt.Errorf("project was not found")
		}
		prefix = project.Slug
	}
	if result.Quota, errResult = ReserveProjectQuota(QuotaReservationInput{ProjectID: input.ProjectID, Deployment: prefix, Dimensions: totals, ExpiresAt: time.Now().UTC().Add(24 * time.Hour)}); errResult != nil {
		return
	}
	defer func() {
		if errResult != nil {
			rollbackDeploymentPlan(result)
			_ = FinishQuotaReservation(result.Quota.ID, false)
		}
	}()
	for index, groupID := range input.GroupIDs {
		var deployment *Deployment
		var uuid string
		if uuid, errResult = randomUUID(); errResult != nil {
			return
		}
		var name string = fmt.Sprintf("%s-g%02d", prefix, index+1)
		var now time.Time = time.Now().UTC()
		deployment = &Deployment{UUID: uuid, ProjectID: input.ProjectID, BlueprintVersionID: version.ID, GroupID: groupID, QuotaReservationID: &result.Quota.ID, Name: name, Status: "planned", CreatedAt: now, UpdatedAt: now}
		if errResult = Deployments.Insert(deployment); errResult != nil {
			return
		}
		result.Deployments = append(result.Deployments, deployment)
		for _, resource := range document.Resources {
			var desired map[string]any
			var body []byte
			desired = map[string]any{"name": blueprintResourceName(document.NamePattern, name, resource.Key), "template": resource.Template, "vcpu": resource.VCPU, "memory_mb": resource.MemoryMB, "disk_gb": resource.DiskGB, "networks": resource.Networks, "configuration_role": resource.ConfigurationRole}
			if body, errResult = json.Marshal(desired); errResult != nil {
				return
			}
			var item *DeploymentResource = &DeploymentResource{DeploymentID: deployment.ID, ResourceKey: resource.Key, Kind: resource.Kind, DesiredJSON: string(body), CreatedAt: now}
			if errResult = DeploymentResources.Insert(item); errResult != nil {
				return
			}
			result.Resources = append(result.Resources, item)
			if errResult = appendDeploymentAllocation(&result, pools["vmid"], deployment.ID, "vmid:"+resource.Key); errResult != nil {
				return
			}
		}
		for _, network := range document.Networks {
			if errResult = appendDeploymentAllocation(&result, pools["vlan"], deployment.ID, "vlan:"+network.Key); errResult != nil {
				return
			}
			if network.Public {
				if errResult = appendDeploymentAllocation(&result, pools["ipv4"], deployment.ID, "ipv4:"+network.Key); errResult != nil {
					return
				}
				if errResult = appendDeploymentAllocation(&result, pools["external_port"], deployment.ID, "external_port:"+network.Key); errResult != nil {
					return
				}
			}
		}
		_ = groups[groupID]
	}
	return
}

// DeploymentsForProject lists persisted desired-state deployments.
func DeploymentsForProject(projectID int) (results []*Deployment, errResult error) {
	return Deployments.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(Deployments.FieldBySQLName("project_id"), gosqlite.OpEqual, projectID))
}

func deploymentGroups(projectID int, groupIDs []int) (groupsResult map[int]*CloudGroup, errResult error) {
	if len(groupIDs) == 0 {
		return nil, fmt.Errorf("at least one group is required")
	}
	var groups []*CloudGroup
	if groups, errResult = CloudGroupsForOwner(RoleBindingScopeProject, &projectID); errResult != nil {
		return
	}
	groupsResult = make(map[int]*CloudGroup, len(groups))
	for _, group := range groups {
		groupsResult[group.ID] = group
	}
	var seen map[int]bool = make(map[int]bool, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupsResult[groupID] == nil || seen[groupID] {
			return nil, fmt.Errorf("group IDs must be unique project groups")
		}
		seen[groupID] = true
	}
	return
}

func deploymentPools(projectID int, ids map[string]int, needs map[string]int) (poolsResult map[string]*AllocationPool, errResult error) {
	poolsResult = make(map[string]*AllocationPool, len(needs))
	for kind, needed := range needs {
		if needed == 0 {
			continue
		}
		var pool *AllocationPool
		if pool, errResult = AllocationPools.Select(ids[kind]); errResult != nil || pool == nil || pool.ProjectID != projectID || pool.Kind != kind || pool.ArchivedAt != nil {
			return nil, fmt.Errorf("a project %s allocation pool is required", kind)
		}
		var available int
		if available, errResult = AllocationPoolAvailable(pool); errResult != nil {
			return
		}
		if available < needed {
			return nil, fmt.Errorf("%s pool has %d available but %d are required", kind, available, needed)
		}
		poolsResult[kind] = pool
	}
	return
}

func appendDeploymentAllocation(result *DeploymentPlanResult, pool *AllocationPool, deploymentID int, purpose string) (errResult error) {
	var value string
	if value, errResult = nextPoolValue(pool); errResult != nil {
		return
	}
	var allocation *Allocation = &Allocation{PoolID: pool.ID, DeploymentID: deploymentID, AllocationKey: fmt.Sprintf("%d:%s", pool.ID, value), Purpose: purpose, Value: value, CreatedAt: time.Now().UTC()}
	if errResult = Allocations.Insert(allocation); errResult != nil {
		return
	}
	result.Allocations = append(result.Allocations, allocation)
	return
}

func nextPoolValue(pool *AllocationPool) (valueResult string, errResult error) {
	var allocations []*Allocation
	if allocations, errResult = Allocations.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(Allocations.FieldBySQLName("pool_id"), gosqlite.OpEqual, pool.ID)); errResult != nil {
		return
	}
	var used map[string]bool = make(map[string]bool, len(allocations))
	for _, allocation := range allocations {
		if allocation.ReleasedAt == nil {
			used[allocation.Value] = true
		}
	}
	for offset := pool.Start; offset <= pool.End; offset++ {
		var candidate string
		if candidate, errResult = allocationValue(pool, offset); errResult != nil {
			return
		}
		if !used[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s allocation pool is exhausted", pool.Kind)
}

func allocationValue(pool *AllocationPool, offset int) (valueResult string, errResult error) {
	if pool.Kind != "ipv4" && pool.Kind != "ipv6" {
		return strconv.Itoa(offset), nil
	}
	var prefix netip.Prefix
	if prefix, errResult = netip.ParsePrefix(pool.CIDR); errResult != nil {
		return
	}
	var address netip.Addr = prefix.Addr()
	for index := 0; index < offset; index++ {
		address = address.Next()
		if !address.IsValid() || !prefix.Contains(address) {
			return "", fmt.Errorf("address pool range exceeds CIDR")
		}
	}
	return address.String(), nil
}

func rollbackDeploymentPlan(result DeploymentPlanResult) {
	for index := len(result.Allocations) - 1; index >= 0; index-- {
		_ = Allocations.Delete(result.Allocations[index].ID)
	}
	for index := len(result.Resources) - 1; index >= 0; index-- {
		_ = DeploymentResources.Delete(result.Resources[index].ID)
	}
	for index := len(result.Deployments) - 1; index >= 0; index-- {
		_ = Deployments.Delete(result.Deployments[index].ID)
	}
}

func blueprintQuota(document BlueprintDocument, count int) (result QuotaDimensions) {
	for _, resource := range document.Resources {
		if resource.Kind == "ct" {
			result.Containers += count
		} else {
			result.VMs += count
		}
		result.VCPU += resource.VCPU * count
		result.MemoryMB += resource.MemoryMB * count
		result.StorageGB += resource.DiskGB * count
	}
	result.Networks = len(document.Networks) * count
	for _, network := range document.Networks {
		if network.Public {
			result.PublicIPs += count
		}
	}
	return
}

func blueprintAllocationNeeds(document BlueprintDocument, count int) (result map[string]int) {
	result = map[string]int{"vmid": len(document.Resources) * count, "vlan": len(document.Networks) * count}
	for _, network := range document.Networks {
		if network.Public {
			result["ipv4"] += count
			result["external_port"] += count
		}
	}
	return
}

func deploymentSlug(value string) (result string) {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
			result += string(character)
		}
	}
	return strings.Trim(result, "-")
}

func blueprintResourceName(pattern, deployment, resource string) string {
	return strings.ReplaceAll(strings.ReplaceAll(pattern, "{{deployment}}", deployment), "{{resource}}", resource)
}

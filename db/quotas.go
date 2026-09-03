package db

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/z46-dev/gosqlite"
)

type (
	QuotaDimensions struct {
		VMs        int `json:"vms"`
		Containers int `json:"containers"`
		VCPU       int `json:"vcpu"`
		MemoryMB   int `json:"memoryMB"`
		StorageGB  int `json:"storageGB"`
		Networks   int `json:"networks"`
		PublicIPs  int `json:"publicIPs"`
	}

	QuotaPolicyInput struct {
		Name          string
		Description   string
		MaxVMs        *int
		MaxContainers *int
		MaxVCPU       *int
		MaxMemoryMB   *int
		MaxStorageGB  *int
		MaxNetworks   *int
		MaxPublicIPs  *int
	}

	QuotaReservationInput struct {
		ProjectID  int
		JobID      *int
		Deployment string
		Dimensions QuotaDimensions
		ExpiresAt  time.Time
	}
)

var quotaReservationMu sync.Mutex

// CreateQuotaPolicy creates a reusable quota policy.
func CreateQuotaPolicy(input QuotaPolicyInput) (policyResult *QuotaPolicy, errResult error) {
	var policy *QuotaPolicy

	policy = &QuotaPolicy{CreatedAt: time.Now().UTC()}
	if errResult = applyQuotaPolicyInput(policy, input); errResult != nil {
		return nil, errResult
	}
	policy.UpdatedAt = policy.CreatedAt
	if errResult = QuotaPolicies.Insert(policy); errResult != nil {
		return nil, errResult
	}
	return policy, nil
}

// UpdateQuotaPolicy updates an active quota policy.
func UpdateQuotaPolicy(policy *QuotaPolicy, input QuotaPolicyInput) (errResult error) {
	if policy == nil || policy.ArchivedAt != nil {
		return fmt.Errorf("quota policy was not found")
	}
	if errResult = applyQuotaPolicyInput(policy, input); errResult != nil {
		return errResult
	}
	policy.UpdatedAt = time.Now().UTC()
	return QuotaPolicies.Update(policy)
}

// ArchiveQuotaPolicy archives a policy and removes its active bindings.
func ArchiveQuotaPolicy(policy *QuotaPolicy) (errResult error) {
	if policy == nil || policy.ArchivedAt != nil {
		return fmt.Errorf("quota policy was not found")
	}
	var now time.Time

	now = time.Now().UTC()
	policy.ArchivedAt = &now
	policy.UpdatedAt = now
	if _, errResult = QuotaBindings.DeleteWithFilter(gosqlite.NewFilter().KeyCmp(QuotaBindings.FieldBySQLName("quota_policy_id"), gosqlite.OpEqual, policy.ID)); errResult != nil {
		return errResult
	}
	return QuotaPolicies.Update(policy)
}

// BindQuotaPolicy binds a policy to an organization, project, or group.
func BindQuotaPolicy(policyID int, scopeType RoleBindingScope, scopeID int) (bindingResult *QuotaBinding, errResult error) {
	if scopeType != RoleBindingScopeOrg && scopeType != RoleBindingScopeProject && scopeType != RoleBindingScopeGroup {
		return nil, fmt.Errorf("unsupported quota scope")
	}
	if scopeID <= 0 {
		return nil, fmt.Errorf("quota scope is required")
	}
	var found bool

	switch scopeType {
	case RoleBindingScopeOrg:
		var organization *Organization
		organization, found, errResult = GetOrganizationByID(scopeID)
		if errResult == nil && (!found || organization.ArchivedAt != nil) {
			return nil, fmt.Errorf("organization was not found")
		}
	case RoleBindingScopeProject:
		var project *Project
		project, found, errResult = GetProjectByID(scopeID)
		if errResult == nil && (!found || !project.IsActive) {
			return nil, fmt.Errorf("project was not found")
		}
	case RoleBindingScopeGroup:
		var group *CloudGroup
		group, found, errResult = GetCloudGroupByID(scopeID)
		if errResult == nil && (!found || group.ArchivedAt != nil) {
			return nil, fmt.Errorf("group was not found")
		}
	}
	if errResult != nil {
		return nil, errResult
	}
	var policy *QuotaPolicy

	policy, errResult = QuotaPolicies.Select(policyID)
	if errResult != nil {
		return nil, errResult
	}
	if policy == nil || policy.ArchivedAt != nil {
		return nil, fmt.Errorf("quota policy was not found")
	}
	var existing []*QuotaBinding

	existing, errResult = QuotaBindings.SelectAllWithFilter(gosqlite.NewFilter().
		KeyCmp(QuotaBindings.FieldBySQLName("subject_type"), gosqlite.OpEqual, scopeType).
		And().KeyCmp(QuotaBindings.FieldBySQLName("subject_id"), gosqlite.OpEqual, scopeID).Limit(1))
	if errResult != nil {
		return nil, errResult
	}
	if len(existing) > 0 {
		existing[0].QuotaPolicyID = policyID
		if errResult = QuotaBindings.Update(existing[0]); errResult != nil {
			return nil, errResult
		}
		return existing[0], nil
	}
	var binding *QuotaBinding

	binding = &QuotaBinding{QuotaPolicyID: policyID, SubjectType: scopeType, SubjectID: scopeID, CreatedAt: time.Now().UTC()}
	if errResult = QuotaBindings.Insert(binding); errResult != nil {
		return nil, errResult
	}
	return binding, nil
}

// EffectiveProjectQuota resolves the most restrictive inherited project quota.
func EffectiveProjectQuota(projectID int, groupIDs ...int) (policyResult *QuotaPolicy, errResult error) {
	var (
		project   *Project
		found     bool
		bindings  []*QuotaBinding
		ancestors []int
	)

	project, found, errResult = GetProjectByID(projectID)
	if errResult != nil || !found {
		return nil, errResult
	}
	ancestors, errResult = OrganizationAncestorIDs(project.OrganizationID)
	if errResult != nil {
		return nil, errResult
	}
	bindings, errResult = QuotaBindings.SelectAll()
	if errResult != nil {
		return nil, errResult
	}
	var effective *QuotaPolicy

	for _, binding := range bindings {
		if !quotaBindingApplies(binding, projectID, ancestors, groupIDs) {
			continue
		}
		var policy *QuotaPolicy

		policy, errResult = QuotaPolicies.Select(binding.QuotaPolicyID)
		if errResult != nil {
			return nil, errResult
		}
		if policy == nil || policy.ArchivedAt != nil {
			continue
		}
		if effective == nil {
			var copy QuotaPolicy = *policy
			effective = &copy
			continue
		}
		mergeQuotaMinimums(effective, policy)
	}
	return effective, nil
}

// ProjectQuotaUsage calculates current provisioned usage.
func ProjectQuotaUsage(projectID int) (usageResult QuotaDimensions, errResult error) {
	var resources []*Resource

	resources, errResult = ResourcesForProject(projectID)
	if errResult != nil {
		return
	}
	for _, resource := range resources {
		switch resource.ResourceType {
		case ResourceTypeVM:
			usageResult.VMs++
			var details []*VirtualMachine
			details, errResult = VirtualMachines.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(VirtualMachines.FieldBySQLName("resource_id"), gosqlite.OpEqual, resource.ID).Limit(1))
			if len(details) > 0 {
				usageResult.VCPU += details[0].CPUCores
				usageResult.MemoryMB += details[0].MemoryMB
				if details[0].DiskGB != nil {
					usageResult.StorageGB += *details[0].DiskGB
				}
			}
		case ResourceTypeCT:
			usageResult.Containers++
			var details []*Container
			details, errResult = Containers.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(Containers.FieldBySQLName("resource_id"), gosqlite.OpEqual, resource.ID).Limit(1))
			if len(details) > 0 {
				usageResult.VCPU += details[0].CPUCores
				usageResult.MemoryMB += details[0].MemoryMB
				if details[0].DiskGB != nil {
					usageResult.StorageGB += *details[0].DiskGB
				}
			}
		case ResourceTypeNetwork:
			usageResult.Networks++
			var details []*VirtualNetwork
			details, errResult = VirtualNetworks.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(VirtualNetworks.FieldBySQLName("resource_id"), gosqlite.OpEqual, resource.ID).Limit(1))
			if len(details) > 0 && details[0].IsInternetRoutable {
				usageResult.PublicIPs++
			}
		}
		if errResult != nil {
			return
		}
	}
	return
}

// ReserveProjectQuota atomically checks and reserves intended project capacity.
func ReserveProjectQuota(input QuotaReservationInput) (reservationResult *QuotaReservation, errResult error) {
	quotaReservationMu.Lock()
	defer quotaReservationMu.Unlock()
	if input.Dimensions.VMs < 0 || input.Dimensions.Containers < 0 || input.Dimensions.VCPU < 0 || input.Dimensions.MemoryMB < 0 || input.Dimensions.StorageGB < 0 || input.Dimensions.Networks < 0 || input.Dimensions.PublicIPs < 0 {
		return nil, fmt.Errorf("quota reservation dimensions cannot be negative")
	}
	var (
		usage        QuotaDimensions
		policy       *QuotaPolicy
		reservations []*QuotaReservation
		now          time.Time = time.Now().UTC()
	)

	usage, errResult = ProjectQuotaUsage(input.ProjectID)
	if errResult != nil {
		return nil, errResult
	}
	policy, errResult = EffectiveProjectQuota(input.ProjectID)
	if errResult != nil {
		return nil, errResult
	}
	reservations, errResult = QuotaReservations.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(QuotaReservations.FieldBySQLName("project_id"), gosqlite.OpEqual, input.ProjectID))
	if errResult != nil {
		return nil, errResult
	}
	for _, reservation := range reservations {
		if reservation.State == QuotaReservationPending && reservation.ExpiresAt.After(now) {
			usage = addQuotaDimensions(usage, quotaDimensionsForReservation(reservation))
		}
	}
	usage = addQuotaDimensions(usage, input.Dimensions)
	if errResult = validateQuotaDimensions(policy, usage); errResult != nil {
		return nil, errResult
	}
	if input.ExpiresAt.IsZero() {
		input.ExpiresAt = now.Add(30 * time.Minute)
	}
	var reservation *QuotaReservation

	reservation = &QuotaReservation{ProjectID: input.ProjectID, JobID: input.JobID, Deployment: strings.TrimSpace(input.Deployment), VMs: input.Dimensions.VMs, Containers: input.Dimensions.Containers, VCPU: input.Dimensions.VCPU, MemoryMB: input.Dimensions.MemoryMB, StorageGB: input.Dimensions.StorageGB, Networks: input.Dimensions.Networks, PublicIPs: input.Dimensions.PublicIPs, State: QuotaReservationPending, ExpiresAt: input.ExpiresAt, CreatedAt: now, UpdatedAt: now}
	if errResult = QuotaReservations.Insert(reservation); errResult != nil {
		return nil, errResult
	}
	return reservation, nil
}

// FinishQuotaReservation commits or releases a reservation.
func FinishQuotaReservation(id int, committed bool) (errResult error) {
	quotaReservationMu.Lock()
	defer quotaReservationMu.Unlock()
	var reservation *QuotaReservation

	reservation, errResult = QuotaReservations.Select(id)
	if errResult != nil || reservation == nil {
		return errResult
	}
	reservation.State = QuotaReservationReleased
	if committed {
		reservation.State = QuotaReservationCommitted
	}
	reservation.UpdatedAt = time.Now().UTC()
	return QuotaReservations.Update(reservation)
}

func applyQuotaPolicyInput(policy *QuotaPolicy, input QuotaPolicyInput) (errResult error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return fmt.Errorf("quota policy name is required")
	}
	for _, value := range []*int{input.MaxVMs, input.MaxContainers, input.MaxVCPU, input.MaxMemoryMB, input.MaxStorageGB, input.MaxNetworks, input.MaxPublicIPs} {
		if value != nil && *value < 0 {
			return fmt.Errorf("quota limits cannot be negative")
		}
	}
	policy.Name, policy.Description = input.Name, strings.TrimSpace(input.Description)
	policy.MaxVMs, policy.MaxContainers, policy.MaxVCPU = input.MaxVMs, input.MaxContainers, input.MaxVCPU
	policy.MaxMemoryMB, policy.MaxStorageGB, policy.MaxNetworks, policy.MaxPublicIPs = input.MaxMemoryMB, input.MaxStorageGB, input.MaxNetworks, input.MaxPublicIPs
	return nil
}

func quotaBindingApplies(binding *QuotaBinding, projectID int, orgIDs []int, groupIDs []int) bool {
	if binding.SubjectType == RoleBindingScopeProject {
		return binding.SubjectID == projectID
	}
	var values []int
	if binding.SubjectType == RoleBindingScopeOrg {
		values = orgIDs
	} else if binding.SubjectType == RoleBindingScopeGroup {
		values = groupIDs
	}
	for _, value := range values {
		if value == binding.SubjectID {
			return true
		}
	}
	return false
}

func mergeQuotaMinimums(target, source *QuotaPolicy) {
	target.MaxVMs = minimumQuota(target.MaxVMs, source.MaxVMs)
	target.MaxContainers = minimumQuota(target.MaxContainers, source.MaxContainers)
	target.MaxVCPU = minimumQuota(target.MaxVCPU, source.MaxVCPU)
	target.MaxMemoryMB = minimumQuota(target.MaxMemoryMB, source.MaxMemoryMB)
	target.MaxStorageGB = minimumQuota(target.MaxStorageGB, source.MaxStorageGB)
	target.MaxNetworks = minimumQuota(target.MaxNetworks, source.MaxNetworks)
	target.MaxPublicIPs = minimumQuota(target.MaxPublicIPs, source.MaxPublicIPs)
}

func minimumQuota(first, second *int) *int {
	if first == nil {
		return second
	}
	if second == nil || *first <= *second {
		return first
	}
	return second
}
func addQuotaDimensions(a, b QuotaDimensions) QuotaDimensions {
	return QuotaDimensions{a.VMs + b.VMs, a.Containers + b.Containers, a.VCPU + b.VCPU, a.MemoryMB + b.MemoryMB, a.StorageGB + b.StorageGB, a.Networks + b.Networks, a.PublicIPs + b.PublicIPs}
}
func quotaDimensionsForReservation(r *QuotaReservation) QuotaDimensions {
	return QuotaDimensions{r.VMs, r.Containers, r.VCPU, r.MemoryMB, r.StorageGB, r.Networks, r.PublicIPs}
}
func validateQuotaDimensions(policy *QuotaPolicy, usage QuotaDimensions) error {
	if policy == nil {
		return nil
	}
	var checks = []struct {
		name  string
		used  int
		limit *int
	}{{"VMs", usage.VMs, policy.MaxVMs}, {"containers", usage.Containers, policy.MaxContainers}, {"vCPU", usage.VCPU, policy.MaxVCPU}, {"memory", usage.MemoryMB, policy.MaxMemoryMB}, {"storage", usage.StorageGB, policy.MaxStorageGB}, {"networks", usage.Networks, policy.MaxNetworks}, {"public addresses", usage.PublicIPs, policy.MaxPublicIPs}}
	for _, check := range checks {
		if check.limit != nil && check.used > *check.limit {
			return fmt.Errorf("quota exceeded for %s: %d > %d", check.name, check.used, *check.limit)
		}
	}
	return nil
}

package db

import (
	"testing"
	"time"
)

func TestMVPTableRegistration(t *testing.T) {
	initTestDB(t)

	registered := map[string]any{
		"Users":                 Users,
		"CloudGroups":           CloudGroups,
		"CloudGroupMemberships": CloudGroupMemberships,
		"Organizations":         Organizations,
		"Projects":              Projects,
		"ProjectMemberships":    ProjectMemberships,
		"Roles":                 Roles,
		"Permissions":           Permissions,
		"RolePermissions":       RolePermissions,
		"RoleBindings":          RoleBindings,
		"QuotaPolicies":         QuotaPolicies,
		"QuotaBindings":         QuotaBindings,
		"Resources":             Resources,
		"ProxmoxClusters":       ProxmoxClusters,
		"ProxmoxNodes":          ProxmoxNodes,
		"VirtualMachines":       VirtualMachines,
		"Containers":            Containers,
		"VirtualNetworks":       VirtualNetworks,
		"Jobs":                  Jobs,
		"JobLogs":               JobLogs,
		"AuditEvents":           AuditEvents,
		"Secrets":               Secrets,
	}

	for name, table := range registered {
		if table == nil {
			t.Fatalf("%s was not registered", name)
		}
	}

	if count, err := Organizations.Count(); err != nil {
		t.Fatalf("count organizations: %v", err)
	} else if count != 0 {
		t.Fatalf("expected empty organizations table, got %d rows", count)
	}
}

func TestMVPResourceChainInsert(t *testing.T) {
	initTestDB(t)

	now := time.Now().UTC()

	org := &Organization{
		UUID:      "00000000-0000-0000-0000-000000000001",
		Name:      "Lab",
		Slug:      "lab",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := Organizations.Insert(org); err != nil {
		t.Fatalf("insert organization: %v", err)
	}

	project := &Project{
		UUID:           "00000000-0000-0000-0000-000000000002",
		OrganizationID: org.ID,
		Name:           "Admin Infrastructure",
		Slug:           "admin-infra",
		ProjectType:    ProjectTypeAdmin,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := Projects.Insert(project); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	resource := &Resource{
		UUID:         "00000000-0000-0000-0000-000000000003",
		ProjectID:    project.ID,
		OwnerType:    OwnerTypeProject,
		OwnerID:      project.ID,
		ResourceType: ResourceTypeVM,
		Name:         "admin-vm-1",
		Slug:         "admin-vm-1",
		Status:       ResourceStatusCreating,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := Resources.Insert(resource); err != nil {
		t.Fatalf("insert resource: %v", err)
	}

	cluster := &ProxmoxCluster{
		UUID:      "00000000-0000-0000-0000-000000000004",
		Name:      "lab-pve",
		APIURL:    "https://pve.example.test:8006",
		VerifyTLS: true,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := ProxmoxClusters.Insert(cluster); err != nil {
		t.Fatalf("insert cluster: %v", err)
	}

	node := &ProxmoxNode{
		ClusterID:     cluster.ID,
		Name:          "pve01",
		Status:        "online",
		CPUTotal:      16,
		MemoryTotalMB: 65536,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := ProxmoxNodes.Insert(node); err != nil {
		t.Fatalf("insert node: %v", err)
	}

	vm := &VirtualMachine{
		ResourceID:  resource.ID,
		ClusterID:   cluster.ID,
		NodeID:      &node.ID,
		ProxmoxVMID: 1201,
		Name:        "admin-vm-1",
		CPUCores:    2,
		MemoryMB:    4096,
		PowerState:  PowerStateStopped,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := VirtualMachines.Insert(vm); err != nil {
		t.Fatalf("insert vm: %v", err)
	}

	fetched, err := VirtualMachines.Select(vm.ID)
	if err != nil {
		t.Fatalf("select vm: %v", err)
	}
	if fetched.ResourceID != resource.ID {
		t.Fatalf("expected resource id %d, got %d", resource.ID, fetched.ResourceID)
	}
	if fetched.NodeID == nil || *fetched.NodeID != node.ID {
		t.Fatalf("expected node id %d, got %#v", node.ID, fetched.NodeID)
	}
}

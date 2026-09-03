package db

import (
	"context"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/proxmox"
)

func TestSyncProxmoxInventoryScopesToManagedTagAndRetainsMissing(t *testing.T) {
	initTestDB(t)
	var fake *proxmox.FakeService = &proxmox.FakeService{
		Nodes:    []proxmox.Node{{Name: "pve-a", Status: "online"}},
		Storages: []proxmox.Storage{{ID: "local-zfs", Node: "pve-a", Active: true}},
		Networks: []proxmox.Network{{ID: "vmbr0", Node: "pve-a", Active: true}},
		Guests: []proxmox.Guest{
			{VMID: 101, Node: "pve-a", Name: "managed-vm", Kind: "qemu", Tags: []string{"organesson-managed"}, CPUCores: 2},
			{VMID: 999, Node: "pve-a", Name: "someone-elses-vm", Kind: "qemu", Tags: []string{"other"}},
		},
	}
	var result *ProxmoxInventorySyncResult
	var err error
	if result, err = SyncProxmoxInventory(context.Background(), fake, "test-cluster", "organesson-managed"); err != nil {
		t.Fatalf("SyncProxmoxInventory returned error: %v", err)
	}
	if len(result.Guests) != 1 || result.Guests[0].ProxmoxVMID != 101 || result.Guests[0].DriftState != ProxmoxDriftUnmanaged {
		t.Fatalf("unexpected managed inventory: %#v", result.Guests)
	}
	if len(result.Nodes) != 1 || len(result.Storages) != 1 || len(result.Networks) != 1 {
		t.Fatalf("expected cluster context in sync result: %#v", result)
	}
	fake.Guests = nil
	if result, err = SyncProxmoxInventory(context.Background(), fake, "test-cluster", "organesson-managed"); err != nil {
		t.Fatalf("second sync returned error: %v", err)
	}
	if len(result.Guests) != 1 || result.Guests[0].DriftState != ProxmoxDriftMissing || result.Guests[0].MissingSince == nil {
		t.Fatalf("expected missing history to be retained: %#v", result.Guests)
	}
}

func TestSyncProxmoxInventoryLinksExistingIdentityAndDetectsChange(t *testing.T) {
	initTestDB(t)
	var err error
	if err = EnsureInitialSetup(); err != nil {
		t.Fatalf("EnsureInitialSetup returned error: %v", err)
	}
	var project *Project
	var resource *Resource
	if project, err = CreateProject(ProjectCreateInput{Name: "Inventory", Slug: "inventory"}); err != nil {
		t.Fatalf("CreateProject returned error: %v", err)
	}
	if resource, err = CreateResource(ResourceCreateInput{ProjectID: project.ID, Name: "Lab VM", Slug: "lab-vm", ResourceType: ResourceTypeVM}); err != nil {
		t.Fatalf("CreateResource returned error: %v", err)
	}
	var now time.Time = time.Now().UTC()
	var cluster *ProxmoxCluster = &ProxmoxCluster{UUID: "test-cluster", Name: "Test", APIURL: "https://pve.test", VerifyTLS: true, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err = ProxmoxClusters.Insert(cluster); err != nil {
		t.Fatalf("insert cluster: %v", err)
	}
	if err = VirtualMachines.Insert(&VirtualMachine{ResourceID: resource.ID, ClusterID: cluster.ID, ProxmoxVMID: 1201, Name: "lab-vm", CPUCores: 2, MemoryMB: 2048, PowerState: PowerStateStopped, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("insert virtual machine: %v", err)
	}
	var fake *proxmox.FakeService = &proxmox.FakeService{Guests: []proxmox.Guest{{VMID: 1201, Node: "pve-a", Name: "lab-vm", Kind: "qemu", Status: "running", Tags: []string{"organesson-managed"}, CPUCores: 2}}}
	var result *ProxmoxInventorySyncResult
	if result, err = SyncProxmoxInventory(context.Background(), fake, "test-cluster", "organesson-managed"); err != nil {
		t.Fatalf("first sync returned error: %v", err)
	}
	if len(result.Guests) != 1 || result.Guests[0].ResourceID == nil || *result.Guests[0].ResourceID != resource.ID || result.Guests[0].DriftState != ProxmoxDriftInSync {
		t.Fatalf("expected linked in-sync guest, got %#v", result.Guests)
	}
	var machine *VirtualMachine
	var found bool
	if machine, found, err = VirtualMachineForResource(resource.ID); err != nil || !found || machine.PowerState != PowerStateRunning {
		t.Fatalf("expected sync to refresh linked power state, machine=%#v found=%t err=%v", machine, found, err)
	}
	fake.Guests[0].CPUCores = 4
	if result, err = SyncProxmoxInventory(context.Background(), fake, "test-cluster", "organesson-managed"); err != nil {
		t.Fatalf("changed sync returned error: %v", err)
	}
	if result.Guests[0].DriftState != ProxmoxDriftChanged {
		t.Fatalf("expected changed drift, got %#v", result.Guests[0])
	}
	if result, err = SyncProxmoxInventory(context.Background(), fake, "test-cluster", "organesson-managed"); err != nil {
		t.Fatalf("stable sync returned error: %v", err)
	}
	if result.Guests[0].DriftState != ProxmoxDriftInSync {
		t.Fatalf("expected stable guest to return in sync, got %#v", result.Guests[0])
	}
}

func TestSyncProxmoxInventoryMarksDuplicateIdentityAmbiguous(t *testing.T) {
	initTestDB(t)
	var fake *proxmox.FakeService = &proxmox.FakeService{Guests: []proxmox.Guest{
		{VMID: 101, Node: "pve-a", Kind: "qemu", Tags: []string{"organesson-managed"}},
		{VMID: 101, Node: "pve-b", Kind: "qemu", Tags: []string{"organesson-managed"}},
	}}
	var result *ProxmoxInventorySyncResult
	var err error
	if result, err = SyncProxmoxInventory(context.Background(), fake, "test-cluster", "organesson-managed"); err != nil {
		t.Fatalf("SyncProxmoxInventory returned error: %v", err)
	}
	if len(result.Guests) != 1 || result.Guests[0].DriftState != ProxmoxDriftAmbiguous {
		t.Fatalf("expected ambiguous identity, got %#v", result.Guests)
	}
}

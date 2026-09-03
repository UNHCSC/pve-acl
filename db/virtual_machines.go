package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/UNHCSC/organesson/proxmox"
	"github.com/z46-dev/gosqlite"
)

type ProxmoxGuestImportInput struct {
	ProjectID       int
	Name            string
	Slug            string
	ClusterIdentity string
	APIURL          string
	VerifyTLS       bool
	Guest           proxmox.Guest
	CreatedByUserID *int
}

// VirtualMachineForResource returns the Proxmox identity linked to a local resource.
func VirtualMachineForResource(resourceID int) (machineResult *VirtualMachine, okResult bool, errResult error) {
	var machines []*VirtualMachine
	if machines, errResult = VirtualMachines.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(VirtualMachines.FieldBySQLName("resource_id"), gosqlite.OpEqual, resourceID).Limit(2)); errResult != nil || len(machines) == 0 {
		return nil, false, errResult
	}
	if len(machines) != 1 {
		return nil, false, nil
	}
	return machines[0], true, nil
}

// UpdateVirtualMachinePower stores the last provider-observed power state and freshness timestamp.
func UpdateVirtualMachinePower(machine *VirtualMachine, state PowerState) (errResult error) {
	machine.PowerState = state
	machine.UpdatedAt = time.Now().UTC()
	return VirtualMachines.Update(machine)
}

// ImportProxmoxGuest creates a local resource linked to one discovered Proxmox QEMU guest.
func ImportProxmoxGuest(input ProxmoxGuestImportInput) (resourceResult *Resource, errResult error) {
	if input.Guest.Kind != "qemu" || input.Guest.Template {
		return nil, fmt.Errorf("only non-template QEMU guests can be imported")
	}
	var existingIDs []int
	if existingIDs, errResult = proxmoxResourceIDs(input.ClusterIdentity, input.Guest.VMID); errResult != nil {
		return
	}
	if len(existingIDs) != 0 {
		return nil, fmt.Errorf("Proxmox guest is already linked to a resource")
	}
	var (
		cluster *ProxmoxCluster
		node    *ProxmoxNode
		now     time.Time = time.Now().UTC()
	)
	if cluster, errResult = ensureProxmoxCluster(input.ClusterIdentity, input.APIURL, input.VerifyTLS, now); errResult != nil {
		return
	}
	if node, errResult = ensureProxmoxNode(cluster.ID, input.Guest.Node, now); errResult != nil {
		return
	}
	var resourceType ResourceType = ResourceTypeVM
	if resourceResult, errResult = CreateResource(ResourceCreateInput{ProjectID: input.ProjectID, Name: input.Name, Slug: input.Slug, ResourceType: resourceType, Status: ResourceStatusReady, CreatedByUserID: input.CreatedByUserID}); errResult != nil {
		return
	}
	var memoryMB int = int(input.Guest.MemoryTotal / (1024 * 1024))
	var diskGB *int
	if input.Guest.DiskTotal > 0 {
		var value int = int(input.Guest.DiskTotal / (1024 * 1024 * 1024))
		diskGB = &value
	}
	var machine *VirtualMachine = &VirtualMachine{ResourceID: resourceResult.ID, ClusterID: cluster.ID, NodeID: &node.ID, ProxmoxVMID: input.Guest.VMID, Name: input.Guest.Name, CPUCores: input.Guest.CPUCores, MemoryMB: memoryMB, DiskGB: diskGB, OSType: input.Guest.OSType, PowerState: PowerStateFromProxmox(input.Guest.Status), CreatedAt: now, UpdatedAt: now}
	if errResult = VirtualMachines.Insert(machine); errResult != nil {
		_ = ArchiveResource(resourceResult)
		return nil, errResult
	}
	var inventory []*ProxmoxInventoryGuest
	if inventory, errResult = ListProxmoxInventoryGuests(input.ClusterIdentity); errResult != nil {
		return
	}
	for _, item := range inventory {
		if item.ProxmoxVMID == input.Guest.VMID {
			item.ResourceID = &resourceResult.ID
			item.DriftState = ProxmoxDriftInSync
			item.LastError = ""
			item.UpdatedAt = now
			errResult = ProxmoxInventoryGuests.Update(item)
			break
		}
	}
	return
}

func ensureProxmoxCluster(identity, apiURL string, verifyTLS bool, now time.Time) (clusterResult *ProxmoxCluster, errResult error) {
	var clusters []*ProxmoxCluster
	if clusters, errResult = ProxmoxClusters.SelectAll(); errResult != nil {
		return
	}
	for _, cluster := range clusters {
		if cluster.UUID == identity {
			return cluster, nil
		}
	}
	clusterResult = &ProxmoxCluster{UUID: strings.TrimSpace(identity), Name: strings.TrimSpace(identity), APIURL: strings.TrimSpace(apiURL), VerifyTLS: verifyTLS, IsActive: true, CreatedAt: now, UpdatedAt: now}
	errResult = ProxmoxClusters.Insert(clusterResult)
	return
}

func ensureProxmoxNode(clusterID int, name string, now time.Time) (nodeResult *ProxmoxNode, errResult error) {
	var nodes []*ProxmoxNode
	if nodes, errResult = ProxmoxNodes.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(ProxmoxNodes.FieldBySQLName("cluster_id"), gosqlite.OpEqual, clusterID).And().KeyCmp(ProxmoxNodes.FieldBySQLName("name"), gosqlite.OpEqual, name)); errResult != nil {
		return
	}
	if len(nodes) > 0 {
		return nodes[0], nil
	}
	nodeResult = &ProxmoxNode{ClusterID: clusterID, Name: strings.TrimSpace(name), Status: "online", CreatedAt: now, UpdatedAt: now}
	errResult = ProxmoxNodes.Insert(nodeResult)
	return
}

// PowerStateFromProxmox maps a provider status string to the local power-state enum.
func PowerStateFromProxmox(status string) (stateResult PowerState) {
	stateResult = PowerStateUnknown
	if strings.EqualFold(status, "running") {
		stateResult = PowerStateRunning
	} else if strings.EqualFold(status, "stopped") {
		stateResult = PowerStateStopped
	} else if strings.EqualFold(status, "paused") {
		stateResult = PowerStatePaused
	}
	return
}

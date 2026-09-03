package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/UNHCSC/organesson/proxmox"
	"github.com/z46-dev/gosqlite"
)

const (
	ProxmoxDriftInSync    ProxmoxDriftState = "in_sync"
	ProxmoxDriftMissing   ProxmoxDriftState = "missing"
	ProxmoxDriftChanged   ProxmoxDriftState = "changed"
	ProxmoxDriftUnmanaged ProxmoxDriftState = "unmanaged"
	ProxmoxDriftAmbiguous ProxmoxDriftState = "ambiguous"
	ProxmoxDriftError     ProxmoxDriftState = "error"
)

type ProxmoxInventorySyncResult struct {
	ClusterIdentity string                   `json:"cluster_identity"`
	ManagedTag      string                   `json:"managed_tag"`
	Nodes           []proxmox.Node           `json:"nodes"`
	Storages        []proxmox.Storage        `json:"storages"`
	Networks        []proxmox.Network        `json:"networks"`
	Guests          []*ProxmoxInventoryGuest `json:"guests"`
	SyncedAt        time.Time                `json:"synced_at"`
}

// SyncProxmoxInventory reconciles only explicitly tagged guests and never mutates Proxmox or adopts local ownership.
func SyncProxmoxInventory(ctx context.Context, service proxmox.Service, clusterIdentity, managedTag string) (resultResult *ProxmoxInventorySyncResult, errResult error) {
	clusterIdentity = strings.TrimSpace(clusterIdentity)
	managedTag = strings.TrimSpace(managedTag)
	if service == nil || clusterIdentity == "" || managedTag == "" {
		return nil, fmt.Errorf("Proxmox service, cluster identity, and managed tag are required")
	}
	var (
		nodes    []proxmox.Node
		storages []proxmox.Storage
		networks []proxmox.Network
		guests   []proxmox.Guest
	)
	if errResult = service.Health(ctx); errResult != nil {
		return nil, fmt.Errorf("Proxmox health check: %w", errResult)
	}
	if nodes, errResult = service.ListNodes(ctx); errResult != nil {
		return nil, fmt.Errorf("list Proxmox nodes: %w", errResult)
	}
	if storages, errResult = service.ListStorages(ctx); errResult != nil {
		return nil, fmt.Errorf("list Proxmox storages: %w", errResult)
	}
	if networks, errResult = service.ListNetworks(ctx); errResult != nil {
		return nil, fmt.Errorf("list Proxmox networks: %w", errResult)
	}
	if guests, errResult = service.ListGuests(ctx); errResult != nil {
		return nil, fmt.Errorf("list Proxmox guests: %w", errResult)
	}
	var now time.Time = time.Now().UTC()
	var discovered map[int][]proxmox.Guest = make(map[int][]proxmox.Guest)
	for _, guest := range guests {
		if proxmox.HasTag(guest, managedTag) {
			discovered[guest.VMID] = append(discovered[guest.VMID], guest)
		}
	}
	var existing []*ProxmoxInventoryGuest
	if existing, errResult = ListProxmoxInventoryGuests(clusterIdentity); errResult != nil {
		return
	}
	var existingByVMID map[int]*ProxmoxInventoryGuest = make(map[int]*ProxmoxInventoryGuest, len(existing))
	for _, item := range existing {
		existingByVMID[item.ProxmoxVMID] = item
	}
	for vmID, candidates := range discovered {
		var (
			guest       proxmox.Guest          = candidates[0]
			item        *ProxmoxInventoryGuest = existingByVMID[vmID]
			resourceIDs []int
		)
		if len(candidates) > 1 {
			if item, errResult = upsertProxmoxInventoryGuest(item, clusterIdentity, guest, ProxmoxDriftAmbiguous, "multiple guests share this cluster/VMID identity", now); errResult != nil {
				return
			}
			delete(existingByVMID, vmID)
			continue
		}
		if resourceIDs, errResult = proxmoxResourceIDs(clusterIdentity, vmID); errResult != nil {
			return
		}
		if len(resourceIDs) > 1 {
			if item, errResult = upsertProxmoxInventoryGuest(item, clusterIdentity, guest, ProxmoxDriftAmbiguous, "multiple local resources share this cluster/VMID identity", now); errResult != nil {
				return
			}
			delete(existingByVMID, vmID)
			continue
		}
		var previouslySeen bool = item != nil
		if len(resourceIDs) == 1 {
			if item == nil {
				item = &ProxmoxInventoryGuest{}
			}
			item.ResourceID = &resourceIDs[0]
		}
		if guest, errResult = service.GetGuest(ctx, guest.Node, guest.VMID); errResult != nil {
			if item, errResult = upsertProxmoxInventoryGuest(item, clusterIdentity, candidates[0], ProxmoxDriftError, errResult.Error(), now); errResult != nil {
				return
			}
			delete(existingByVMID, vmID)
			continue
		}
		if !proxmox.HasTag(guest, managedTag) {
			continue
		}
		var nextFingerprint string
		if nextFingerprint, errResult = proxmoxGuestFingerprint(guest); errResult != nil {
			return
		}
		var driftState ProxmoxDriftState = ProxmoxDriftUnmanaged
		if item != nil && item.ResourceID != nil {
			driftState = ProxmoxDriftInSync
			if previouslySeen && item.Fingerprint != nextFingerprint {
				driftState = ProxmoxDriftChanged
			}
		}
		if item, errResult = upsertProxmoxInventoryGuest(item, clusterIdentity, guest, driftState, "", now); errResult != nil {
			return
		}
		delete(existingByVMID, vmID)
	}
	for _, item := range existingByVMID {
		if item.MissingSince == nil {
			item.MissingSince = &now
		}
		item.DriftState = ProxmoxDriftMissing
		item.LastError = "managed guest was not returned with the required tag"
		item.UpdatedAt = now
		if errResult = ProxmoxInventoryGuests.Update(item); errResult != nil {
			return
		}
	}
	resultResult = &ProxmoxInventorySyncResult{ClusterIdentity: clusterIdentity, ManagedTag: managedTag, Nodes: nodes, Storages: storages, Networks: networks, SyncedAt: now}
	resultResult.Guests, errResult = ListProxmoxInventoryGuests(clusterIdentity)
	return
}

func proxmoxResourceIDs(clusterIdentity string, vmID int) (idsResult []int, errResult error) {
	var clusters []*ProxmoxCluster
	if clusters, errResult = ProxmoxClusters.SelectAll(); errResult != nil {
		return
	}
	for _, cluster := range clusters {
		if cluster.UUID != clusterIdentity && cluster.Name != clusterIdentity {
			continue
		}
		var virtualMachines []*VirtualMachine
		if virtualMachines, errResult = VirtualMachines.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(VirtualMachines.FieldBySQLName("cluster_id"), gosqlite.OpEqual, cluster.ID).And().KeyCmp(VirtualMachines.FieldBySQLName("proxmox_vmid"), gosqlite.OpEqual, vmID)); errResult != nil {
			return
		}
		for _, virtualMachine := range virtualMachines {
			idsResult = append(idsResult, virtualMachine.ResourceID)
		}
	}
	return
}

// ListProxmoxInventoryGuests returns retained managed-guest history for one cluster.
func ListProxmoxInventoryGuests(clusterIdentity string) (itemsResult []*ProxmoxInventoryGuest, errResult error) {
	return ProxmoxInventoryGuests.SelectAllWithFilter(gosqlite.NewFilter().KeyCmp(ProxmoxInventoryGuests.FieldBySQLName("cluster_identity"), gosqlite.OpEqual, clusterIdentity))
}

func upsertProxmoxInventoryGuest(item *ProxmoxInventoryGuest, clusterIdentity string, guest proxmox.Guest, driftState ProxmoxDriftState, lastError string, now time.Time) (itemResult *ProxmoxInventoryGuest, errResult error) {
	var fingerprint string
	if fingerprint, errResult = proxmoxGuestFingerprint(guest); errResult != nil {
		return
	}
	if item == nil {
		item = &ProxmoxInventoryGuest{Identity: clusterIdentity + "/" + strconv.Itoa(guest.VMID), ClusterIdentity: clusterIdentity, ProxmoxVMID: guest.VMID, FirstSeenAt: now}
	}
	if item.Identity == "" {
		item.Identity = clusterIdentity + "/" + strconv.Itoa(guest.VMID)
		item.ClusterIdentity = clusterIdentity
		item.ProxmoxVMID = guest.VMID
		item.FirstSeenAt = now
	}
	item.Node = guest.Node
	item.Name = guest.Name
	item.Kind = guest.Kind
	item.IsTemplate = guest.Template
	item.Status = guest.Status
	item.Tags = strings.Join(guest.Tags, ";")
	item.Fingerprint = fingerprint
	item.DriftState = driftState
	item.LastError = lastError
	item.LastSeenAt = now
	item.MissingSince = nil
	item.UpdatedAt = now
	if item.ID == 0 {
		errResult = ProxmoxInventoryGuests.Insert(item)
	} else {
		errResult = ProxmoxInventoryGuests.Update(item)
	}
	return item, errResult
}

func proxmoxGuestFingerprint(guest proxmox.Guest) (valueResult string, errResult error) {
	guest.Tags = append([]string(nil), guest.Tags...)
	slices.Sort(guest.Tags)
	var fingerprintSource struct {
		VMID        int      `json:"vmid"`
		Node        string   `json:"node"`
		Name        string   `json:"name"`
		Kind        string   `json:"kind"`
		Tags        []string `json:"tags"`
		Template    bool     `json:"template"`
		CPUCores    int      `json:"cpu_cores"`
		MemoryTotal int64    `json:"memory_total"`
		DiskTotal   int64    `json:"disk_total"`
		OSType      string   `json:"os_type"`
	}
	fingerprintSource.VMID, fingerprintSource.Node, fingerprintSource.Name, fingerprintSource.Kind = guest.VMID, guest.Node, guest.Name, guest.Kind
	fingerprintSource.Tags, fingerprintSource.Template, fingerprintSource.CPUCores = guest.Tags, guest.Template, guest.CPUCores
	fingerprintSource.MemoryTotal, fingerprintSource.DiskTotal, fingerprintSource.OSType = guest.MemoryTotal, guest.DiskTotal, guest.OSType
	var serialized []byte
	if serialized, errResult = json.Marshal(fingerprintSource); errResult != nil {
		return
	}
	var digest [sha256.Size]byte = sha256.Sum256(serialized)
	valueResult = hex.EncodeToString(digest[:])
	return
}

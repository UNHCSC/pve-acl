package db

import (
	"time"

	"github.com/z46-dev/gosqlite"
)

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

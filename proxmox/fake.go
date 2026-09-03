package proxmox

import (
	"context"
	"fmt"
)

type FakeService struct {
	HealthError   error
	Nodes         []Node
	Storages      []Storage
	Networks      []Network
	Guests        []Guest
	GetGuestError map[int]error
}

func (service *FakeService) Health(context.Context) (errResult error) {
	return service.HealthError
}

func (service *FakeService) ListNodes(context.Context) (itemsResult []Node, errResult error) {
	return append([]Node(nil), service.Nodes...), nil
}

func (service *FakeService) ListStorages(context.Context) (itemsResult []Storage, errResult error) {
	return append([]Storage(nil), service.Storages...), nil
}

func (service *FakeService) ListNetworks(context.Context) (itemsResult []Network, errResult error) {
	return append([]Network(nil), service.Networks...), nil
}

func (service *FakeService) ListGuests(context.Context) (itemsResult []Guest, errResult error) {
	return append([]Guest(nil), service.Guests...), nil
}

func (service *FakeService) GetGuest(_ context.Context, node string, vmID int) (guestResult Guest, errResult error) {
	if errResult = service.GetGuestError[vmID]; errResult != nil {
		return
	}
	for _, guest := range service.Guests {
		if guest.VMID == vmID && guest.Node == node {
			return guest, nil
		}
	}
	errResult = fmt.Errorf("guest %s/%d was not found", node, vmID)
	return
}

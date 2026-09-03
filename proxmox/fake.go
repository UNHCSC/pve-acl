package proxmox

import (
	"context"
	"fmt"
)

type FakeService struct {
	HealthError    error
	Nodes          []Node
	Storages       []Storage
	Networks       []Network
	Guests         []Guest
	GetGuestError  map[int]error
	ActionError    error
	Actions        []string
	Tasks          map[string]Task
	ConsoleTickets map[int]ConsoleTicket
}

func (service *FakeService) action(operation, node string, vmID int) (taskResult string, errResult error) {
	service.Actions = append(service.Actions, fmt.Sprintf("%s:%s:%d", operation, node, vmID))
	return "UPID:test", service.ActionError
}

func (service *FakeService) StartGuest(_ context.Context, node string, vmID int) (string, error) {
	return service.action("start", node, vmID)
}
func (service *FakeService) ShutdownGuest(_ context.Context, node string, vmID int) (string, error) {
	return service.action("shutdown", node, vmID)
}
func (service *FakeService) StopGuest(_ context.Context, node string, vmID int) (string, error) {
	return service.action("stop", node, vmID)
}
func (service *FakeService) RebootGuest(_ context.Context, node string, vmID int) (string, error) {
	return service.action("reboot", node, vmID)
}
func (service *FakeService) PauseGuest(_ context.Context, node string, vmID int) (string, error) {
	return service.action("pause", node, vmID)
}
func (service *FakeService) ResumeGuest(_ context.Context, node string, vmID int) (string, error) {
	return service.action("resume", node, vmID)
}
func (service *FakeService) GetTask(_ context.Context, node, taskID string) (Task, error) {
	return service.Tasks[taskID], nil
}
func (service *FakeService) CreateConsoleTicket(_ context.Context, node string, vmID int) (ConsoleTicket, error) {
	return service.ConsoleTickets[vmID], service.ActionError
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

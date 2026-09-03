package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UNHCSC/organesson/proxmox"
	"github.com/gofiber/fiber/v2"
)

func TestProxmoxInventoryAPIRequiresAdminAndFiltersUntaggedGuests(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var original proxmoxIntegrationState = proxmoxIntegration
	proxmoxIntegration = proxmoxIntegrationState{
		enabled:         true,
		clusterIdentity: "test-cluster",
		managedTag:      proxmox.DefaultManagedTag,
		service: &proxmox.FakeService{Guests: []proxmox.Guest{
			{VMID: 101, Node: "pve-a", Name: "managed", Kind: "qemu", Tags: []string{proxmox.DefaultManagedTag}},
			{VMID: 999, Node: "pve-a", Name: "unmanaged", Kind: "qemu", Tags: []string{"someone-else"}},
		}},
	}
	t.Cleanup(func() { proxmoxIntegration = original })
	var fiberApp *fiber.App = newAuthenticatedFiberApp()
	fiberApp.Post("/api/v1/proxmox/inventory/sync", postProxmoxInventorySync)
	var userToken string = authenticateTestUser(t, "inventory-user", false)
	var adminToken string = authenticateTestUser(t, "inventory-admin", true)
	var request *http.Request
	var response *http.Response
	var err error
	request = httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/inventory/sync", nil)
	request.Header.Set("Authorization", "Bearer "+userToken)
	if response, err = testFiberRequest(fiberApp, request); err != nil {
		t.Fatalf("non-admin request returned error: %v", err)
	}
	if response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected non-admin request to be forbidden, got %d", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/inventory/sync", nil)
	request.Header.Set("Authorization", "Bearer "+adminToken)
	if response, err = testFiberRequest(fiberApp, request); err != nil {
		t.Fatalf("admin sync returned error: %v", err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("expected admin sync 200, got %d", response.StatusCode)
	}
	var body struct {
		Guests []struct {
			VMID int `json:"vmid"`
		} `json:"guests"`
	}
	if err = json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode sync response: %v", err)
	}
	if len(body.Guests) != 1 || body.Guests[0].VMID != 101 {
		t.Fatalf("expected only tagged guest, got %#v", body.Guests)
	}
}

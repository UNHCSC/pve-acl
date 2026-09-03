package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/db"
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

func TestCreateResourceImportsOnlyFreshTaggedGuest(t *testing.T) {
	initACLTestDB(t)
	ensureInitialSetupForTest(t)
	var project *db.Project = createResourceAPIProject(t, "Imported VMs")
	var fake *proxmox.FakeService = &proxmox.FakeService{Guests: []proxmox.Guest{{VMID: 501, Node: "pve-a", Name: "managed-vm", Kind: "qemu", Status: "stopped", Tags: []string{proxmox.DefaultManagedTag}, CPUCores: 2, MemoryTotal: 512 * 1024 * 1024}}}
	var original proxmoxIntegrationState = proxmoxIntegration
	var originalConfig config.Configuration = config.Config
	proxmoxIntegration = proxmoxIntegrationState{enabled: true, clusterIdentity: "test-cluster", managedTag: proxmox.DefaultManagedTag, service: fake}
	config.Config.Proxmox.Hostname = "pve.test"
	config.Config.Proxmox.Port = "8006"
	config.Config.Proxmox.VerifyTLS = true
	t.Cleanup(func() { proxmoxIntegration = original; config.Config = originalConfig })
	var syncResult *db.ProxmoxInventorySyncResult
	var err error
	if syncResult, err = db.SyncProxmoxInventory(t.Context(), fake, "test-cluster", proxmox.DefaultManagedTag); err != nil {
		t.Fatalf("sync inventory: %v", err)
	}
	var app *fiber.App = newAuthenticatedFiberApp()
	app.Post("/api/v1/projects/:id/resources", postCreateProjectResource)
	var token string = authenticateTestUser(t, "import-admin", true)
	var payload []byte
	if payload, err = json.Marshal(map[string]any{"name": "Course VM", "resourceType": "vm", "proxmoxInventoryGuestID": syncResult.Guests[0].ID}); err != nil {
		t.Fatal(err)
	}
	var request *http.Request = httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+strconv.Itoa(project.ID)+"/resources", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	var response *http.Response
	if response, err = testFiberRequest(app, request); err != nil || response.StatusCode != fiber.StatusCreated {
		t.Fatalf("import status=%d err=%v", response.StatusCode, err)
	}
	var resource db.Resource
	if err = json.NewDecoder(response.Body).Decode(&resource); err != nil {
		t.Fatal(err)
	}
	var machine *db.VirtualMachine
	var found bool
	if machine, found, err = db.VirtualMachineForResource(resource.ID); err != nil || !found || machine.ProxmoxVMID != 501 {
		t.Fatalf("linked machine=%#v found=%t err=%v", machine, found, err)
	}
	fake.Guests = []proxmox.Guest{{VMID: 502, Node: "pve-a", Name: "lost-tag", Kind: "qemu", Tags: []string{proxmox.DefaultManagedTag}}}
	if syncResult, err = db.SyncProxmoxInventory(t.Context(), fake, "test-cluster", proxmox.DefaultManagedTag); err != nil {
		t.Fatal(err)
	}
	fake.Guests[0].Tags = nil
	if payload, err = json.Marshal(map[string]any{"name": "Unsafe VM", "resourceType": "vm", "proxmoxInventoryGuestID": syncResult.Guests[1].ID}); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+strconv.Itoa(project.ID)+"/resources", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	if response, err = testFiberRequest(app, request); err != nil || response.StatusCode == fiber.StatusCreated {
		t.Fatalf("lost-tag import status=%d err=%v", response.StatusCode, err)
	}
}

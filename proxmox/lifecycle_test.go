package proxmox_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/proxmox"
	"github.com/fasthttp/websocket"
)

func TestRealClusterTaggedGuestPowerLifecycle(t *testing.T) {
	if os.Getenv("ORGANESSON_PROXMOX_LIFECYCLE") != "1" {
		t.Skip("set ORGANESSON_PROXMOX_LIFECYCLE=1 with an explicit node and VMID to run mutations")
	}
	var node string = os.Getenv("ORGANESSON_PROXMOX_TEST_NODE")
	var vmID int
	var err error
	if os.Getenv("ORGANESSON_PROXMOX_TEST_VMID") != "" {
		if vmID, err = strconv.Atoi(os.Getenv("ORGANESSON_PROXMOX_TEST_VMID")); err != nil || vmID <= 0 {
			t.Fatal("ORGANESSON_PROXMOX_TEST_VMID must be a positive integer")
		}
	}
	if err = config.Init("../config.toml"); err != nil {
		t.Fatalf("load config: %v", err)
	}
	var baseURL string
	if baseURL, err = smokeBaseURL(config.Config.Proxmox.Hostname, config.Config.Proxmox.Port); err != nil {
		t.Fatalf("base URL: %v", err)
	}
	var client *proxmox.Client
	if client, err = proxmox.NewClient(proxmox.ClientConfig{BaseURL: baseURL, TokenID: config.Config.Proxmox.TokenID, Secret: config.Config.Proxmox.Secret, VerifyTLS: config.Config.Proxmox.VerifyTLS, TLSFingerprintSHA256: config.Config.Proxmox.TLSFingerprintSHA256}); err != nil {
		t.Fatalf("client: %v", err)
	}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var created bool
	if vmID == 0 {
		if os.Getenv("ORGANESSON_PROXMOX_CREATE_TEST_GUEST") != "1" {
			t.Fatal("provide ORGANESSON_PROXMOX_TEST_VMID or explicitly set ORGANESSON_PROXMOX_CREATE_TEST_GUEST=1")
		}
		if node == "" {
			var nodes []proxmox.Node
			if nodes, err = client.ListNodes(ctx); err != nil || len(nodes) == 0 {
				t.Fatalf("select test node: %v", err)
			}
			node = nodes[0].Name
		}
		if vmID, err = client.NextVMID(ctx); err != nil {
			t.Fatalf("allocate test VMID: %v", err)
		}
		var createTask string
		if createTask, err = client.CreateTestGuest(ctx, node, vmID, "organesson-lifecycle-"+strconv.Itoa(vmID), config.Config.Proxmox.ManagedTag); err != nil {
			t.Fatalf("create tagged test guest: %v", err)
		}
		waitForTask(t, ctx, client, node, createTask)
		created = true
		defer cleanupTestGuest(t, client, node, vmID, config.Config.Proxmox.ManagedTag)
	}
	var guest proxmox.Guest
	if guest, err = client.GetGuest(ctx, node, vmID); err != nil {
		t.Fatalf("get test guest: %v", err)
	}
	if !proxmox.HasTag(guest, config.Config.Proxmox.ManagedTag) {
		t.Fatal("refusing lifecycle test: exact managed tag is absent")
	}
	if guest.Status != "stopped" {
		t.Fatal("refusing lifecycle test: test guest must begin stopped")
	}
	var taskID string
	if taskID, err = client.StartGuest(ctx, node, vmID); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForTask(t, ctx, client, node, taskID)
	defer func() {
		if taskID, err = client.ShutdownGuest(context.Background(), node, vmID); err == nil {
			waitForTask(t, context.Background(), client, node, taskID)
		}
	}()
	if guest, err = client.GetGuest(ctx, node, vmID); err != nil || guest.Status != "running" {
		t.Fatalf("expected running guest, guest=%#v err=%v", guest, err)
	}
	var ticket proxmox.ConsoleTicket
	if ticket, err = client.CreateConsoleTicket(ctx, node, vmID); err != nil {
		t.Fatalf("create console ticket: %v", err)
	}
	var connection *websocket.Conn
	if connection, err = client.DialConsole(ctx, node, vmID, ticket.Port, ticket.Ticket); err != nil {
		t.Fatalf("open console websocket: %v", err)
	}
	_ = connection.Close()
	t.Logf("real lifecycle and console succeeded for tagged guest %d (fixture created=%t)", vmID, created)
}

func cleanupTestGuest(t *testing.T, client *proxmox.Client, node string, vmID int, managedTag string) {
	t.Helper()
	var guest proxmox.Guest
	var err error
	if guest, err = client.GetGuest(context.Background(), node, vmID); err != nil {
		t.Errorf("inspect test guest for cleanup: %v", err)
		return
	}
	if guest.Status != "stopped" {
		var stopTask string
		if stopTask, err = client.StopGuest(context.Background(), node, vmID); err != nil {
			t.Errorf("stop test guest for cleanup: %v", err)
			return
		}
		waitForTask(t, context.Background(), client, node, stopTask)
	}
	var deleteTask string
	if deleteTask, err = client.DeleteTestGuest(context.Background(), node, vmID, managedTag); err != nil {
		t.Errorf("delete test guest: %v", err)
		return
	}
	waitForTask(t, context.Background(), client, node, deleteTask)
}

func waitForTask(t *testing.T, ctx context.Context, client *proxmox.Client, node, taskID string) {
	t.Helper()
	for {
		var task proxmox.Task
		var err error
		if task, err = client.GetTask(ctx, node, taskID); err != nil {
			t.Fatalf("poll task: %v", err)
		}
		if task.Status == "stopped" {
			if task.ExitStatus != "OK" {
				t.Fatalf("task failed: %s", task.ExitStatus)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("task timeout")
		case <-time.After(500 * time.Millisecond):
		}
	}
}

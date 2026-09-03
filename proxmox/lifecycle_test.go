package proxmox_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/proxmox"
)

func TestRealClusterTaggedGuestPowerLifecycle(t *testing.T) {
	if os.Getenv("ORGANESSON_PROXMOX_LIFECYCLE") != "1" {
		t.Skip("set ORGANESSON_PROXMOX_LIFECYCLE=1 with an explicit node and VMID to run mutations")
	}
	var node string = os.Getenv("ORGANESSON_PROXMOX_TEST_NODE")
	var vmID int
	var err error
	if vmID, err = strconv.Atoi(os.Getenv("ORGANESSON_PROXMOX_TEST_VMID")); err != nil || node == "" || vmID <= 0 {
		t.Fatal("ORGANESSON_PROXMOX_TEST_NODE and ORGANESSON_PROXMOX_TEST_VMID are required")
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

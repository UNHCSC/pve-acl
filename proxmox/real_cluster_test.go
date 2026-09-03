package proxmox_test

import (
	"context"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/proxmox"
)

func TestRealClusterReadOnlyInventory(t *testing.T) {
	if os.Getenv("ORGANESSON_PROXMOX_SMOKE") != "1" {
		t.Skip("set ORGANESSON_PROXMOX_SMOKE=1 to run the read-only real-cluster smoke test")
	}
	var err error
	if err = config.Init("../config.toml"); err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	if !config.Config.Proxmox.Enabled {
		t.Fatal("Proxmox integration is disabled")
	}
	var baseURL string
	if baseURL, err = smokeBaseURL(config.Config.Proxmox.Hostname, config.Config.Proxmox.Port); err != nil {
		t.Fatalf("build Proxmox URL: %v", err)
	}
	var service *proxmox.Client
	if service, err = proxmox.NewClient(proxmox.ClientConfig{BaseURL: baseURL, TokenID: config.Config.Proxmox.TokenID, Secret: config.Config.Proxmox.Secret, VerifyTLS: config.Config.Proxmox.VerifyTLS, TLSFingerprintSHA256: config.Config.Proxmox.TLSFingerprintSHA256}); err != nil {
		t.Fatalf("create Proxmox client: %v", err)
	}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err = service.Health(ctx); err != nil {
		t.Fatalf("health check: %v", err)
	}
	var (
		nodes    []proxmox.Node
		storages []proxmox.Storage
		networks []proxmox.Network
		guests   []proxmox.Guest
	)
	if nodes, err = service.ListNodes(ctx); err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if storages, err = service.ListStorages(ctx); err != nil {
		t.Fatalf("list storages: %v", err)
	}
	if networks, err = service.ListNetworks(ctx); err != nil {
		t.Fatalf("list networks: %v", err)
	}
	if guests, err = service.ListGuests(ctx); err != nil {
		t.Fatalf("list guests: %v", err)
	}
	var managedCount int
	for _, guest := range guests {
		if !proxmox.HasTag(guest, config.Config.Proxmox.ManagedTag) {
			continue
		}
		managedCount++
		if _, err = service.GetGuest(ctx, guest.Node, guest.VMID); err != nil {
			t.Fatalf("get tagged guest %d: %v", guest.VMID, err)
		}
	}
	t.Logf("read-only inventory succeeded: %d nodes, %d storage records, %d networks, %d tagged guests", len(nodes), len(storages), len(networks), managedCount)
}

func smokeBaseURL(hostname, port string) (valueResult string, errResult error) {
	if strings.Contains(hostname, "://") {
		var parsed *url.URL
		if parsed, errResult = url.Parse(hostname); errResult != nil {
			return
		}
		if parsed.Port() == "" {
			parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
		}
		return strings.TrimRight(parsed.String(), "/"), nil
	}
	return "https://" + net.JoinHostPort(hostname, port), nil
}

package proxmox

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientUsesTokenAndMapsGuestInventory(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "PVEAPIToken=root@pam!organesson=secret" {
			t.Errorf("unexpected authorization header")
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(response).Encode(map[string]any{"data": map[string]any{"version": "9.0"}})
		case "/api2/json/cluster/resources":
			_ = json.NewEncoder(response).Encode(map[string]any{"data": []map[string]any{{"vmid": 101, "node": "pve-a", "name": "lab-ad", "type": "qemu", "tags": "organesson-managed;class-a", "status": "running", "maxcpu": 4, "maxmem": 8589934592}}})
		case "/api2/json/nodes/pve-a/qemu/101/config":
			_ = json.NewEncoder(response).Encode(map[string]any{"data": map[string]any{"ostype": "win11", "tags": "class-a;organesson-managed"}})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	var certificateDigest [sha256.Size]byte = sha256.Sum256(server.Certificate().Raw)
	var tlsConfig *tls.Config
	var err error
	if tlsConfig, err = newTLSConfig("example.com", true, hex.EncodeToString(certificateDigest[:])); err != nil {
		t.Fatalf("newTLSConfig returned error: %v", err)
	}
	if err = tlsConfig.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{server.Certificate()}}); err != nil {
		t.Fatalf("pinned certificate verification failed: %v", err)
	}
	var client *Client
	if client, err = NewClient(ClientConfig{BaseURL: server.URL, TokenID: "root@pam!organesson", Secret: "secret", VerifyTLS: false}); err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if err = client.Health(context.Background()); err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	var guests []Guest
	if guests, err = client.ListGuests(context.Background()); err != nil {
		t.Fatalf("ListGuests returned error: %v", err)
	}
	if len(guests) != 1 || guests[0].VMID != 101 || !HasTag(guests[0], DefaultManagedTag) {
		t.Fatalf("unexpected guests: %#v", guests)
	}
	var guest Guest
	if guest, err = client.GetGuest(context.Background(), "pve-a", 101); err != nil {
		t.Fatalf("GetGuest returned error: %v", err)
	}
	if guest.OSType != "win11" || !strings.Contains(strings.Join(guest.Tags, ";"), DefaultManagedTag) {
		t.Fatalf("unexpected guest detail: %#v", guest)
	}
}

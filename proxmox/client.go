package proxmox

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

type ClientConfig struct {
	BaseURL              string
	TokenID              string
	Secret               string
	VerifyTLS            bool
	TLSFingerprintSHA256 string
	Timeout              time.Duration
}

type Client struct {
	baseURL    *url.URL
	tokenID    string
	secret     string
	httpClient *http.Client
}

type apiResponse[T any] struct {
	Data T `json:"data"`
}

type apiResource struct {
	ID       string  `json:"id"`
	Node     string  `json:"node"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Type     string  `json:"type"`
	Tags     string  `json:"tags"`
	Content  string  `json:"content"`
	Storage  string  `json:"storage"`
	OSType   string  `json:"ostype"`
	Template int     `json:"template"`
	Active   int     `json:"active"`
	Shared   int     `json:"shared"`
	CPU      float64 `json:"cpu"`
	MaxCPU   int     `json:"maxcpu"`
	Mem      int64   `json:"mem"`
	MaxMem   int64   `json:"maxmem"`
	Disk     int64   `json:"disk"`
	MaxDisk  int64   `json:"maxdisk"`
	Uptime   int64   `json:"uptime"`
	VMID     int     `json:"vmid"`
	Total    int64   `json:"total"`
	Used     int64   `json:"used"`
	Avail    int64   `json:"avail"`
}

type apiNetwork struct {
	Iface   string `json:"iface"`
	Type    string `json:"type"`
	Bridge  string `json:"bridge_ports"`
	CIDR    string `json:"cidr"`
	Address string `json:"address"`
	Netmask string `json:"netmask"`
	Gateway string `json:"gateway"`
	Active  int    `json:"active"`
}

// NewClient creates a read-only Proxmox API adapter.
func NewClient(clientConfig ClientConfig) (clientResult *Client, errResult error) {
	var parsedURL *url.URL
	if parsedURL, errResult = url.Parse(strings.TrimRight(clientConfig.BaseURL, "/")); errResult != nil {
		return
	}
	if parsedURL.Scheme != "https" || parsedURL.Host == "" {
		errResult = fmt.Errorf("Proxmox base URL must be an absolute HTTPS URL")
		return
	}
	if clientConfig.Timeout <= 0 {
		clientConfig.Timeout = 15 * time.Second
	}
	var tlsConfig *tls.Config
	if tlsConfig, errResult = newTLSConfig(parsedURL.Hostname(), clientConfig.VerifyTLS, clientConfig.TLSFingerprintSHA256); errResult != nil {
		return
	}
	clientResult = &Client{
		baseURL: parsedURL,
		tokenID: strings.TrimSpace(clientConfig.TokenID),
		secret:  strings.TrimSpace(clientConfig.Secret),
		httpClient: &http.Client{
			Timeout:   clientConfig.Timeout,
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		},
	}
	if clientResult.tokenID == "" || clientResult.secret == "" {
		errResult = fmt.Errorf("Proxmox token ID and secret are required")
		clientResult = nil
	}
	return
}

func newTLSConfig(hostname string, verifyTLS bool, fingerprint string) (configResult *tls.Config, errResult error) {
	fingerprint = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(fingerprint), ":", ""))
	if fingerprint != "" {
		var expected []byte
		if expected, errResult = hex.DecodeString(fingerprint); errResult != nil || len(expected) != sha256.Size {
			return nil, fmt.Errorf("invalid Proxmox TLS SHA-256 fingerprint")
		}
		configResult = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} // #nosec G402 -- the callback verifies hostname and the configured certificate pin.
		configResult.VerifyConnection = func(state tls.ConnectionState) (verifyErr error) {
			if len(state.PeerCertificates) == 0 {
				return fmt.Errorf("Proxmox did not present a TLS certificate")
			}
			if verifyErr = state.PeerCertificates[0].VerifyHostname(hostname); verifyErr != nil {
				return verifyErr
			}
			var actual [sha256.Size]byte = sha256.Sum256(state.PeerCertificates[0].Raw)
			if !slices.Equal(actual[:], expected) {
				return fmt.Errorf("Proxmox TLS certificate fingerprint mismatch")
			}
			return
		}
		return
	}
	configResult = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: !verifyTLS} // #nosec G402 -- explicitly controlled by operator configuration.
	return
}

func (client *Client) Health(ctx context.Context) (errResult error) {
	var version map[string]any
	errResult = client.get(ctx, "/version", nil, &version)
	return
}

func (client *Client) ListNodes(ctx context.Context) (itemsResult []Node, errResult error) {
	var resources []apiResource
	if errResult = client.get(ctx, "/nodes", nil, &resources); errResult != nil {
		return
	}
	for _, item := range resources {
		itemsResult = append(itemsResult, Node{Name: item.Node, Status: item.Status, CPUUsage: item.CPU, CPUTotal: item.MaxCPU, MemoryUsed: item.Mem, MemoryTotal: item.MaxMem, UptimeSeconds: item.Uptime})
	}
	return
}

func (client *Client) ListStorages(ctx context.Context) (itemsResult []Storage, errResult error) {
	var nodes []Node
	if nodes, errResult = client.ListNodes(ctx); errResult != nil {
		return
	}
	for _, node := range nodes {
		var resources []apiResource
		if errResult = client.get(ctx, "/nodes/"+url.PathEscape(node.Name)+"/storage", nil, &resources); errResult != nil {
			return
		}
		for _, item := range resources {
			itemsResult = append(itemsResult, Storage{ID: item.Storage, Node: node.Name, Type: item.Type, Content: item.Content, Available: item.Avail, Total: item.Total, Used: item.Used, Active: item.Active == 1, Shared: item.Shared == 1})
		}
	}
	return
}

func (client *Client) ListNetworks(ctx context.Context) (itemsResult []Network, errResult error) {
	var nodes []Node
	if nodes, errResult = client.ListNodes(ctx); errResult != nil {
		return
	}
	for _, node := range nodes {
		var resources []apiNetwork
		if errResult = client.get(ctx, "/nodes/"+url.PathEscape(node.Name)+"/network", nil, &resources); errResult != nil {
			return
		}
		for _, item := range resources {
			var cidr string = item.CIDR
			if cidr == "" && item.Address != "" {
				cidr = item.Address
				if item.Netmask != "" {
					cidr += "/" + item.Netmask
				}
			}
			itemsResult = append(itemsResult, Network{ID: item.Iface, Node: node.Name, Type: item.Type, Bridge: item.Bridge, CIDR: cidr, Gateway: item.Gateway, Active: item.Active == 1})
		}
	}
	return
}

func (client *Client) ListGuests(ctx context.Context) (itemsResult []Guest, errResult error) {
	var resources []apiResource
	var query url.Values = url.Values{"type": []string{"vm"}}
	if errResult = client.get(ctx, "/cluster/resources", query, &resources); errResult != nil {
		return
	}
	for _, item := range resources {
		if item.Type != "qemu" && item.Type != "lxc" {
			continue
		}
		itemsResult = append(itemsResult, guestFromResource(item))
	}
	return
}

func (client *Client) GetGuest(ctx context.Context, node string, vmID int) (guestResult Guest, errResult error) {
	var guests []Guest
	if guests, errResult = client.ListGuests(ctx); errResult != nil {
		return
	}
	for _, guest := range guests {
		if guest.Node != node || guest.VMID != vmID {
			continue
		}
		var config map[string]any
		if errResult = client.get(ctx, "/nodes/"+url.PathEscape(node)+"/"+guest.Kind+"/"+strconv.Itoa(vmID)+"/config", nil, &config); errResult != nil {
			return Guest{}, errResult
		}
		if value, ok := config["ostype"].(string); ok {
			guest.OSType = value
		}
		if value, ok := config["tags"].(string); ok {
			guest.Tags = ParseTags(value)
		}
		return guest, nil
	}
	errResult = fmt.Errorf("guest %s/%d was not found", node, vmID)
	return
}

func guestFromResource(item apiResource) (guestResult Guest) {
	return Guest{VMID: item.VMID, Node: item.Node, Name: item.Name, Kind: item.Type, Status: item.Status, Tags: ParseTags(item.Tags), Template: item.Template == 1, CPUUsage: item.CPU, CPUCores: item.MaxCPU, MemoryUsed: item.Mem, MemoryTotal: item.MaxMem, DiskUsed: item.Disk, DiskTotal: item.MaxDisk, UptimeSeconds: item.Uptime, OSType: item.OSType}
}

func (client *Client) get(ctx context.Context, apiPath string, query url.Values, output any) (errResult error) {
	var endpoint *url.URL = client.baseURL.JoinPath("api2", "json", strings.TrimPrefix(apiPath, "/"))
	endpoint.RawQuery = query.Encode()
	var request *http.Request
	if request, errResult = http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil); errResult != nil {
		return
	}
	request.Header.Set("Authorization", "PVEAPIToken="+client.tokenID+"="+client.secret)
	request.Header.Set("Accept", "application/json")
	var response *http.Response
	if response, errResult = client.httpClient.Do(request); errResult != nil {
		errResult = fmt.Errorf("Proxmox request failed: %w", errResult)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errResult = fmt.Errorf("Proxmox API returned %s", response.Status)
		return
	}
	var envelope apiResponse[json.RawMessage]
	if errResult = json.NewDecoder(response.Body).Decode(&envelope); errResult != nil {
		errResult = fmt.Errorf("decode Proxmox response: %w", errResult)
		return
	}
	if errResult = json.Unmarshal(envelope.Data, output); errResult != nil {
		errResult = fmt.Errorf("decode Proxmox data: %w", errResult)
	}
	return
}

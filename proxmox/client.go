package proxmox

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/fasthttp/websocket"
	"io"
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
	ID        string  `json:"id"`
	Node      string  `json:"node"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	QMPStatus string  `json:"qmpstatus"`
	Type      string  `json:"type"`
	Tags      string  `json:"tags"`
	Content   string  `json:"content"`
	Storage   string  `json:"storage"`
	OSType    string  `json:"ostype"`
	Template  int     `json:"template"`
	Active    int     `json:"active"`
	Shared    int     `json:"shared"`
	CPU       float64 `json:"cpu"`
	MaxCPU    int     `json:"maxcpu"`
	Mem       int64   `json:"mem"`
	MaxMem    int64   `json:"maxmem"`
	Disk      int64   `json:"disk"`
	MaxDisk   int64   `json:"maxdisk"`
	Uptime    int64   `json:"uptime"`
	VMID      int     `json:"vmid"`
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Avail     int64   `json:"avail"`
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

// Permissions returns the effective API-token privileges by Proxmox object path.
func (client *Client) Permissions(ctx context.Context) (permissionsResult map[string]map[string]int, errResult error) {
	errResult = client.get(ctx, "/access/permissions", nil, &permissionsResult)
	return
}

// NextVMID asks Proxmox for the next unused VM identifier.
func (client *Client) NextVMID(ctx context.Context) (vmIDResult int, errResult error) {
	var value json.Number
	if errResult = client.get(ctx, "/cluster/nextid", nil, &value); errResult == nil {
		vmIDResult, errResult = strconv.Atoi(value.String())
	}
	return
}

// CreateTestGuest allocates a diskless QEMU guest for an explicitly gated lifecycle test.
func (client *Client) CreateTestGuest(ctx context.Context, node string, vmID int, name, managedTag string) (taskResult string, errResult error) {
	var values url.Values = url.Values{"vmid": []string{strconv.Itoa(vmID)}, "name": []string{name}, "tags": []string{managedTag + ";organesson-test"}, "cores": []string{"1"}, "memory": []string{"256"}, "ostype": []string{"l26"}, "scsihw": []string{"virtio-scsi-single"}}
	errResult = client.request(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/qemu", values, &taskResult)
	return
}

// DeleteTestGuest deletes only a stopped guest that still carries the exact managed and test tags.
func (client *Client) DeleteTestGuest(ctx context.Context, node string, vmID int, managedTag string) (taskResult string, errResult error) {
	var guest Guest
	if guest, errResult = client.GetGuest(ctx, node, vmID); errResult != nil {
		return
	}
	if guest.Status != "stopped" || !HasTag(guest, managedTag) || !HasTag(guest, "organesson-test") {
		return "", fmt.Errorf("refusing deletion: guest is not a stopped Organesson test guest")
	}
	errResult = client.request(ctx, http.MethodDelete, "/nodes/"+url.PathEscape(node)+"/qemu/"+strconv.Itoa(vmID), nil, &taskResult)
	return
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
		var current apiResource
		if errResult = client.get(ctx, "/nodes/"+url.PathEscape(node)+"/"+guest.Kind+"/"+strconv.Itoa(vmID)+"/status/current", nil, &current); errResult != nil {
			return Guest{}, errResult
		}
		guest.Status = current.Status
		if current.QMPStatus == "paused" {
			guest.Status = "paused"
		}
		return guest, nil
	}
	errResult = fmt.Errorf("guest %s/%d was not found", node, vmID)
	return
}

func (client *Client) guestAction(ctx context.Context, node string, vmID int, operation string) (taskResult string, errResult error) {
	var data string
	errResult = client.request(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/qemu/"+strconv.Itoa(vmID)+"/status/"+operation, nil, &data)
	return data, errResult
}

func (client *Client) StartGuest(ctx context.Context, node string, vmID int) (string, error) {
	return client.guestAction(ctx, node, vmID, "start")
}
func (client *Client) ShutdownGuest(ctx context.Context, node string, vmID int) (string, error) {
	return client.guestAction(ctx, node, vmID, "shutdown")
}
func (client *Client) StopGuest(ctx context.Context, node string, vmID int) (string, error) {
	return client.guestAction(ctx, node, vmID, "stop")
}
func (client *Client) RebootGuest(ctx context.Context, node string, vmID int) (string, error) {
	return client.guestAction(ctx, node, vmID, "reboot")
}
func (client *Client) PauseGuest(ctx context.Context, node string, vmID int) (string, error) {
	return client.guestAction(ctx, node, vmID, "suspend")
}
func (client *Client) ResumeGuest(ctx context.Context, node string, vmID int) (string, error) {
	return client.guestAction(ctx, node, vmID, "resume")
}

func (client *Client) GetTask(ctx context.Context, node, taskID string) (taskResult Task, errResult error) {
	var data struct {
		Status     string `json:"status"`
		ExitStatus string `json:"exitstatus"`
	}
	if errResult = client.get(ctx, "/nodes/"+url.PathEscape(node)+"/tasks/"+url.PathEscape(taskID)+"/status", nil, &data); errResult == nil {
		taskResult = Task{ID: taskID, Status: data.Status, ExitStatus: data.ExitStatus}
	}
	return
}

func (client *Client) CreateConsoleTicket(ctx context.Context, node string, vmID int) (ticketResult ConsoleTicket, errResult error) {
	var data struct {
		Ticket string      `json:"ticket"`
		Port   json.Number `json:"port"`
		User   string      `json:"user"`
	}
	if errResult = client.request(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/qemu/"+strconv.Itoa(vmID)+"/vncproxy", url.Values{"websocket": []string{"1"}}, &data); errResult != nil {
		return
	}
	ticketResult.Ticket, ticketResult.User, ticketResult.ExpiresAt = data.Ticket, data.User, time.Now().UTC().Add(2*time.Minute)
	ticketResult.Port, _ = strconv.Atoi(data.Port.String())
	return
}

func (client *Client) DialConsole(ctx context.Context, node string, vmID, port int, ticket string) (connectionResult *websocket.Conn, errResult error) {
	var endpoint *url.URL = client.baseURL.JoinPath("api2", "json", "nodes", node, "qemu", strconv.Itoa(vmID), "vncwebsocket")
	endpoint.Scheme = "wss"
	endpoint.RawQuery = url.Values{"port": []string{strconv.Itoa(port)}, "vncticket": []string{ticket}}.Encode()
	var transport *http.Transport = client.httpClient.Transport.(*http.Transport)
	var dialer websocket.Dialer = websocket.Dialer{TLSClientConfig: transport.TLSClientConfig.Clone(), HandshakeTimeout: 15 * time.Second}
	var response *http.Response
	connectionResult, response, errResult = dialer.DialContext(ctx, endpoint.String(), http.Header{"Authorization": []string{"PVEAPIToken=" + client.tokenID + "=" + client.secret}})
	if response != nil {
		_ = response.Body.Close()
	}
	if errResult != nil {
		errResult = fmt.Errorf("connect Proxmox console: %w", errResult)
	}
	return
}

func guestFromResource(item apiResource) (guestResult Guest) {
	return Guest{VMID: item.VMID, Node: item.Node, Name: item.Name, Kind: item.Type, Status: item.Status, Tags: ParseTags(item.Tags), Template: item.Template == 1, CPUUsage: item.CPU, CPUCores: item.MaxCPU, MemoryUsed: item.Mem, MemoryTotal: item.MaxMem, DiskUsed: item.Disk, DiskTotal: item.MaxDisk, UptimeSeconds: item.Uptime, OSType: item.OSType}
}

func (client *Client) get(ctx context.Context, apiPath string, query url.Values, output any) (errResult error) {
	return client.request(ctx, http.MethodGet, apiPath, query, output)
}

func (client *Client) request(ctx context.Context, method, apiPath string, values url.Values, output any) (errResult error) {
	var endpoint *url.URL = client.baseURL.JoinPath("api2", "json", strings.TrimPrefix(apiPath, "/"))
	var body io.Reader
	if method == http.MethodGet {
		endpoint.RawQuery = values.Encode()
	} else if values != nil {
		body = strings.NewReader(values.Encode())
	}
	var request *http.Request
	if request, errResult = http.NewRequestWithContext(ctx, method, endpoint.String(), body); errResult != nil {
		return
	}
	request.Header.Set("Authorization", "PVEAPIToken="+client.tokenID+"="+client.secret)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
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

package app

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/UNHCSC/organesson/config"
	"github.com/UNHCSC/organesson/proxmox"
	jobscheduler "github.com/UNHCSC/organesson/scheduler"
)

type proxmoxIntegrationState struct {
	service         proxmox.Service
	clusterIdentity string
	managedTag      string
	enabled         bool
}

var proxmoxIntegration proxmoxIntegrationState

// configureProxmoxIntegration builds the read-only adapter from application configuration.
func configureProxmoxIntegration() (errResult error) {
	proxmoxIntegration = proxmoxIntegrationState{enabled: config.Config.Proxmox.Enabled}
	if !config.Config.Proxmox.Enabled {
		return
	}
	var baseURL string
	if baseURL, errResult = proxmoxBaseURL(config.Config.Proxmox.Hostname, config.Config.Proxmox.Port); errResult != nil {
		return
	}
	var service proxmox.Service
	if service, errResult = proxmox.NewClient(proxmox.ClientConfig{BaseURL: baseURL, TokenID: config.Config.Proxmox.TokenID, Secret: config.Config.Proxmox.Secret, VerifyTLS: config.Config.Proxmox.VerifyTLS, TLSFingerprintSHA256: config.Config.Proxmox.TLSFingerprintSHA256}); errResult != nil {
		return
	}
	var clusterIdentity string = strings.TrimSpace(config.Config.Proxmox.ClusterID)
	if clusterIdentity == "" {
		clusterIdentity = strings.TrimSpace(config.Config.Proxmox.Hostname)
	}
	proxmoxIntegration = proxmoxIntegrationState{service: service, clusterIdentity: clusterIdentity, managedTag: strings.TrimSpace(config.Config.Proxmox.ManagedTag), enabled: true}
	if jobscheduler.Default() != nil {
		if errResult = jobscheduler.Default().RegisterConsumer(jobscheduler.TaskTypeProxmoxAction, consumeProxmoxAction); errResult != nil {
			return
		}
	}
	return
}

func proxmoxBaseURL(hostname, port string) (valueResult string, errResult error) {
	hostname = strings.TrimSpace(hostname)
	port = strings.TrimSpace(port)
	if strings.Contains(hostname, "://") {
		var parsed *url.URL
		if parsed, errResult = url.Parse(hostname); errResult != nil {
			return
		}
		if parsed.Scheme != "https" || parsed.Host == "" {
			return "", fmt.Errorf("Proxmox hostname URL must use HTTPS")
		}
		if parsed.Port() == "" && port != "" {
			parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
		}
		valueResult = strings.TrimRight(parsed.String(), "/")
		return
	}
	if hostname == "" || port == "" {
		return "", fmt.Errorf("Proxmox hostname and port are required")
	}
	valueResult = "https://" + net.JoinHostPort(hostname, port)
	return
}

// Package proxmox defines the application-owned boundary to Proxmox VE.
package proxmox

import (
	"context"
	"slices"
	"strings"
)

const DefaultManagedTag = "organesson-managed"

type (
	Service interface {
		Health(ctx context.Context) error
		ListNodes(ctx context.Context) ([]Node, error)
		ListStorages(ctx context.Context) ([]Storage, error)
		ListNetworks(ctx context.Context) ([]Network, error)
		ListGuests(ctx context.Context) ([]Guest, error)
		GetGuest(ctx context.Context, node string, vmID int) (Guest, error)
	}

	Node struct {
		Name          string  `json:"name"`
		Status        string  `json:"status"`
		CPUUsage      float64 `json:"cpu_usage"`
		CPUTotal      int     `json:"cpu_total"`
		MemoryUsed    int64   `json:"memory_used"`
		MemoryTotal   int64   `json:"memory_total"`
		UptimeSeconds int64   `json:"uptime_seconds"`
	}

	Storage struct {
		ID        string `json:"id"`
		Node      string `json:"node,omitempty"`
		Type      string `json:"type"`
		Content   string `json:"content,omitempty"`
		Available int64  `json:"available"`
		Total     int64  `json:"total"`
		Used      int64  `json:"used"`
		Active    bool   `json:"active"`
		Shared    bool   `json:"shared"`
	}

	Network struct {
		ID      string `json:"id"`
		Node    string `json:"node,omitempty"`
		Type    string `json:"type"`
		Bridge  string `json:"bridge,omitempty"`
		CIDR    string `json:"cidr,omitempty"`
		Gateway string `json:"gateway,omitempty"`
		Active  bool   `json:"active"`
	}

	Guest struct {
		VMID          int      `json:"vmid"`
		Node          string   `json:"node"`
		Name          string   `json:"name"`
		Kind          string   `json:"kind"`
		Status        string   `json:"status"`
		Tags          []string `json:"tags"`
		Template      bool     `json:"template"`
		CPUUsage      float64  `json:"cpu_usage"`
		CPUCores      int      `json:"cpu_cores"`
		MemoryUsed    int64    `json:"memory_used"`
		MemoryTotal   int64    `json:"memory_total"`
		DiskUsed      int64    `json:"disk_used"`
		DiskTotal     int64    `json:"disk_total"`
		UptimeSeconds int64    `json:"uptime_seconds"`
		OSType        string   `json:"os_type,omitempty"`
	}
)

// ParseTags normalizes Proxmox's semicolon-delimited tag representation.
func ParseTags(value string) (tagsResult []string) {
	for tag := range strings.FieldsFuncSeq(value, func(character rune) bool {
		return character == ';' || character == ','
	}) {
		tag = strings.TrimSpace(tag)
		if tag != "" && !slices.Contains(tagsResult, tag) {
			tagsResult = append(tagsResult, tag)
		}
	}
	slices.Sort(tagsResult)
	return
}

// HasTag reports whether a guest contains an exact normalized Proxmox tag.
func HasTag(guest Guest, requiredTag string) (okResult bool) {
	return slices.Contains(guest.Tags, strings.TrimSpace(requiredTag))
}

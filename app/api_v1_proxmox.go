package app

import (
	"context"
	"time"

	"github.com/UNHCSC/organesson/db"
	"github.com/gofiber/fiber/v2"
)

// getProxmoxHealth reports configured connectivity without exposing credentials.
func getProxmoxHealth(c *fiber.Ctx) (errResult error) {
	if !currentUserIsSiteAdmin(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "site administrator access required"})
	}
	if !proxmoxIntegration.enabled || proxmoxIntegration.service == nil {
		return c.JSON(fiber.Map{"enabled": false, "healthy": false, "managed_tag": proxmoxIntegration.managedTag})
	}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(c.UserContext(), 15*time.Second)
	defer cancel()
	if errResult = proxmoxIntegration.service.Health(ctx); errResult != nil {
		return c.JSON(fiber.Map{"enabled": true, "healthy": false, "managed_tag": proxmoxIntegration.managedTag, "error": errResult.Error()})
	}
	return c.JSON(fiber.Map{"enabled": true, "healthy": true, "managed_tag": proxmoxIntegration.managedTag})
}

// getProxmoxInventory returns retained tagged guest inventory without contacting or mutating Proxmox.
func getProxmoxInventory(c *fiber.Ctx) (errResult error) {
	if !currentUserIsSiteAdmin(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "site administrator access required"})
	}
	if !proxmoxIntegration.enabled {
		return c.JSON(fiber.Map{"enabled": false, "managed_tag": proxmoxIntegration.managedTag, "guests": []any{}})
	}
	var guests []*db.ProxmoxInventoryGuest
	if guests, errResult = db.ListProxmoxInventoryGuests(proxmoxIntegration.clusterIdentity); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load Proxmox inventory"})
	}
	return c.JSON(fiber.Map{"enabled": true, "cluster_identity": proxmoxIntegration.clusterIdentity, "managed_tag": proxmoxIntegration.managedTag, "guests": guests})
}

// postProxmoxInventorySync performs a read-only discovery and persists reconciliation state for tagged guests.
func postProxmoxInventorySync(c *fiber.Ctx) (errResult error) {
	if !currentUserIsSiteAdmin(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "site administrator access required"})
	}
	if !proxmoxIntegration.enabled || proxmoxIntegration.service == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Proxmox integration is disabled"})
	}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(c.UserContext(), 45*time.Second)
	defer cancel()
	var result *db.ProxmoxInventorySyncResult
	if result, errResult = db.SyncProxmoxInventory(ctx, proxmoxIntegration.service, proxmoxIntegration.clusterIdentity, proxmoxIntegration.managedTag); errResult != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": errResult.Error()})
	}
	var actorUserID *int
	var user *db.User = currentDBUser(c)
	if user != nil {
		actorUserID = &user.ID
	}
	if _, errResult = db.WriteAudit(db.AuditInput{ActorUserID: actorUserID, Action: "proxmox.inventory.sync", TargetType: "proxmox_cluster", SourceIP: c.IP(), UserAgent: c.Get(fiber.HeaderUserAgent), Metadata: map[string]any{"cluster_identity": result.ClusterIdentity, "managed_tag": result.ManagedTag, "guest_count": len(result.Guests)}}); errResult != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "inventory synchronized but audit recording failed"})
	}
	return c.JSON(result)
}

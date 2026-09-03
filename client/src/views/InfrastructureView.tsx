import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../api";
import { EmptyState, PanelHeading, TextButton } from "../components/common";
import type { ProxmoxHealth, ProxmoxInventory } from "../types";

const byteCount = (value = 0) => {
    if (!value) {
        return "—";
    }
    const units = ["B", "KiB", "MiB", "GiB", "TiB"];
    let amount = value;
    let unit = 0;
    while (amount >= 1024 && unit < units.length - 1) {
        amount /= 1024;
        unit += 1;
    }
    return `${amount.toFixed(amount >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
};

const driftLabel = (value: string) => value.replaceAll("_", " ");

export function InfrastructureView({ showToast }: { showToast: (message: string, kind: "success" | "warning") => void }) {
    const queryClient = useQueryClient();
    const healthQuery = useQuery({ queryKey: ["proxmox", "health"], queryFn: () => apiFetch<ProxmoxHealth>("/api/v1/proxmox/health") });
    const inventoryQuery = useQuery({ queryKey: ["proxmox", "inventory"], queryFn: () => apiFetch<ProxmoxInventory>("/api/v1/proxmox/inventory") });
    const syncMutation = useMutation({
        mutationFn: () => apiFetch<ProxmoxInventory>("/api/v1/proxmox/inventory/sync", { method: "POST" }),
        onSuccess: (inventory) => {
            queryClient.setQueryData(["proxmox", "inventory"], inventory);
            queryClient.invalidateQueries({ queryKey: ["proxmox", "health"] });
            showToast(`Inventory synchronized: ${inventory.guests.length} managed guests`, "success");
        },
        onError: (error) => showToast(error instanceof Error ? error.message : "Proxmox inventory sync failed", "warning")
    });
    const health = healthQuery.data;
    const inventory = inventoryQuery.data;
    const guests = inventory?.guests || [];
    const nodes = inventory?.nodes || [];
    const storages = inventory?.storages || [];
    const networks = inventory?.networks || [];
    const templates = guests.filter((guest) => guest.is_template).length;

    return (
        <section className="dashboard-view is-active infrastructure-view">
            <div className="metric-grid" aria-label="Proxmox inventory summary">
                <article className="metric-card">
                    <span className="panel-label">Connection</span>
                    <strong>{health?.healthy ? "Healthy" : health?.enabled ? "Unavailable" : "Disabled"}</strong>
                </article>
                <article className="metric-card">
                    <span className="panel-label">Managed guests</span>
                    <strong>{guests.length}</strong>
                </article>
                <article className="metric-card">
                    <span className="panel-label">Templates</span>
                    <strong>{templates}</strong>
                </article>
                <article className="metric-card">
                    <span className="panel-label">Required tag</span>
                    <strong className="metric-code">{inventory?.managed_tag || health?.managed_tag || "organesson-managed"}</strong>
                </article>
            </div>

            <article className="dashboard-panel">
                <PanelHeading
                    label="Read-only discovery"
                    title="Managed guest inventory"
                    action={<TextButton onClick={() => syncMutation.mutate()}>{syncMutation.isPending ? "Synchronizing…" : "Synchronize"}</TextButton>}
                />
                {health?.error && <p className="inventory-warning">{health.error}</p>}
                {guests.length === 0 ? (
                    <EmptyState>No guests carrying the required exact tag have been synchronized.</EmptyState>
                ) : (
                    <div className="data-table inventory-table">
                        <div className="data-table-head"><span>Guest</span><span>Location</span><span>Drift</span><span>Status</span></div>
                        {guests.map((guest) => (
                            <div className="data-table-row" key={`${guest.cluster_identity}/${guest.vmid}`}>
                                <span><strong>{guest.name || `VM ${guest.vmid}`}</strong><span>{guest.is_template ? "Template" : guest.kind.toUpperCase()} · VMID {guest.vmid}</span></span>
                                <span><strong>{guest.node}</strong><span>{guest.cluster_identity}</span></span>
                                <span><strong className={`drift-state drift-${guest.drift_state}`}>{driftLabel(guest.drift_state)}</strong><span>{guest.last_error || "Reconciled"}</span></span>
                                <span><strong>{guest.status || "unknown"}</strong><span>{guest.missing_since ? "Missing from latest sync" : "Tag verified"}</span></span>
                            </div>
                        ))}
                    </div>
                )}
            </article>

            <div className="infrastructure-grid">
                <article className="dashboard-panel">
                    <PanelHeading label="Cluster context" title={`Nodes (${nodes.length})`} />
                    {nodes.length === 0 ? <EmptyState>Synchronize to load nodes.</EmptyState> : <div className="compact-list">{nodes.map((node) => <div className="compact-list-row" key={node.name}><strong>{node.name}</strong><span>{node.status} · {node.cpu_total} CPUs · {byteCount(node.memory_total)}</span></div>)}</div>}
                </article>
                <article className="dashboard-panel">
                    <PanelHeading label="Cluster context" title={`Storage (${storages.length})`} />
                    {storages.length === 0 ? <EmptyState>Synchronize to load storage.</EmptyState> : <div className="compact-list">{storages.map((storage, index) => <div className="compact-list-row" key={`${storage.node}/${storage.id}/${index}`}><strong>{storage.id}</strong><span>{storage.node || "shared"} · {byteCount(storage.available)} available</span></div>)}</div>}
                </article>
                <article className="dashboard-panel">
                    <PanelHeading label="Cluster context" title={`Networks (${networks.length})`} />
                    {networks.length === 0 ? <EmptyState>Synchronize to load networks.</EmptyState> : <div className="compact-list">{networks.map((network, index) => <div className="compact-list-row" key={`${network.node}/${network.id}/${index}`}><strong>{network.id}</strong><span>{network.node} · {network.type}{network.cidr ? ` · ${network.cidr}` : ""}</span></div>)}</div>}
                </article>
            </div>
        </section>
    );
}

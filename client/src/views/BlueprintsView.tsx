import { useEffect, useState } from "react";
import { apiFetch } from "../api";
import { EmptyState, PanelHeading, TextButton } from "../components/common";
import type { AllocationPool, Blueprint, BlueprintDocument, Deployment, Project } from "../types";

const starterDocument: BlueprintDocument = {
    format_version: 1,
    opentofu_module: "git::https://example.invalid/infrastructure.git//modules/lab?ref=v1.0.0",
    ansible_project: "git::https://example.invalid/configuration.git?ref=v1.0.0",
    name_pattern: "{{deployment}}-{{resource}}",
    resources: [{ key: "server", kind: "vm", template: "ubuntu-lts-v1", vcpu: 2, memory_mb: 2048, disk_gb: 20, networks: ["lan"], configuration_role: "server" }],
    networks: [{ key: "lan", kind: "isolated", ipv4_cidr: "192.168.1.0/24" }]
};

export function BlueprintsView({ projects, showToast }: { projects: Project[]; showToast: (message: string, kind: "success" | "warning") => void }) {
    const [projectID, setProjectID] = useState<number>(projects[0]?.id || 0);
    const [blueprints, setBlueprints] = useState<Blueprint[]>([]);
    const [pools, setPools] = useState<AllocationPool[]>([]);
    const [deployments, setDeployments] = useState<Deployment[]>([]);
    const [name, setName] = useState("");
    const [slug, setSlug] = useState("");
    const [document, setDocument] = useState(JSON.stringify(starterDocument, null, 2));
    const [groupIDs, setGroupIDs] = useState("");
    const [preview, setPreview] = useState<Record<string, unknown> | null>(null);
    const [previewVersionID, setPreviewVersionID] = useState(0);
    const [poolName, setPoolName] = useState("");
    const [poolKind, setPoolKind] = useState<AllocationPool["kind"]>("vmid");
    const [poolStart, setPoolStart] = useState("100");
    const [poolEnd, setPoolEnd] = useState("999");
    const [poolCIDR, setPoolCIDR] = useState("");

    useEffect(() => {
        if (!projectID && projects.length > 0) setProjectID(projects[0].id);
    }, [projectID, projects]);

    const load = async () => {
        if (!projectID) return;
        try {
            const [nextBlueprints, nextPools, nextDeployments] = await Promise.all([
                apiFetch<Blueprint[]>(`/api/v1/projects/${projectID}/blueprints`),
                apiFetch<AllocationPool[]>(`/api/v1/projects/${projectID}/allocation-pools`),
                apiFetch<Deployment[]>(`/api/v1/projects/${projectID}/deployments`)
            ]);
            setBlueprints(nextBlueprints);
            setPools(nextPools);
            setDeployments(nextDeployments);
        } catch (error) {
            showToast(error instanceof Error ? error.message : "Failed to load blueprints", "warning");
        }
    };

    useEffect(() => { void load(); }, [projectID]);

    const createBlueprint = async () => {
        try {
            await apiFetch(`/api/v1/projects/${projectID}/blueprints`, { method: "POST", body: JSON.stringify({ name, slug, description: "" }) });
            setName(""); setSlug(""); await load(); showToast("Blueprint created", "success");
        } catch (error) { showToast(error instanceof Error ? error.message : "Blueprint creation failed", "warning"); }
    };

    const publish = async (blueprintID: number) => {
        try {
            await apiFetch(`/api/v1/blueprints/${blueprintID}/versions`, { method: "POST", body: JSON.stringify({ document: JSON.parse(document) }) });
            await load(); showToast("Immutable blueprint version published", "success");
        } catch (error) { showToast(error instanceof Error ? error.message : "Blueprint publication failed", "warning"); }
    };

    const createPool = async () => {
        try {
            await apiFetch(`/api/v1/projects/${projectID}/allocation-pools`, { method: "POST", body: JSON.stringify({ name: poolName, kind: poolKind, start: Number(poolStart), end: Number(poolEnd), cidr: poolCIDR }) });
            setPoolName(""); await load(); showToast("Allocation pool created", "success");
        } catch (error) { showToast(error instanceof Error ? error.message : "Allocation pool creation failed", "warning"); }
    };

    const generatePreview = async (versionID: number) => {
        try {
            const ids = groupIDs.split(",").map((value) => Number(value.trim())).filter(Boolean);
            setPreview(await apiFetch<Record<string, unknown>>(`/api/v1/projects/${projectID}/deployment-previews`, { method: "POST", body: JSON.stringify({ blueprintVersionID: versionID, groupIDs: ids, allocationPoolIDs: Object.fromEntries(pools.map((pool) => [pool.kind, pool.id])) }) }));
            setPreviewVersionID(versionID);
        } catch (error) { showToast(error instanceof Error ? error.message : "Preview failed", "warning"); }
    };

    const reservePlan = async () => {
        try {
            const ids = groupIDs.split(",").map((value) => Number(value.trim())).filter(Boolean);
            await apiFetch(`/api/v1/projects/${projectID}/deployments`, { method: "POST", body: JSON.stringify({ blueprintVersionID: previewVersionID, groupIDs: ids, allocationPoolIDs: Object.fromEntries(pools.map((pool) => [pool.kind, pool.id])) }) });
            await load(); setPreview(null); showToast("Deployment plan and allocations reserved", "success");
        } catch (error) { showToast(error instanceof Error ? error.message : "Deployment reservation failed", "warning"); }
    };

    return <section className="dashboard-view is-active">
        <article className="dashboard-panel">
            <PanelHeading label="Desired state" title="Versioned lab blueprints" />
            <p className="panel-copy">Blueprints reference pinned OpenTofu and Ansible sources. Publishing creates an immutable version; previews never change infrastructure.</p>
            <label>Project<select value={projectID} onChange={(event) => setProjectID(Number(event.target.value))}>{projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}</select></label>
            <div className="form-grid"><label>Blueprint name<input value={name} onChange={(event) => setName(event.target.value)} /></label><label>Slug<input value={slug} onChange={(event) => setSlug(event.target.value)} /></label></div>
            <TextButton onClick={createBlueprint}>Create blueprint</TextButton>
        </article>
        <article className="dashboard-panel">
            <PanelHeading label="Collision boundaries" title="Allocation pools" />
            <div className="form-grid">
                <label>Pool name<input value={poolName} onChange={(event) => setPoolName(event.target.value)} /></label>
                <label>Kind<select value={poolKind} onChange={(event) => setPoolKind(event.target.value as AllocationPool["kind"])}>{["vmid", "vlan", "vxlan", "external_port", "ipv4", "ipv6"].map((kind) => <option key={kind}>{kind}</option>)}</select></label>
                <label>Start offset/value<input type="number" value={poolStart} onChange={(event) => setPoolStart(event.target.value)} /></label>
                <label>End offset/value<input type="number" value={poolEnd} onChange={(event) => setPoolEnd(event.target.value)} /></label>
                {(poolKind === "ipv4" || poolKind === "ipv6") && <label>CIDR<input value={poolCIDR} onChange={(event) => setPoolCIDR(event.target.value)} /></label>}
            </div>
            <TextButton onClick={createPool}>Create allocation pool</TextButton>
            <div className="compact-list">{pools.map((pool) => <div className="compact-list-row" key={pool.id}><strong>{pool.name}</strong><span>{pool.kind} · {pool.available} available</span></div>)}</div>
        </article>
        <article className="dashboard-panel">
            <PanelHeading label="Runner contract" title="Blueprint document" />
            <textarea className="blueprint-document" rows={18} value={document} onChange={(event) => setDocument(event.target.value)} spellCheck={false} />
            <label>Preview group IDs, comma-separated<input value={groupIDs} onChange={(event) => setGroupIDs(event.target.value)} placeholder="12, 13, 14" /></label>
        </article>
        {blueprints.length === 0 ? <EmptyState>No blueprints exist for this project.</EmptyState> : blueprints.map((blueprint) => <article className="dashboard-panel" key={blueprint.id}>
            <PanelHeading label={blueprint.slug} title={blueprint.name} action={<TextButton onClick={() => publish(blueprint.id)}>Publish current document</TextButton>} />
            {blueprint.versions.map((version) => <div className="compact-list-row" key={version.id}><span><strong>Version {version.version}</strong><span>{version.document_digest}</span></span><TextButton onClick={() => generatePreview(version.id)}>Preview</TextButton></div>)}
        </article>)}
        <article className="dashboard-panel">
            <PanelHeading label="Reserved desired state" title={`Deployment plans (${deployments.length})`} />
            {deployments.length === 0 ? <EmptyState>No deployment plans have been reserved.</EmptyState> : <div className="compact-list">{deployments.map((deployment) => <div className="compact-list-row" key={deployment.id}><strong>{deployment.name}</strong><span>{deployment.status} · group {deployment.group_id} · blueprint version {deployment.blueprint_version_id}</span></div>)}</div>}
        </article>
        {preview && <article className="dashboard-panel"><PanelHeading label="No infrastructure changes" title="Deployment preview" action={<TextButton onClick={reservePlan}>Reserve deployment plan</TextButton>} /><pre className="blueprint-preview">{JSON.stringify(preview, null, 2)}</pre></article>}
    </section>;
}

import { lazy, Suspense, useEffect, useState } from "react";
import { apiFetch, requestKey } from "../api";
import { EmptyState, PanelHeading, TextButton } from "../components/common";
import type { AllocationPool, Blueprint, BlueprintDocument, Deployment, Project, RunnerRun } from "../types";

const CodeEditor = lazy(async () => ({ default: (await import("../components/CodeEditor")).CodeEditor }));
const runnerStatusNames = ["queued", "running", "succeeded", "failed", "cancelled"];

const starterDocument: BlueprintDocument = {
    format_version: 1,
    opentofu_module: "https://example.invalid/infrastructure.git?ref=0000000000000000000000000000000000000000",
    ansible_project: "https://example.invalid/configuration.git?ref=0000000000000000000000000000000000000000",
    name_pattern: "{{deployment}}-{{resource}}",
    resources: [{ key: "server", kind: "vm", template: "ubuntu-lts-v1", vcpu: 2, memory_mb: 2048, disk_gb: 20, networks: ["lan"], configuration_role: "server" }],
    networks: [{ key: "lan", kind: "isolated", ipv4_cidr: "192.168.1.0/24" }]
};

export function BlueprintsView({ projects, showToast }: { projects: Project[]; showToast: (message: string, kind: "success" | "warning") => void }) {
    const [projectID, setProjectID] = useState<number>(projects[0]?.id || 0);
    const [blueprints, setBlueprints] = useState<Blueprint[]>([]);
    const [pools, setPools] = useState<AllocationPool[]>([]);
    const [deployments, setDeployments] = useState<Deployment[]>([]);
    const [runs, setRuns] = useState<Record<number, RunnerRun[]>>({});
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

    const loadRuns = async (items: Deployment[]) => {
        const deploymentRuns = await Promise.all(items.map(async (deployment) => [deployment.id, (await apiFetch<RunnerRun[]>(`/api/v1/deployments/${deployment.id}/runs`)) ?? []] as const));
        setRuns(Object.fromEntries(deploymentRuns));
    };

    const load = async () => {
        if (!projectID) return;
        try {
            const [nextBlueprints, nextPools, nextDeployments] = await Promise.all([
                apiFetch<Blueprint[]>(`/api/v1/projects/${projectID}/blueprints`),
                apiFetch<AllocationPool[]>(`/api/v1/projects/${projectID}/allocation-pools`),
                apiFetch<Deployment[]>(`/api/v1/projects/${projectID}/deployments`)
            ]);
            setBlueprints(nextBlueprints ?? []);
            setPools(nextPools ?? []);
            setDeployments(nextDeployments ?? []);
            await loadRuns(nextDeployments ?? []);
        } catch (error) {
            showToast(error instanceof Error ? error.message : "Failed to load blueprints", "warning");
        }
    };

    useEffect(() => { void load(); }, [projectID]);

    useEffect(() => {
        if (deployments.length === 0) return;
        const interval = window.setInterval(() => { void loadRuns(deployments).catch(() => undefined); }, 2000);
        return () => window.clearInterval(interval);
    }, [deployments]);

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

    const runDeployment = async (deployment: Deployment, action: "tofu.plan" | "tofu.apply" | "tofu.destroy" | "ansible.check") => {
        if (action === "tofu.apply" && !window.confirm(`Apply the latest successful plan to ${deployment.name}?`)) return;
        if (action === "tofu.destroy" && !window.confirm(`Destroy infrastructure for ${deployment.name}? This requires a separately confirmed operation.`)) return;
        const confirm = action === "tofu.apply" || action === "tofu.destroy";
        try {
            await apiFetch(`/api/v1/deployments/${deployment.id}/runs`, { method: "POST", headers: { "Idempotency-Key": requestKey() }, body: JSON.stringify({ action, confirm }) });
            showToast(`${action} queued for ${deployment.name}`, "success");
            window.setTimeout(() => { void load(); }, 500);
        } catch (error) { showToast(error instanceof Error ? error.message : "Runner action failed", "warning"); }
    };

    const latestRunLabel = (deploymentID: number) => {
        const deploymentRuns = runs[deploymentID] || [];
        const latest = deploymentRuns.reduce<RunnerRun | null>((result, run) => !result || run.id > result.id ? run : result, null);
        if (!latest) return "No runner activity";
        let summary = "";
        if (latest.summary_json) {
            try {
                const parsed = JSON.parse(latest.summary_json) as { plan?: { add?: number; change?: number; destroy?: number } };
                if (parsed.plan) summary = ` · +${parsed.plan.add || 0} ~${parsed.plan.change || 0} −${parsed.plan.destroy || 0}`;
            } catch {
                summary = "";
            }
        }
        const status = runnerStatusNames[latest.job_status] || (latest.finished_at ? "finished" : "running");
        return `${latest.action} · ${status}${summary} · run #${latest.id}${latest.error_summary ? ` · ${latest.error_summary}` : ""}`;
    };

    return <section className="dashboard-view is-active">
        <article className="dashboard-panel">
            <PanelHeading label="Desired state" title="Versioned lab blueprints" />
            <div className="blueprint-panel-body">
                <p className="panel-copy">Blueprints reference pinned OpenTofu and Ansible sources. Publishing creates an immutable version; previews never change infrastructure.</p>
                <div className="blueprint-form-grid">
                    <label className="field-group blueprint-project-field"><span className="field-label">Project</span><select className="field-input" value={projectID} onChange={(event) => setProjectID(Number(event.target.value))}>{projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}</select></label>
                    <label className="field-group"><span className="field-label">Blueprint name</span><input className="field-input" value={name} onChange={(event) => setName(event.target.value)} /></label>
                    <label className="field-group"><span className="field-label">Slug</span><input className="field-input" value={slug} onChange={(event) => setSlug(event.target.value)} /></label>
                </div>
                <div className="blueprint-form-actions"><TextButton onClick={createBlueprint}>Create blueprint</TextButton></div>
            </div>
        </article>
        <article className="dashboard-panel">
            <PanelHeading label="Collision boundaries" title="Allocation pools" />
            <div className="blueprint-panel-body">
                <div className="blueprint-pool-grid">
                    <label className="field-group"><span className="field-label">Pool name</span><input className="field-input" value={poolName} onChange={(event) => setPoolName(event.target.value)} /></label>
                    <label className="field-group"><span className="field-label">Kind</span><select className="field-input" value={poolKind} onChange={(event) => setPoolKind(event.target.value as AllocationPool["kind"])}>{["vmid", "vlan", "vxlan", "external_port", "ipv4", "ipv6"].map((kind) => <option key={kind}>{kind}</option>)}</select></label>
                    <label className="field-group"><span className="field-label">Start offset/value</span><input className="field-input" type="number" value={poolStart} onChange={(event) => setPoolStart(event.target.value)} /></label>
                    <label className="field-group"><span className="field-label">End offset/value</span><input className="field-input" type="number" value={poolEnd} onChange={(event) => setPoolEnd(event.target.value)} /></label>
                    {(poolKind === "ipv4" || poolKind === "ipv6") && <label className="field-group"><span className="field-label">CIDR</span><input className="field-input" value={poolCIDR} onChange={(event) => setPoolCIDR(event.target.value)} /></label>}
                </div>
                <div className="blueprint-form-actions"><TextButton onClick={createPool}>Create allocation pool</TextButton></div>
            </div>
            <div className="compact-list">{pools.map((pool) => <div className="compact-list-row" key={pool.id}><strong>{pool.name}</strong><span>{pool.kind} · {pool.available} available</span></div>)}</div>
        </article>
        <article className="dashboard-panel">
            <PanelHeading label="Runner contract" title="Blueprint document" />
            <div className="blueprint-panel-body">
                <label className="field-group"><span className="field-label">Version document (JSON)</span><div className="blueprint-document"><Suspense fallback={<div className="code-editor-loading">Loading editor…</div>}><CodeEditor ariaLabel="Version document (JSON)" language="json" value={document} onChange={setDocument} /></Suspense></div></label>
                <label className="field-group blueprint-group-field"><span className="field-label">Preview group IDs</span><input className="field-input" aria-label="Preview group IDs, comma-separated" value={groupIDs} onChange={(event) => setGroupIDs(event.target.value)} placeholder="12, 13, 14" /><span className="field-help">Enter one or more numeric group IDs, separated by commas.</span></label>
            </div>
        </article>
        {blueprints.length === 0 ? <EmptyState>No blueprints exist for this project.</EmptyState> : blueprints.map((blueprint) => <article className="dashboard-panel" key={blueprint.id}>
            <PanelHeading label={blueprint.slug} title={blueprint.name} action={<TextButton onClick={() => publish(blueprint.id)}>Publish current document</TextButton>} />
            {blueprint.versions.map((version) => <div className="compact-list-row" key={version.id}><span><strong>Version {version.version}</strong><span>{version.document_digest}</span></span><TextButton onClick={() => generatePreview(version.id)}>Preview</TextButton></div>)}
        </article>)}
        <article className="dashboard-panel">
            <PanelHeading label="Reserved desired state" title={`Deployment plans (${deployments.length})`} />
            {deployments.length === 0 ? <EmptyState>No deployment plans have been reserved.</EmptyState> : <div className="compact-list">{deployments.map((deployment) => <div className="compact-list-row runner-deployment-row" key={deployment.id}><span><strong>{deployment.name}</strong><span>{deployment.status} · group {deployment.group_id} · blueprint version {deployment.blueprint_version_id}</span><span>{latestRunLabel(deployment.id)}</span></span><span className="inline-actions"><TextButton onClick={() => runDeployment(deployment, "tofu.plan")}>Plan</TextButton><TextButton onClick={() => runDeployment(deployment, "tofu.apply")}>Apply…</TextButton><TextButton onClick={() => runDeployment(deployment, "ansible.check")}>Ansible check</TextButton><button type="button" className="button-secondary compact-button danger-button" onClick={() => runDeployment(deployment, "tofu.destroy")}>Destroy…</button></span></div>)}</div>}
        </article>
        {preview && <article className="dashboard-panel"><PanelHeading label="No infrastructure changes" title="Deployment preview" action={<TextButton onClick={reservePlan}>Reserve deployment plan</TextButton>} /><div className="blueprint-preview"><Suspense fallback={<div className="code-editor-loading">Loading preview…</div>}><CodeEditor ariaLabel="Deployment preview JSON" language="json" value={JSON.stringify(preview, null, 2)} readOnly /></Suspense></div></article>}
    </section>;
}

import { useEffect, useState } from "react";
import { apiFetch } from "../api";
import { EmptyState, PanelHeading, TextButton } from "../components/common";
import type { Blueprint, BlueprintDocument, Project } from "../types";

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
    const [name, setName] = useState("");
    const [slug, setSlug] = useState("");
    const [document, setDocument] = useState(JSON.stringify(starterDocument, null, 2));
    const [groupIDs, setGroupIDs] = useState("");
    const [preview, setPreview] = useState<Record<string, unknown> | null>(null);

    useEffect(() => {
        if (!projectID && projects.length > 0) setProjectID(projects[0].id);
    }, [projectID, projects]);

    const load = async () => {
        if (!projectID) return;
        try {
            setBlueprints(await apiFetch<Blueprint[]>(`/api/v1/projects/${projectID}/blueprints`));
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

    const generatePreview = async (versionID: number) => {
        try {
            const ids = groupIDs.split(",").map((value) => Number(value.trim())).filter(Boolean);
            setPreview(await apiFetch<Record<string, unknown>>(`/api/v1/projects/${projectID}/deployment-previews`, { method: "POST", body: JSON.stringify({ blueprintVersionID: versionID, groupIDs: ids }) }));
        } catch (error) { showToast(error instanceof Error ? error.message : "Preview failed", "warning"); }
    };

    return <section className="dashboard-view is-active">
        <article className="dashboard-panel">
            <PanelHeading label="Desired state" title="Versioned lab blueprints" />
            <p className="panel-copy">Blueprints reference pinned OpenTofu and Ansible sources. Publishing creates an immutable version; previews never change infrastructure.</p>
            <label>Project<select value={projectID} onChange={(event) => setProjectID(Number(event.target.value))}>{projects.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}</select></label>
            <div className="form-grid"><label>Name<input value={name} onChange={(event) => setName(event.target.value)} /></label><label>Slug<input value={slug} onChange={(event) => setSlug(event.target.value)} /></label></div>
            <TextButton onClick={createBlueprint}>Create blueprint</TextButton>
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
        {preview && <article className="dashboard-panel"><PanelHeading label="No changes made" title="Deployment preview" /><pre className="blueprint-preview">{JSON.stringify(preview, null, 2)}</pre></article>}
    </section>;
}

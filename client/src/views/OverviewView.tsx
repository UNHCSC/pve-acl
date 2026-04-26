import { EmptyState, PanelHeading, TextButton } from "../components/common";
import type { ProjectTree, Selection, Summary, ViewKey } from "../types";
import { formatCount } from "../ui-helpers";

export function OverviewView({
    counts,
    tree,
    capabilities,
    setView,
    selectProject
}: {
    counts: Record<string, number>;
    tree: ProjectTree | null;
    capabilities: Summary["capabilities"];
    setView: (view: ViewKey) => void;
    selectProject: (selection: Selection) => void;
}) {
    const recentProjects = (tree?.projects || []).slice(0, 5);
    const metrics = [
        ["Organizations", "organizations"],
        ["Projects", "projects"],
        ...(capabilities.canViewUsers ? [["Users", "users"]] : []),
        ...(counts.auditEvents ? [["Audit events", "auditEvents"]] : [])
    ];

    return (
        <section className="dashboard-view is-active">
            <div className="metric-grid" aria-label="Cloud summary">
                {metrics.map(([label, key]) => (
                    <article className="metric-card" key={key}>
                        <span className="panel-label">{label}</span>
                        <strong>{formatCount(counts[key])}</strong>
                    </article>
                ))}
            </div>

            <section className="dashboard-grid">
                <article className="dashboard-panel">
                    <PanelHeading label="Directory" title="Recent projects" action={<TextButton onClick={() => setView("directory")}>Open</TextButton>} />
                    <div className="recent-list">
                        {recentProjects.length === 0 && <EmptyState>No projects are visible yet.</EmptyState>}
                        {recentProjects.map((project) => (
                            <button
                                key={project.id}
                                type="button"
                                className="recent-row"
                                onClick={() => {
                                    selectProject({ type: "project", id: project.id, slug: project.slug });
                                    setView("directory");
                                }}
                            >
                                <span>
                                    <strong>{project.name}</strong>
                                    <span>
                                        {project.organization?.name || "Organization"} / {project.slug}
                                    </span>
                                </span>
                                <time>{project.is_active === false ? "inactive" : "active"}</time>
                            </button>
                        ))}
                    </div>
                </article>

            </section>
        </section>
    );
}

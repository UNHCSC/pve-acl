import { EmptyState, PanelHeading, TextButton } from "../components/common";
import type { AccessData, ProjectTree, Selection, ViewKey } from "../types";
import { formatCount } from "../ui-helpers";

export function OverviewView({
    counts,
    tree,
    access,
    setView,
    selectProject
}: {
    counts: Record<string, number>;
    tree: ProjectTree | null;
    access: AccessData;
    setView: (view: ViewKey) => void;
    selectProject: (selection: Selection) => void;
}) {
    const recentProjects = (tree?.projects || []).slice(0, 5);

    return (
        <section className="dashboard-view is-active">
            <div className="metric-grid" aria-label="Cloud summary">
                {[
                    ["Organizations", "organizations"],
                    ["Projects", "projects"],
                    ["Users", "users"],
                    ["Audit events", "auditEvents"]
                ].map(([label, key]) => (
                    <article className="metric-card" key={key}>
                        <span className="panel-label">{label}</span>
                        <strong>{formatCount(counts[key])}</strong>
                    </article>
                ))}
            </div>

            <section className="dashboard-grid">
                <article className="dashboard-panel">
                    <PanelHeading label="System data" title="Local tables" />
                    <div className="table-count-grid">
                        {[
                            ["Groups", "groups"],
                            ["Roles", "roles"],
                            ["Permissions", "permissions"],
                            ["Role bindings", "roleBindings"]
                        ].map(([label, key]) => (
                            <div key={key}>
                                <span>{label}</span>
                                <strong>{formatCount(counts[key])}</strong>
                            </div>
                        ))}
                    </div>
                </article>

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

                <article className="dashboard-panel wide-panel">
                    <PanelHeading label="Access" title="Permissions available" />
                    <div className="permission-cloud">
                        {access.permissions.slice(0, 24).map((permission) => (
                            <span className="permission-pill" key={permission.id}>
                                {permission.name}
                            </span>
                        ))}
                        {access.permissions.length === 0 && <span className="permission-pill">No permissions loaded</span>}
                    </div>
                </article>
            </section>
        </section>
    );
}

import { ReactNode } from "react";
import { Detail, EmptyDetail, EmptyState, PanelHeading } from "../components/common";
import type { ModalKey, OrgNode, Organization, Project, ProjectMembership, ProjectTree, Selection } from "../types";
import { classNames, roleLabel, subjectTypeLabel } from "../ui-helpers";

type DirectoryViewProps = {
    orgTree: OrgNode[];
    tree: ProjectTree | null;
    expanded: Set<number>;
    selection: Selection;
    selectedOrg: OrgNode | null;
    selectedProject: Project | null;
    memberships: ProjectMembership[];
    loadingProject: boolean;
    openMenu: string | null;
    setOpenMenu: (key: string | null) => void;
    toggleOrg: (id: number) => void;
    selectOrg: (org: Organization) => void;
    selectProject: (project: Project) => void;
    openModal: (modal: ModalKey, context?: Organization | Project | null) => void;
    moveOrg: (org: Organization) => void;
    moveProject: (project: Project) => void;
    deleteOrg: (org: Organization) => void;
    deleteProject: (project: Project) => void;
    addProjectMember: () => void;
    updateProjectMember: (membership: ProjectMembership, role: string) => void;
    deleteProjectMember: (membership: ProjectMembership) => void;
};

export function DirectoryView(props: DirectoryViewProps) {
    return (
        <section className="dashboard-view is-active">
            <div className="project-browser">
                <aside className="project-tree-sidebar">
                    <div className="project-tree-heading">
                        <div>
                            <span className="panel-label">Organization tree</span>
                            <h2>Directory</h2>
                        </div>
                        <span className="tree-count-pill">{props.tree?.projects.length || 0} projects</span>
                    </div>
                    <div className="project-tree" aria-label="Organization and project tree">
                        {props.orgTree.length === 0 && <EmptyState>No organizations are visible yet.</EmptyState>}
                        {props.orgTree.map((node) => (
                            <TreeNode key={node.id} node={node} depth={0} {...props} />
                        ))}
                    </div>
                </aside>

                <div className="project-render-pane">
                    {props.selection?.type === "org" && props.selectedOrg && <OrgDetail org={props.selectedOrg} />}
                    {props.selection?.type === "project" && props.selectedProject && (
                        <ProjectDetail
                            project={props.selectedProject}
                            memberships={props.memberships}
                            loading={props.loadingProject}
                            addProjectMember={props.addProjectMember}
                            updateProjectMember={props.updateProjectMember}
                            deleteProjectMember={props.deleteProjectMember}
                        />
                    )}
                    {!props.selection && <EmptyDetail title="Select an organization or project" text="The directory tree renders the selected item here." />}
                </div>
            </div>
        </section>
    );
}

function TreeNode(props: DirectoryViewProps & { node: OrgNode; depth: number }) {
    const { node, depth } = props;
    const { node: _node, depth: _depth, ...directoryProps } = props;
    const expanded = props.expanded.has(node.id);
    const selected = props.selection?.type === "org" && props.selection.id === node.id;
    const menuKey = `org-${node.id}`;

    return (
        <>
            <div className={classNames("tree-row", selected && "is-selected")} style={{ "--tree-depth": depth } as React.CSSProperties}>
                <button type="button" className="tree-toggle" disabled={node.children.length + node.projects.length === 0} onClick={() => props.toggleOrg(node.id)} aria-label={expanded ? "Collapse organization" : "Expand organization"}>
                    {expanded ? "−" : "+"}
                </button>
                <button type="button" className="tree-label" onClick={() => props.selectOrg(node)} title={node.name}>
                    <span className="tree-kind-badge">O</span>
                    <span>{node.name}</span>
                </button>
                <TreeActions open={props.openMenu === menuKey} setOpen={(open) => props.setOpenMenu(open ? menuKey : null)}>
                    <button type="button" onClick={() => props.openModal("org", node)}>New organization</button>
                    <button type="button" onClick={() => props.openModal("project", node)}>New project</button>
                    {node.parent_org_id !== null && <button type="button" onClick={() => props.moveOrg(node)}>Move</button>}
                    {node.parent_org_id !== null && <button type="button" className="danger-action" onClick={() => props.deleteOrg(node)}>Delete</button>}
                </TreeActions>
            </div>
            {expanded && (
                <>
                    {node.children.map((child) => (
                        <TreeNode key={child.id} {...directoryProps} node={child} depth={depth + 1} />
                    ))}
                    {node.projects.map((project) => (
                        <ProjectTreeRow key={project.id} {...directoryProps} project={project} depth={depth + 1} />
                    ))}
                </>
            )}
        </>
    );
}

function ProjectTreeRow(props: DirectoryViewProps & { project: Project; depth: number }) {
    const { project, depth } = props;
    const selected = props.selection?.type === "project" && props.selection.id === project.id;
    const menuKey = `project-${project.id}`;

    return (
        <div className={classNames("tree-row tree-project-row", selected && "is-selected")} style={{ "--tree-depth": depth } as React.CSSProperties}>
            <span className="tree-toggle" aria-hidden="true" />
            <button type="button" className="tree-label" onClick={() => props.selectProject(project)} title={project.name}>
                <span className="tree-kind-badge project-kind-badge">P</span>
                <span>{project.name}</span>
            </button>
            <TreeActions open={props.openMenu === menuKey} setOpen={(open) => props.setOpenMenu(open ? menuKey : null)}>
                <button type="button" onClick={() => props.moveProject(project)}>Move</button>
                <button type="button" className="danger-action" onClick={() => props.deleteProject(project)}>Delete</button>
            </TreeActions>
        </div>
    );
}

function TreeActions({ children, open, setOpen }: { children: ReactNode; open: boolean; setOpen: (open: boolean) => void }) {
    return (
        <div className="tree-actions">
            <button
                type="button"
                className="icon-button tree-menu-button"
                aria-label="More actions"
                onClick={(event) => {
                    event.stopPropagation();
                    setOpen(!open);
                }}
            >
                ...
            </button>
            {open && <div className="row-menu tree-inline-menu">{children}</div>}
        </div>
    );
}

function OrgDetail({ org }: { org: OrgNode }) {
    return (
        <article className="project-detail-page">
            <header className="project-detail-header">
                <div>
                    <span className="panel-label">Organization</span>
                    <h2>{org.name}</h2>
                    <p>{org.description || "No organization description set."}</p>
                </div>
                <span className="detail-status-pill">org</span>
            </header>
            <div className="project-detail-content">
                <section className="dashboard-panel">
                    <PanelHeading label="Tree position" title="Contents" />
                    <dl className="detail-list expanded-detail-list">
                        <Detail label="Slug">{org.slug}</Detail>
                        <Detail label="Child orgs">{org.children.length}</Detail>
                        <Detail label="Projects">{org.projects.length}</Detail>
                    </dl>
                </section>
            </div>
        </article>
    );
}

function ProjectDetail(props: {
    project: Project;
    memberships: ProjectMembership[];
    loading: boolean;
    addProjectMember: () => void;
    updateProjectMember: (membership: ProjectMembership, role: string) => void;
    deleteProjectMember: (membership: ProjectMembership) => void;
}) {
    return (
        <article className="project-detail-page">
            <header className="project-detail-header">
                <div>
                    <span className="panel-label">Project</span>
                    <h2>{props.project.name}</h2>
                    <p>{props.project.description || "No project description set."}</p>
                </div>
                <div className="project-detail-actions">
                    <span className="detail-status-pill">{props.project.is_active === false ? "inactive" : "active"}</span>
                    <button className="button-primary compact-button" type="button" onClick={props.addProjectMember}>
                        Add member
                    </button>
                </div>
            </header>
            <div className="project-detail-content">
                <section className="dashboard-panel">
                    <PanelHeading label="Project data" title="Details" />
                    <dl className="detail-list expanded-detail-list">
                        <Detail label="Slug">{props.project.slug}</Detail>
                        <Detail label="Organization ID">{props.project.organization_id}</Detail>
                        <Detail label="Direct members">{props.memberships.length}</Detail>
                    </dl>
                </section>
                <section className="dashboard-panel project-members-panel">
                    <PanelHeading label="Access" title="Project members" />
                    {props.loading && <EmptyState>Loading project members...</EmptyState>}
                    {!props.loading && props.memberships.length === 0 && <EmptyState>No direct project members.</EmptyState>}
                    {!props.loading && props.memberships.length > 0 && (
                        <div className="compact-list">
                            {props.memberships.map((membership) => (
                                <div className="compact-list-row action-list-row" key={membership.id}>
                                    <div>
                                        <strong>{membership.subject?.label || membership.subject?.name || membership.subject?.username || `Subject ${membership.subject_id}`}</strong>
                                        <span>{subjectTypeLabel(membership.subject_type)} / {membership.subject?.meta || roleLabel(membership.project_role)}</span>
                                    </div>
                                    <div className="project-access-actions">
                                        <select className="field-input compact-select" value={roleLabel(membership.project_role)} onChange={(event) => props.updateProjectMember(membership, event.target.value)}>
                                            {["viewer", "operator", "developer", "manager", "owner"].map((role) => (
                                                <option key={role} value={role}>
                                                    {role}
                                                </option>
                                            ))}
                                        </select>
                                        <button type="button" className="button-secondary compact-button danger-button" onClick={() => props.deleteProjectMember(membership)}>
                                            Remove
                                        </button>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </section>
            </div>
        </article>
    );
}

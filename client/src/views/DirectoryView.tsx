import { Children, useEffect, useRef, useState, type CSSProperties, type DragEvent as ReactDragEvent, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Detail, EmptyDetail, EmptyState, PanelHeading } from "../components/common";
import type { ModalKey, OrgNode, Organization, Project, ProjectMembership, Selection } from "../types";
import { findOrg, orgContains } from "../tree";
import { classNames, subjectTypeLabel } from "../ui-helpers";

const MIN_SIDEBAR_WIDTH = 232;
const MAX_SIDEBAR_WIDTH = 520;

type DraggedItem =
    | { type: "org"; org: Organization }
    | { type: "project"; project: Project }
    | null;

type ProjectMemberSubject = "user" | "group";

type DirectoryViewProps = {
    orgTree: OrgNode[];
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
    moveOrg: (org: Organization, parentOrgID: number | null) => Promise<void> | void;
    moveProject: (project: Project, organizationID: number) => Promise<void> | void;
    deleteOrg: (org: Organization) => void;
    deleteProject: (project: Project) => void;
    addProjectMember: (subjectType: ProjectMemberSubject) => void;
    deleteProjectMember: (membership: ProjectMembership) => void;
};

type TreeRuntime = {
    canDropOnOrg: (target: OrgNode) => boolean;
    canDropOnRoot: () => boolean;
    clearDrag: () => void;
    clearDropTarget: () => void;
    dragState: DraggedItem;
    dropOnOrg: (event: ReactDragEvent, target: OrgNode) => void;
    dropOnRoot: (event: ReactDragEvent) => void;
    dropTarget: string | null;
    markOrgDropTarget: (event: ReactDragEvent, target: OrgNode) => void;
    markRootDropTarget: (event: ReactDragEvent) => void;
    startOrgDrag: (event: ReactDragEvent, org: Organization) => void;
    startProjectDrag: (event: ReactDragEvent, project: Project) => void;
};

export function DirectoryView(props: DirectoryViewProps) {
    const browserRef = useRef<HTMLDivElement>(null);
    const [dragState, setDragState] = useState<DraggedItem>(null);
    const [dropTarget, setDropTarget] = useState<string | null>(null);
    const [resizing, setResizing] = useState(false);
    const [sidebarWidth, setSidebarWidth] = useState(292);

    useEffect(() => {
        if (!resizing) {
            return;
        }

        const resize = (event: PointerEvent) => {
            const bounds = browserRef.current?.getBoundingClientRect();
            if (!bounds) {
                return;
            }
            const maxWidth = Math.min(MAX_SIDEBAR_WIDTH, Math.max(MIN_SIDEBAR_WIDTH, window.innerWidth - bounds.left - 360));
            const nextWidth = Math.min(maxWidth, Math.max(MIN_SIDEBAR_WIDTH, event.clientX - bounds.left));
            setSidebarWidth(nextWidth);
        };
        const stop = () => setResizing(false);

        window.addEventListener("pointermove", resize);
        window.addEventListener("pointerup", stop);
        document.body.classList.add("is-resizing-directory");
        return () => {
            window.removeEventListener("pointermove", resize);
            window.removeEventListener("pointerup", stop);
            document.body.classList.remove("is-resizing-directory");
        };
    }, [resizing]);

    const canDropOnOrg = (target: OrgNode) => {
        if (!dragState) {
            return false;
        }
        if (dragState.type === "project") {
            return dragState.project.organization_id !== target.id;
        }

        if (dragState.org.id === target.id || dragState.org.parent_org_id === target.id) {
            return false;
        }
        const draggedNode = findOrg(props.orgTree, dragState.org.id);
        return draggedNode ? !orgContains(draggedNode, target.id) : false;
    };

    const canDropOnRoot = () => dragState?.type === "org" && dragState.org.parent_org_id !== null;

    const clearDrag = () => {
        setDragState(null);
        setDropTarget(null);
    };

    const startOrgDrag = (event: ReactDragEvent, org: Organization) => {
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData("text/plain", `org:${org.id}`);
        setDragState({ type: "org", org });
        setDropTarget(null);
        props.setOpenMenu(null);
    };

    const startProjectDrag = (event: ReactDragEvent, project: Project) => {
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData("text/plain", `project:${project.id}`);
        setDragState({ type: "project", project });
        setDropTarget(null);
        props.setOpenMenu(null);
    };

    const markOrgDropTarget = (event: ReactDragEvent, target: OrgNode) => {
        if (!canDropOnOrg(target)) {
            event.dataTransfer.dropEffect = "none";
            return;
        }
        event.preventDefault();
        event.dataTransfer.dropEffect = "move";
        setDropTarget(`org-${target.id}`);
    };

    const markRootDropTarget = (event: ReactDragEvent) => {
        if (!canDropOnRoot()) {
            event.dataTransfer.dropEffect = "none";
            return;
        }
        event.preventDefault();
        event.dataTransfer.dropEffect = "move";
        setDropTarget("root");
    };

    const dropOnOrg = (event: ReactDragEvent, target: OrgNode) => {
        event.preventDefault();
        if (!canDropOnOrg(target) || !dragState) {
            clearDrag();
            return;
        }
        const dropped = dragState;
        clearDrag();
        if (dropped.type === "org") {
            void props.moveOrg(dropped.org, target.id);
        } else {
            void props.moveProject(dropped.project, target.id);
        }
    };

    const dropOnRoot = (event: ReactDragEvent) => {
        event.preventDefault();
        if (!canDropOnRoot() || dragState?.type !== "org") {
            clearDrag();
            return;
        }
        const org = dragState.org;
        clearDrag();
        void props.moveOrg(org, null);
    };

    const runtime: TreeRuntime = {
        canDropOnOrg,
        canDropOnRoot,
        clearDrag,
        clearDropTarget: () => setDropTarget(null),
        dragState,
        dropOnOrg,
        dropOnRoot,
        dropTarget,
        markOrgDropTarget,
        markRootDropTarget,
        startOrgDrag,
        startProjectDrag
    };

    return (
        <section className="dashboard-view is-active">
            <div
                className={classNames("project-browser", resizing && "is-resizing")}
                ref={browserRef}
                style={{ "--directory-sidebar-width": `${sidebarWidth}px` } as CSSProperties}
            >
                <aside className="project-tree-sidebar">
                    <div className="project-tree" aria-label="Organization and project tree">
                        {props.orgTree.length === 0 && <EmptyState>No organizations are visible yet.</EmptyState>}
                        {dragState?.type === "org" && (
                            <div
                                className={classNames("tree-root-drop", dropTarget === "root" && "is-drop-target", canDropOnRoot() && "can-drop")}
                                onDragLeave={() => setDropTarget(null)}
                                onDragOver={runtime.markRootDropTarget}
                                onDrop={runtime.dropOnRoot}
                            >
                                Root level
                            </div>
                        )}
                        {props.orgTree.map((node) => (
                            <TreeNode key={node.id} node={node} depth={0} runtime={runtime} {...props} />
                        ))}
                    </div>
                </aside>

                <button
                    type="button"
                    className="project-sidebar-resizer"
                    aria-label="Resize directory sidebar"
                    onPointerDown={(event: ReactPointerEvent<HTMLButtonElement>) => {
                        event.preventDefault();
                        setResizing(true);
                    }}
                />

                <div className="project-render-pane">
                    {props.selection?.type === "org" && props.selectedOrg && <OrgDetail org={props.selectedOrg} />}
                    {props.selection?.type === "project" && props.selectedProject && (
                        <ProjectDetail
                            project={props.selectedProject}
                            memberships={props.memberships}
                            loading={props.loadingProject}
                            addProjectMember={props.addProjectMember}
                            deleteProjectMember={props.deleteProjectMember}
                        />
                    )}
                    {!props.selection && <EmptyDetail title="Select an organization or project" text="The directory tree renders the selected item here." />}
                </div>
            </div>
        </section>
    );
}

function TreeNode(props: DirectoryViewProps & { node: OrgNode; depth: number; runtime: TreeRuntime }) {
    const { node, depth, runtime } = props;
    const { node: _node, depth: _depth, runtime: _runtime, ...directoryProps } = props;
    const expanded = props.expanded.has(node.id);
    const selected = props.selection?.type === "org" && props.selection.id === node.id;
    const menuKey = `org-${node.id}`;
    const isDragged = runtime.dragState?.type === "org" && runtime.dragState.org.id === node.id;
    const canDrop = runtime.canDropOnOrg(node);

    return (
        <>
            <div
                className={classNames("tree-row", depth > 0 && "has-parent", expanded && "is-expanded", selected && "is-selected", isDragged && "is-dragging", canDrop && "can-drop", runtime.dropTarget === menuKey && "is-drop-target")}
                draggable
                onDragEnd={runtime.clearDrag}
                onDragLeave={runtime.clearDropTarget}
                onDragOver={(event) => runtime.markOrgDropTarget(event, node)}
                onDragStart={(event) => runtime.startOrgDrag(event, node)}
                onDrop={(event) => runtime.dropOnOrg(event, node)}
                style={{ "--tree-depth": depth } as CSSProperties}
            >
                <button type="button" className="tree-toggle" disabled={node.children.length + node.projects.length === 0} onClick={() => props.toggleOrg(node.id)} aria-label={expanded ? "Collapse organization" : "Expand organization"}>
                    {expanded ? "−" : "+"}
                </button>
                <button type="button" className="tree-label" onClick={() => props.selectOrg(node)} title={node.name}>
                    <span>{node.name}</span>
                </button>
                <TreeActions open={props.openMenu === menuKey} setOpen={(open) => props.setOpenMenu(open ? menuKey : null)}>
                    <button type="button" onClick={() => props.openModal("org", node)}>New organization</button>
                    <button type="button" onClick={() => props.openModal("project", node)}>New project</button>
                    {node.parent_org_id !== null && <button type="button" className="danger-action" onClick={() => props.deleteOrg(node)}>Delete</button>}
                </TreeActions>
            </div>
            <div className={classNames("tree-children", expanded && "is-expanded")} aria-hidden={!expanded} style={{ "--tree-depth": depth + 1 } as CSSProperties}>
                <div className="tree-children-inner">
                    {node.children.map((child) => (
                        <TreeNode key={child.id} {...directoryProps} runtime={runtime} node={child} depth={depth + 1} />
                    ))}
                    {node.projects.map((project) => (
                        <ProjectTreeRow key={project.id} {...directoryProps} runtime={runtime} project={project} depth={depth + 1} />
                    ))}
                </div>
            </div>
        </>
    );
}

function ProjectTreeRow(props: DirectoryViewProps & { project: Project; depth: number; runtime: TreeRuntime }) {
    const { project, depth, runtime } = props;
    const selected = props.selection?.type === "project" && props.selection.id === project.id;
    const menuKey = `project-${project.id}`;
    const isDragged = runtime.dragState?.type === "project" && runtime.dragState.project.id === project.id;

    return (
        <div
            className={classNames("tree-row tree-project-row", depth > 0 && "has-parent", selected && "is-selected", isDragged && "is-dragging")}
            draggable
            onDragEnd={runtime.clearDrag}
            onDragStart={(event) => runtime.startProjectDrag(event, project)}
            style={{ "--tree-depth": depth } as CSSProperties}
        >
            <span className="tree-toggle" aria-hidden="true" />
            <button type="button" className="tree-label" onClick={() => props.selectProject(project)} title={project.name}>
                <span>{project.name}</span>
            </button>
            <TreeActions open={props.openMenu === menuKey} setOpen={(open) => props.setOpenMenu(open ? menuKey : null)}>
                <button type="button" className="danger-action" onClick={() => props.deleteProject(project)}>Delete</button>
            </TreeActions>
        </div>
    );
}

function TreeActions({ children, open, setOpen }: { children: ReactNode; open: boolean; setOpen: (open: boolean) => void }) {
    const [position, setPosition] = useState<CSSProperties>({});

    return (
        <div className="tree-actions">
            <button
                type="button"
                className="icon-button tree-menu-button"
                aria-label="More actions"
                onClick={(event) => {
                    event.stopPropagation();
                    const width = 176;
                    const height = Math.min(280, Math.max(44, Children.count(children) * 41 + 2));
                    const rect = event.currentTarget.getBoundingClientRect();
                    const top = rect.bottom + 6 + height <= window.innerHeight - 8 ? rect.bottom + 6 : Math.max(8, rect.top - height - 6);
                    setPosition({
                        left: Math.max(8, Math.min(rect.right - width, window.innerWidth - width - 8)),
                        maxHeight: height,
                        top
                    });
                    setOpen(!open);
                }}
            >
                <span className="menu-dots" aria-hidden="true">
                    <span />
                    <span />
                    <span />
                </span>
            </button>
            {open && createPortal(
                <div className="row-menu tree-inline-menu" style={position} onClick={(event) => event.stopPropagation()}>
                    {children}
                </div>,
                document.body
            )}
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
    addProjectMember: (subjectType: ProjectMemberSubject) => void;
    deleteProjectMember: (membership: ProjectMembership) => void;
}) {
    const [membershipView, setMembershipView] = useState<ProjectMemberSubject>("user");
    const userMemberships = props.memberships.filter((membership) => subjectTypeLabel(membership.subject_type) === "user");
    const groupMemberships = props.memberships.filter((membership) => subjectTypeLabel(membership.subject_type) === "group");
    const visibleMemberships = membershipView === "group" ? groupMemberships : userMemberships;

    return (
        <article className="project-detail-page project-detail-card-page">
            <section className="dashboard-panel project-summary-panel">
                <header className="project-detail-header">
                    <div>
                        <span className="panel-label">Project</span>
                        <h2>{props.project.name}</h2>
                        <p>{props.project.description || "No project description set."}</p>
                    </div>
                    <div className="project-detail-actions">
                        <span className="detail-status-pill">{props.project.is_active === false ? "inactive" : "active"}</span>
                    </div>
                </header>
                <dl className="detail-list expanded-detail-list">
                    <Detail label="Slug">{props.project.slug}</Detail>
                    <Detail label="Organization ID">{props.project.organization_id}</Detail>
                    <Detail label="Direct members">{props.memberships.length}</Detail>
                    <Detail label="Users">{userMemberships.length}</Detail>
                    <Detail label="Groups">{groupMemberships.length}</Detail>
                </dl>
            </section>
            <section className="dashboard-panel project-members-panel">
                <PanelHeading
                    label="Access"
                    title="Project members"
                    action={
                        <div className="project-members-heading-actions">
                            <div className="segmented-control project-member-tabs" role="tablist" aria-label="Project membership views">
                                {[
                                    ["user", "Users", userMemberships.length],
                                    ["group", "Groups", groupMemberships.length]
                                ].map(([key, label, count]) => (
                                    <button
                                        key={key}
                                        type="button"
                                        className={membershipView === key ? "is-active" : ""}
                                        onClick={() => setMembershipView(key as ProjectMemberSubject)}
                                    >
                                        <span>{label}</span>
                                        <strong>{count}</strong>
                                    </button>
                                ))}
                            </div>
                            <button className="button-primary compact-button" type="button" onClick={() => props.addProjectMember(membershipView)}>
                                Add {membershipView}
                            </button>
                        </div>
                    }
                />
                {props.loading && <EmptyState>Loading project members...</EmptyState>}
                {!props.loading && visibleMemberships.length === 0 && <EmptyState>No direct {membershipView} memberships.</EmptyState>}
                {!props.loading && visibleMemberships.length > 0 && (
                    <div className="compact-list">
                        {visibleMemberships.map((membership) => (
                            <div className="compact-list-row action-list-row" key={membership.id}>
                                <div>
                                    <strong>{membership.subject?.label || membership.subject?.name || membership.subject?.username || `Subject ${membership.subject_id}`}</strong>
                                    <span>{subjectTypeLabel(membership.subject_type)} / {membership.subject?.meta || "direct project member"}</span>
                                </div>
                                <div className="project-access-actions">
                                    <button type="button" className="button-secondary compact-button danger-button" onClick={() => props.deleteProjectMember(membership)}>
                                        Remove
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </section>
        </article>
    );
}

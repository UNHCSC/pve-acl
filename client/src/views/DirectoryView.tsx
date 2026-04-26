import { useEffect, useRef, useState, type CSSProperties, type DragEvent as ReactDragEvent, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { Detail, EmptyDetail, EmptyState, PanelHeading, RowActionMenu } from "../components/common";
import type { Group, ModalKey, OrgNode, Organization, OrganizationMembership, Project, ProjectMembership, Role, Selection } from "../types";
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
    orgMemberships: OrganizationMembership[];
    orgRoles: Role[];
    orgGroups: Group[];
    memberships: ProjectMembership[];
    projectRoles: Role[];
    projectGroups: Group[];
    loadingOrg: boolean;
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
    addOrganizationMember: (subjectType: ProjectMemberSubject) => void;
    addProjectMember: (subjectType: ProjectMemberSubject) => void;
    createGroup: (context: Organization | Project) => void;
    editRole: (role: Role, context: Organization | Project) => void;
    deleteRole: (role: Role) => void;
    manageGroupMembers: (group: { id: number; name: string }) => void;
    updateOrganizationMemberRole: (membership: OrganizationMembership, roleID: number) => void;
    updateProjectMemberRole: (membership: ProjectMembership, roleID: number) => void;
    deleteOrganizationMember: (membership: OrganizationMembership) => void;
    deleteProjectMember: (membership: ProjectMembership) => void;
};

type TreeRuntime = {
    canDropOnOrg: (target: OrgNode) => boolean;
    clearDrag: () => void;
    clearDropTarget: () => void;
    dragState: DraggedItem;
    dropOnOrg: (event: ReactDragEvent, target: OrgNode) => void;
    dropTarget: string | null;
    markOrgDropTarget: (event: ReactDragEvent, target: OrgNode) => void;
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

    const runtime: TreeRuntime = {
        canDropOnOrg,
        clearDrag,
        clearDropTarget: () => setDropTarget(null),
        dragState,
        dropOnOrg,
        dropTarget,
        markOrgDropTarget,
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
                    {props.selection?.type === "org" && props.selectedOrg && (
                        <OrgDetail
                            org={props.selectedOrg}
                            memberships={props.orgMemberships}
                            orgRoles={props.orgRoles}
                            orgGroups={props.orgGroups}
                            loading={props.loadingOrg}
                            addOrganizationMember={props.addOrganizationMember}
                            createOrgRole={() => props.openModal("role", props.selectedOrg)}
                            createOrgGroup={() => props.createGroup(props.selectedOrg!)}
                            editRole={(role) => props.editRole(role, props.selectedOrg!)}
                            deleteRole={props.deleteRole}
                            manageGroupMembers={props.manageGroupMembers}
                            updateOrganizationMemberRole={props.updateOrganizationMemberRole}
                            deleteOrganizationMember={props.deleteOrganizationMember}
                        />
                    )}
                    {props.selection?.type === "project" && props.selectedProject && (
                        <ProjectDetail
                            project={props.selectedProject}
                            memberships={props.memberships}
                            projectRoles={props.projectRoles}
                            projectGroups={props.projectGroups}
                            loading={props.loadingProject}
                            addProjectMember={props.addProjectMember}
                            createProjectRole={() => props.openModal("role", props.selectedProject)}
                            createProjectGroup={() => props.createGroup(props.selectedProject!)}
                            editRole={(role) => props.editRole(role, props.selectedProject!)}
                            deleteRole={props.deleteRole}
                            manageGroupMembers={props.manageGroupMembers}
                            updateProjectMemberRole={props.updateProjectMemberRole}
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
            <RowActionMenu className="tree-actions" menuClassName="tree-inline-menu" open={props.openMenu === menuKey} setOpen={(open) => props.setOpenMenu(open ? menuKey : null)}>
                <button type="button" onClick={() => props.openModal("org", node)}>New organization</button>
                <button type="button" onClick={() => props.openModal("project", node)}>New project</button>
                {node.parent_org_id !== null && <button type="button" className="danger-action" onClick={() => props.deleteOrg(node)}>Delete</button>}
            </RowActionMenu>
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
            <RowActionMenu className="tree-actions" menuClassName="tree-inline-menu" open={props.openMenu === menuKey} setOpen={(open) => props.setOpenMenu(open ? menuKey : null)}>
                <button type="button" className="danger-action" onClick={() => props.deleteProject(project)}>Delete</button>
            </RowActionMenu>
        </div>
    );
}

type ScopedMembership = ProjectMembership | OrganizationMembership;

function OrgDetail(props: {
    org: OrgNode;
    memberships: OrganizationMembership[];
    orgRoles: Role[];
    orgGroups: Group[];
    loading: boolean;
    addOrganizationMember: (subjectType: ProjectMemberSubject) => void;
    createOrgRole: () => void;
    createOrgGroup: () => void;
    editRole: (role: Role) => void;
    deleteRole: (role: Role) => void;
    manageGroupMembers: (group: { id: number; name: string }) => void;
    updateOrganizationMemberRole: (membership: OrganizationMembership, roleID: number) => void;
    deleteOrganizationMember: (membership: OrganizationMembership) => void;
}) {
    return (
        <article className="project-detail-page project-detail-card-page">
            <section className="dashboard-panel project-summary-panel">
                <header className="project-detail-header">
                    <div>
                        <span className="panel-label">Organization</span>
                        <h2>{props.org.name}</h2>
                        <p>{props.org.description || "No organization description set."}</p>
                    </div>
                    <span className="detail-status-pill">org</span>
                </header>
                <ScopeSummaryStrip
                    items={[
                        { label: "Child orgs", value: props.org.children.length },
                        { label: "Projects", value: props.org.projects.length },
                        { label: "Members", value: props.memberships.length },
                        { label: "Owned groups", value: props.orgGroups.length },
                        { label: "Roles", value: props.orgRoles.length }
                    ]}
                />
                <dl className="detail-list expanded-detail-list scope-detail-list">
                    <Detail label="Slug">{props.org.slug}</Detail>
                    <Detail label="Scope">Organization</Detail>
                    <Detail label="Parent">{props.org.parent_org_id ? `Org ${props.org.parent_org_id}` : "Root"}</Detail>
                </dl>
            </section>
            <div className="access-panel-grid">
                <ScopedMembershipPanel
                    className="access-panel-wide"
                    scopeLabel="Organization"
                    memberships={props.memberships}
                    roles={props.orgRoles}
                    loading={props.loading}
                    addMember={props.addOrganizationMember}
                    manageGroupMembers={props.manageGroupMembers}
                    updateMemberRole={(membership, roleID) => props.updateOrganizationMemberRole(membership as OrganizationMembership, roleID)}
                    deleteMember={(membership) => props.deleteOrganizationMember(membership as OrganizationMembership)}
                />
                <ScopedGroupsPanel
                    scopeLabel="Organization"
                    groups={props.orgGroups}
                    createGroup={props.createOrgGroup}
                    manageGroupMembers={props.manageGroupMembers}
                />
                <ScopedRolesPanel
                    scopeLabel="Organization"
                    scopeType="org"
                    scopeID={props.org.id}
                    roles={props.orgRoles}
                    createRole={props.createOrgRole}
                    editRole={props.editRole}
                    deleteRole={props.deleteRole}
                />
            </div>
        </article>
    );
}

function ProjectDetail(props: {
    project: Project;
    memberships: ProjectMembership[];
    projectRoles: Role[];
    projectGroups: Group[];
    loading: boolean;
    addProjectMember: (subjectType: ProjectMemberSubject) => void;
    createProjectRole: () => void;
    createProjectGroup: () => void;
    editRole: (role: Role) => void;
    deleteRole: (role: Role) => void;
    manageGroupMembers: (group: { id: number; name: string }) => void;
    updateProjectMemberRole: (membership: ProjectMembership, roleID: number) => void;
    deleteProjectMember: (membership: ProjectMembership) => void;
}) {
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
                <ScopeSummaryStrip
                    items={[
                        { label: "Members", value: props.memberships.length },
                        { label: "Owned groups", value: props.projectGroups.length },
                        { label: "Roles", value: props.projectRoles.length },
                        { label: "State", value: props.project.is_active === false ? "Inactive" : "Active" }
                    ]}
                />
                <dl className="detail-list expanded-detail-list scope-detail-list">
                    <Detail label="Slug">{props.project.slug}</Detail>
                    <Detail label="Organization">{props.project.organization?.name || `Org ${props.project.organization_id}`}</Detail>
                    <Detail label="Scope">Project</Detail>
                </dl>
            </section>
            <div className="access-panel-grid">
                <ScopedMembershipPanel
                    className="access-panel-wide"
                    scopeLabel="Project"
                    memberships={props.memberships}
                    roles={props.projectRoles}
                    loading={props.loading}
                    addMember={props.addProjectMember}
                    manageGroupMembers={props.manageGroupMembers}
                    updateMemberRole={(membership, roleID) => props.updateProjectMemberRole(membership as ProjectMembership, roleID)}
                    deleteMember={(membership) => props.deleteProjectMember(membership as ProjectMembership)}
                />
                <ScopedGroupsPanel
                    scopeLabel="Project"
                    groups={props.projectGroups}
                    createGroup={props.createProjectGroup}
                    manageGroupMembers={props.manageGroupMembers}
                />
                <ScopedRolesPanel
                    scopeLabel="Project"
                    scopeType="project"
                    scopeID={props.project.id}
                    roles={props.projectRoles}
                    createRole={props.createProjectRole}
                    editRole={props.editRole}
                    deleteRole={props.deleteRole}
                />
            </div>
        </article>
    );
}

function ScopedMembershipPanel(props: {
    className?: string;
    scopeLabel: "Organization" | "Project";
    memberships: ScopedMembership[];
    roles: Role[];
    loading: boolean;
    addMember: (subjectType: ProjectMemberSubject) => void;
    manageGroupMembers: (group: { id: number; name: string }) => void;
    updateMemberRole: (membership: ScopedMembership, roleID: number) => void;
    deleteMember: (membership: ScopedMembership) => void;
}) {
    const [membershipView, setMembershipView] = useState<ProjectMemberSubject>("user");
    const userMemberships = props.memberships.filter((membership) => subjectTypeLabel(membership.subject_type) === "user");
    const groupMemberships = props.memberships.filter((membership) => subjectTypeLabel(membership.subject_type) === "group");
    const visibleMemberships = membershipView === "group" ? groupMemberships : userMemberships;

    return (
        <section className={classNames("dashboard-panel project-members-panel", props.className)}>
            <PanelHeading
                label="Access assignments"
                title={`${props.scopeLabel} role assignments`}
                action={
                    <div className="project-members-heading-actions">
                        <div className="segmented-control project-member-tabs" role="tablist" aria-label={`${props.scopeLabel} membership views`}>
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
                        {props.roles.length > 0 && (
                            <button className="button-primary compact-button" type="button" onClick={() => props.addMember(membershipView)}>
                                Add {membershipView} grant
                            </button>
                        )}
                    </div>
                }
            />
            {props.loading && <EmptyState>Loading {props.scopeLabel.toLowerCase()} members...</EmptyState>}
            {!props.loading && visibleMemberships.length === 0 && <EmptyState>No direct {membershipView} role assignments.</EmptyState>}
            {!props.loading && visibleMemberships.length > 0 && (
                <div className="compact-list">
                    {visibleMemberships.map((membership) => {
                        const subjectLabel = membership.subject?.label || membership.subject?.name || membership.subject?.username || `Subject ${membership.subject_id}`;
                        const subjectKind = subjectTypeLabel(membership.subject_type) as ProjectMemberSubject;
                        const roleLabel = membership.access_role_name || ("project_role_label" in membership ? membership.project_role_label : "") || "direct member";
                        return (
                            <div className="compact-list-row action-list-row access-list-row" key={membership.id}>
                                <AccessRowSubject
                                    title={subjectLabel}
                                    meta={[subjectKind, membership.subject?.meta || "local"].join(" / ")}
                                />
                                <div className="project-access-actions">
                                    {props.roles.length > 0 ? (
                                        <select
                                            className="field-input compact-select access-role-select"
                                            value={membership.access_role_id || ""}
                                            aria-label={`${props.scopeLabel} access role`}
                                            onChange={(event) => props.updateMemberRole(membership, Number(event.currentTarget.value))}
                                        >
                                            <option value="" disabled>Role</option>
                                            {props.roles.map((role) => (
                                                <option key={role.id} value={role.id}>
                                                    {role.name}
                                                </option>
                                            ))}
                                        </select>
                                    ) : (
                                        <span className="access-pill">{roleLabel}</span>
                                    )}
                                    <RowActionMenu ariaLabel={`${subjectLabel} actions`} className="tree-actions access-row-actions" menuClassName="tree-inline-menu">
                                        {membershipView === "group" && (
                                            <button type="button" role="menuitem" onClick={() => props.manageGroupMembers({ id: membership.subject_id, name: subjectLabel })}>
                                                Manage members
                                            </button>
                                        )}
                                        <button type="button" role="menuitem" className="danger-action" onClick={() => props.deleteMember(membership)}>
                                            Remove assignment
                                        </button>
                                    </RowActionMenu>
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}
        </section>
    );
}

function ScopedGroupsPanel(props: {
    scopeLabel: "Organization" | "Project";
    groups: Group[];
    createGroup: () => void;
    manageGroupMembers: (group: { id: number; name: string }) => void;
}) {
    return (
        <section className="dashboard-panel project-members-panel">
            <PanelHeading
                label="Managed groups"
                title={`${props.scopeLabel}-owned groups`}
                action={<button className="button-secondary compact-button" type="button" onClick={props.createGroup}>New group</button>}
            />
            {props.groups.length === 0 && <EmptyState>No groups are owned by this scope yet.</EmptyState>}
            {props.groups.length > 0 && (
                <div className="compact-list">
                    {props.groups.map((group) => (
                        <div className="compact-list-row action-list-row access-list-row" key={group.id}>
                            <AccessRowSubject
                                title={group.name}
                                meta={group.slug}
                            />
                            <div className="project-access-actions">
                                <span className="access-pill">{group.member_count || 0} members</span>
                                <span className={classNames("access-pill", group.sync_membership && "is-sync")}>{group.sync_membership ? "LDAP" : "Local"}</span>
                                <RowActionMenu ariaLabel={`${group.name} actions`} className="tree-actions access-row-actions" menuClassName="tree-inline-menu">
                                    <button type="button" role="menuitem" onClick={() => props.manageGroupMembers({ id: group.id, name: group.name })}>
                                        Manage members
                                    </button>
                                </RowActionMenu>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </section>
    );
}

function ScopedRolesPanel(props: {
    scopeLabel: "Organization" | "Project";
    scopeType: "org" | "project";
    scopeID: number;
    roles: Role[];
    createRole: () => void;
    editRole: (role: Role) => void;
    deleteRole: (role: Role) => void;
}) {
    return (
        <section className="dashboard-panel project-members-panel">
            <PanelHeading
                label="Roles"
                title={`${props.scopeLabel} roles`}
                action={<button className="button-secondary compact-button" type="button" onClick={props.createRole}>New role</button>}
            />
            {props.roles.length === 0 && <EmptyState>No assignable roles are available here.</EmptyState>}
            {props.roles.length > 0 && (
                <div className="compact-list">
                    {props.roles.map((role) => {
                        const localRole = String(role.owner_scope_label || "").toLowerCase() === props.scopeType && role.owner_scope_id === props.scopeID;
                        const canManageRole = localRole && !role.is_system_role;
                        return (
                            <div className="compact-list-row action-list-row access-list-row role-list-row" key={role.id}>
                                <AccessRowSubject
                                    className="role-list-subject"
                                    title={role.name}
                                    meta={role.description || `${role.permission_count || 0} permissions`}
                                />
                                <div className="project-access-actions">
                                    <span
                                        className={classNames("access-pill role-scope-pill", !localRole && "is-inherited")}
                                        title={localRole ? "Owned by this scope" : `Inherited from ${role.owner_scope_label || "another scope"}`}
                                    >
                                        {localRole ? "Local" : compactScopeLabel(role.owner_scope_label)}
                                    </span>
                                    <RowActionMenu ariaLabel="Role actions" className="tree-actions role-row-actions" menuClassName="role-inline-menu">
                                        <button type="button" role="menuitem" onClick={() => props.editRole(role)}>{canManageRole ? "Edit role" : "View role"}</button>
                                        {canManageRole && <button type="button" role="menuitem" className="danger-action" onClick={() => props.deleteRole(role)}>Delete role</button>}
                                    </RowActionMenu>
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}
        </section>
    );
}

function compactScopeLabel(scopeLabel?: string) {
    switch (String(scopeLabel || "").toLowerCase()) {
        case "project":
            return "Project";
        case "org":
        case "organization":
            return "Org";
        case "global":
            return "Global";
        default:
            return "Inherited";
    }
}

function ScopeSummaryStrip({ items }: { items: { label: string; value: ReactNode }[] }) {
    return (
        <div className="scope-summary-strip">
            {items.map((item) => (
                <div className="scope-summary-item" key={item.label}>
                    <span>{item.label}</span>
                    <strong>{item.value}</strong>
                </div>
            ))}
        </div>
    );
}

function AccessRowSubject({
    className,
    title,
    meta
}: {
    className?: string;
    title: string;
    meta: string;
}) {
    return (
        <div className={classNames("access-row-subject", className)}>
            <div>
                <strong>{title}</strong>
                <span>{meta}</span>
            </div>
        </div>
    );
}

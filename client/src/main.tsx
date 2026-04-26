import "@fontsource-variable/public-sans/index.css";
import "@fontsource/ibm-plex-mono/400.css";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { apiFetch } from "./api";
import { RolePermissionModal } from "./components/RolePermissionModal";
import { ThemeSettings } from "./components/ThemeSettings";
import { ToastStack } from "./components/common";
import { GroupMembersModal, GroupModal, ImportUsersModal, OrgModal, ProjectMemberModal, ProjectModal, RoleModal } from "./components/modals";
import { useDashboardData } from "./hooks/useDashboardData";
import { useDirectorySelection } from "./hooks/useDirectorySelection";
import { useOrganizationAccess } from "./hooks/useOrganizationAccess";
import { useProjectMemberships } from "./hooks/useProjectMemberships";
import { useToasts } from "./hooks/useToasts";
import "./styles/site.css";
import type { Group, ModalKey, Organization, Project, Role, RolePermissionGrant, ThemeKey, UserImportResponse, ViewKey } from "./types";
import { applyTheme, readStoredTheme } from "./theme";
import { viewTitles } from "./types";
import { classNames, displayUser, initialView, initials } from "./ui-helpers";
import { DirectoryView } from "./views/DirectoryView";
import { HomePage } from "./views/HomePage";
import { IdentityView } from "./views/IdentityView";
import { LoginPage } from "./views/LoginPage";
import { OverviewView } from "./views/OverviewView";
import { PeopleView } from "./views/PeopleView";

applyTheme(readStoredTheme());

const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            refetchOnWindowFocus: false,
            retry: 1
        }
    }
});

function viewIsAllowed(view: ViewKey, summary: { capabilities: { canViewUsers?: boolean } } | null) {
    if (view === "people") {
        return Boolean(summary?.capabilities.canViewUsers);
    }
    return true;
}

function allowedViews(summary: { capabilities: { canViewUsers?: boolean } } | null): ViewKey[] {
    return (Object.keys(viewTitles) as ViewKey[]).filter((key) => viewIsAllowed(key, summary));
}

function DashboardApp() {
    const [view, setViewState] = useState<ViewKey>(initialView);
    const [modal, setModal] = useState<ModalKey>(null);
    const [modalContext, setModalContext] = useState<Organization | Project | null>(null);
    const [openMenu, setOpenMenu] = useState<string | null>(null);
    const [navOpen, setNavOpen] = useState(false);
    const [accountOpen, setAccountOpen] = useState(false);
    const [settingsOpen, setSettingsOpen] = useState(false);
    const [theme, setTheme] = useState<ThemeKey>(readStoredTheme);
    const [projectMemberSubjectType, setProjectMemberSubjectType] = useState<"user" | "group">("user");
    const [groupMembersContext, setGroupMembersContext] = useState<{ id: number; name: string } | null>(null);
    const [editingRole, setEditingRole] = useState<{ role: Role; context: Organization | Project } | null>(null);

    const { toasts, showToast } = useToasts();
    const { loadMyAccess, loadSummary, loadTree, loadUsers, myAccess, summary, tree, users } = useDashboardData((message) => showToast(message, "warning"));
    const { expanded, orgTree, selectedOrg, selectedProject, selection, setSelection, toggleOrg } = useDirectorySelection(tree);
    const { loadingOrg, orgGroups, orgMemberships, orgRoles, reloadOrgGroups, reloadOrgMemberships, reloadOrgRoles } = useOrganizationAccess(selectedOrg, (message) => showToast(message, "warning"));
    const { activeProject, loadingProject, memberships, projectGroups, projectRoles, reloadMemberships, reloadProjectGroups, reloadProjectRoles } = useProjectMemberships(selectedProject, (message) => showToast(message, "warning"));
    const counts = summary?.counts || {};

    useEffect(() => {
        const handler = () => setViewState(initialView());
        window.addEventListener("popstate", handler);
        return () => window.removeEventListener("popstate", handler);
    }, []);

    useEffect(() => {
        if (summary && !viewIsAllowed(view, summary)) {
            setView("overview");
        }
    }, [summary, view]);

    useEffect(() => {
        if (!openMenu) {
            return;
        }
        const closeMenu = () => setOpenMenu(null);
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                setOpenMenu(null);
            }
        };
        window.addEventListener("click", closeMenu);
        window.addEventListener("keydown", closeOnEscape);
        return () => {
            window.removeEventListener("click", closeMenu);
            window.removeEventListener("keydown", closeOnEscape);
        };
    }, [openMenu]);

    useEffect(() => {
        if (!navOpen) {
            return;
        }
        const closeNav = (event: MouseEvent) => {
            const target = event.target;
            if (target instanceof Element && target.closest("[data-dashboard-nav]")) {
                return;
            }
            setNavOpen(false);
        };
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                setNavOpen(false);
            }
        };
        window.addEventListener("click", closeNav);
        window.addEventListener("keydown", closeOnEscape);
        return () => {
            window.removeEventListener("click", closeNav);
            window.removeEventListener("keydown", closeOnEscape);
        };
    }, [navOpen]);

    useEffect(() => {
        applyTheme(theme);
    }, [theme]);

    useEffect(() => {
        if (!accountOpen && !settingsOpen) {
            return;
        }
        const closeMenus = (event: MouseEvent) => {
            const target = event.target;
            if (target instanceof Element && target.closest("[data-topbar-menu]")) {
                return;
            }
            setAccountOpen(false);
            setSettingsOpen(false);
        };
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                setAccountOpen(false);
                setSettingsOpen(false);
            }
        };
        window.addEventListener("click", closeMenus);
        window.addEventListener("keydown", closeOnEscape);
        return () => {
            window.removeEventListener("click", closeMenus);
            window.removeEventListener("keydown", closeOnEscape);
        };
    }, [accountOpen, settingsOpen]);

    const setView = (nextView: ViewKey) => {
        if (!viewIsAllowed(nextView, summary)) {
            nextView = "overview";
        }
        setViewState(nextView);
        const url = new URL(window.location.href);
        url.searchParams.set("view", nextView);
        window.history.pushState({}, "", url);
    };

    const openContextModal = (nextModal: ModalKey, context: Organization | Project | null = null) => {
        setOpenMenu(null);
        setModalContext(context);
        setModal(nextModal);
    };

    const errorMessage = (error: unknown, fallback = "Action failed") => (error instanceof Error ? error.message : fallback);

    const toastError = (error: unknown, fallback = "Action failed") => {
        const message = errorMessage(error, fallback);
        showToast(message, "warning");
        return message;
    };

    const submitModalMutation = async (operation: () => Promise<void>) => {
        try {
            await operation();
            setModal(null);
        } catch (error) {
            toastError(error);
            throw error;
        }
    };

    const createOrg = async (values: { name: string; slug: string; description: string; parentOrgID: number | null }) => {
        await apiFetch<Organization>("/api/v1/organizations", { method: "POST", body: JSON.stringify(values) });
        await Promise.all([loadTree(), loadSummary()]);
        showToast("Organization created", "success");
    };

    const createProject = async (values: { name: string; slug: string; description: string; organizationID: number }) => {
        await apiFetch<Project>("/api/v1/projects", { method: "POST", body: JSON.stringify(values) });
        await Promise.all([loadTree(), loadSummary()]);
        showToast("Project created", "success");
    };

    const importUsers = async (values: { entries: string }) => {
        const result = await apiFetch<UserImportResponse>("/api/v1/users/import", { method: "POST", body: JSON.stringify(values) });
        await Promise.all([loadUsers(), loadSummary()]);
        showToast(`${result.imported} imported, ${result.failed} failed`, result.failed > 0 ? "warning" : "success");
        return result;
    };

    const createGroup = async (values: { name: string; slug: string; description: string }) => {
        const scope =
            modalContext && "organization_id" in modalContext
                ? { ownerScopeType: "project", ownerScopeID: modalContext.id, groupType: "project" }
                : modalContext && "parent_org_id" in modalContext
                    ? { ownerScopeType: "org", ownerScopeID: modalContext.id, groupType: "custom" }
                    : { ownerScopeType: "global", groupType: "custom" };
        await apiFetch<Group>("/api/v1/groups", {
            method: "POST",
            body: JSON.stringify({ ...values, ...scope, syncSource: "local", syncMembership: false })
        });
        await Promise.all([
            loadSummary(),
            loadTree(),
            activeProject ? reloadProjectGroups() : Promise.resolve(),
            selectedOrg ? reloadOrgGroups() : Promise.resolve()
        ]);
        showToast("Group created", "success");
    };

    const createRole = async (values: { name: string; description: string }) => {
        const scope =
            modalContext && "organization_id" in modalContext
                ? { scopeType: "project", scopeID: modalContext.id }
                : modalContext && "parent_org_id" in modalContext
                    ? { scopeType: "org", scopeID: modalContext.id }
                    : {};
        await apiFetch<Role>("/api/v1/roles", { method: "POST", body: JSON.stringify({ ...values, ...scope }) });
        await Promise.all([
            loadSummary(),
            activeProject ? reloadProjectRoles() : Promise.resolve(),
            selectedOrg ? reloadOrgRoles() : Promise.resolve()
        ]);
        showToast("Role created", "success");
    };

    const reloadRoleSurfaces = async () => {
        await Promise.all([
            loadMyAccess(),
            loadSummary(),
            activeProject ? reloadProjectRoles() : Promise.resolve(),
            selectedOrg ? reloadOrgRoles() : Promise.resolve()
        ]);
    };

    const saveRole = async (role: Role, values: { name: string; description: string; permissionIDs: number[]; updateDetails: boolean; updatePermissions: boolean }) => {
        try {
            let updatedRole = role;
            if (values.updateDetails) {
                updatedRole = await apiFetch<Role>(`/api/v1/roles/${role.id}`, {
                    method: "PATCH",
                    body: JSON.stringify({ name: values.name, description: values.description })
                });
            }

            if (values.updatePermissions) {
                const currentGrants = await apiFetch<RolePermissionGrant[]>(`/api/v1/roles/${role.id}/permissions`);
                const currentIDs = new Set(currentGrants.map((grant) => grant.permission_id));
                const nextIDs = new Set(values.permissionIDs);
                const additions = values.permissionIDs.filter((permissionID) => !currentIDs.has(permissionID));
                const removals = currentGrants.filter((grant) => !nextIDs.has(grant.permission_id));

                await Promise.all([
                    ...additions.map((permissionID) =>
                        apiFetch(`/api/v1/roles/${role.id}/permissions`, {
                            method: "POST",
                            body: JSON.stringify({ permissionID })
                        })
                    ),
                    ...removals.map((grant) =>
                        apiFetch(`/api/v1/roles/${role.id}/permissions/${grant.permission_id}`, {
                            method: "DELETE"
                        })
                    )
                ]);
            }

            await reloadRoleSurfaces();
            showToast("Role updated", "success");
            return updatedRole;
        } catch (error) {
            toastError(error, "Failed to update role");
            throw error;
        }
    };

    const deleteRole = async (role: Role) => {
        if (role.is_system_role) {
            showToast("System roles cannot be deleted.", "warning");
            return;
        }
        if (!window.confirm(`Delete ${role.name}? This only works when the role is not assigned anywhere.`)) {
            return;
        }
        await apiFetch(`/api/v1/roles/${role.id}`, { method: "DELETE" });
        await reloadRoleSurfaces();
        showToast("Role deleted", "success");
    };

    const addOrganizationMember = async (values: { subjectType: string; subjectRef: string; roleID?: number }) => {
        if (!selectedOrg) {
            return;
        }
        await apiFetch(`/api/v1/organizations/${selectedOrg.id}/memberships`, { method: "POST", body: JSON.stringify(values) });
        await reloadOrgMemberships();
        showToast("Organization member added", "success");
    };

    const addProjectMember = async (values: { subjectType: string; subjectRef: string; roleID?: number }) => {
        if (!activeProject) {
            return;
        }
        await apiFetch(`/api/v1/projects/${activeProject.id}/memberships`, { method: "POST", body: JSON.stringify(values) });
        await reloadMemberships();
        showToast("Project member added", "success");
    };

    const moveOrg = async (org: Organization, parentOrgID: number | null) => {
        await apiFetch(`/api/v1/organizations/${org.id}`, { method: "PATCH", body: JSON.stringify({ parentOrgID }) });
        await loadTree();
        showToast("Organization moved", "success");
    };

    const moveProject = async (project: Project, organizationID: number) => {
        await apiFetch(`/api/v1/projects/${project.id}`, { method: "PATCH", body: JSON.stringify({ organizationID }) });
        await loadTree();
        showToast("Project moved", "success");
    };

    const deleteOrg = async (org: Organization) => {
        if (!window.confirm(`Delete ${org.name}? Organizations must be empty before they can be deleted.`)) {
            return;
        }
        await apiFetch(`/api/v1/organizations/${org.id}`, { method: "DELETE" });
        await Promise.all([loadTree(), loadSummary()]);
        setSelection((previous) => (previous?.type === "org" && previous.id === org.id ? null : previous));
        showToast("Organization deleted", "success");
    };

    const deleteProject = async (project: Project) => {
        if (!window.confirm(`Delete ${project.name}?`)) {
            return;
        }
        await apiFetch(`/api/v1/projects/${encodeURIComponent(project.slug)}`, { method: "DELETE" });
        await Promise.all([loadTree(), loadSummary()]);
        setSelection((previous) => (previous?.type === "project" && previous.id === project.id ? null : previous));
        showToast("Project deleted", "success");
    };

    const mutateWithToast = async (operation: () => Promise<void>) => {
        setOpenMenu(null);
        try {
            await operation();
        } catch (error) {
            toastError(error);
        }
    };

    const logout = async () => {
        try {
            await fetch("/api/v1/auth/logout", { credentials: "same-origin", method: "POST" });
        } finally {
            window.location.href = "/login";
        }
    };

    const renderDashboardActions = () => (
        <>
            <ThemeSettings
                open={settingsOpen}
                setOpen={(open) => {
                    setSettingsOpen(open);
                    if (open) {
                        setAccountOpen(false);
                    }
                }}
                theme={theme}
                setTheme={setTheme}
            />
            {summary ? (
                <div className="account-menu" data-topbar-menu>
                    <button
                        className="account-button"
                        type="button"
                        aria-haspopup="menu"
                        aria-expanded={accountOpen}
                        onClick={() => {
                            setAccountOpen((open) => !open);
                            setSettingsOpen(false);
                        }}
                    >
                        <span className="account-avatar">{initials(displayUser(summary.currentUser))}</span>
                        <span>{displayUser(summary.currentUser)}</span>
                    </button>
                    <div className="account-dropdown" hidden={!accountOpen}>
                        <div className="account-summary">
                            <strong>{displayUser(summary.currentUser)}</strong>
                            <span>{summary.currentUser.email}</span>
                            <span>{summary.currentUser.isSiteAdmin ? "Site admin" : `${summary.currentUser.groupCount || 0} groups`}</span>
                        </div>
                        <button className="logout-button" type="button" onClick={logout}>
                            Logout
                        </button>
                    </div>
                </div>
            ) : (
                <a href="/login" className="button-primary">
                    Sign in
                </a>
            )}
        </>
    );

    return (
        <div className="dashboard-shell">
            <aside className="dashboard-sidebar" data-dashboard-nav>
                <div className="dashboard-sidebar-header">
                    <a href="/" className="brand-mark">
                        <img className="brand-logo" src="/static/logo.svg" alt="" aria-hidden="true" />
                        <span>Organesson Cloud</span>
                    </a>
                    <div className="dashboard-sidebar-controls">
                        <div className="dashboard-actions dashboard-sidebar-actions">{renderDashboardActions()}</div>
                        <button className="nav-menu-button" type="button" aria-label="Open dashboard navigation" aria-expanded={navOpen} onClick={() => setNavOpen((open) => !open)}>
                            <span />
                            <span />
                            <span />
                        </button>
                    </div>
                </div>
                <nav className={classNames("dashboard-nav", navOpen && "is-open")} aria-label="Dashboard navigation">
                    {allowedViews(summary).map((key) => (
                        <button
                            key={key}
                            type="button"
                            className={view === key ? "is-active" : ""}
                            onClick={() => {
                                setView(key);
                                setNavOpen(false);
                            }}
                        >
                            {viewTitles[key]}
                        </button>
                    ))}
                </nav>
            </aside>

            <div className={classNames("dashboard-main", view === "directory" && "is-directory-view")}>
                <header className="dashboard-topbar">
                    <div>
                        <p className="eyebrow">Control plane</p>
                        <h1>{viewTitles[view]}</h1>
                    </div>
                    <div className="dashboard-actions dashboard-topbar-actions">{renderDashboardActions()}</div>
                </header>

                <main className="dashboard-content">
                    {view === "overview" && <OverviewView counts={counts} tree={tree} capabilities={summary?.capabilities || {}} setView={setView} selectProject={setSelection} />}
                    {view === "directory" && (
                        <DirectoryView
                            orgTree={orgTree}
                            expanded={expanded}
                            selection={selection}
                            selectedOrg={selectedOrg}
                            selectedProject={activeProject || selectedProject}
                            orgMemberships={orgMemberships}
                            orgRoles={orgRoles}
                            orgGroups={orgGroups}
                            memberships={memberships}
                            projectRoles={projectRoles}
                            projectGroups={projectGroups}
                            loadingOrg={loadingOrg}
                            loadingProject={loadingProject}
                            openMenu={openMenu}
                            setOpenMenu={setOpenMenu}
                            toggleOrg={toggleOrg}
                            selectOrg={(org) => setSelection({ type: "org", id: org.id })}
                            selectProject={(project) => setSelection({ type: "project", id: project.id, slug: project.slug })}
                            openModal={openContextModal}
                            moveOrg={(org, parentOrgID) => mutateWithToast(() => moveOrg(org, parentOrgID))}
                            moveProject={(project, organizationID) => mutateWithToast(() => moveProject(project, organizationID))}
                            deleteOrg={(org) => mutateWithToast(() => deleteOrg(org))}
                            deleteProject={(project) => mutateWithToast(() => deleteProject(project))}
                            addOrganizationMember={(subjectType) => {
                                setProjectMemberSubjectType(subjectType);
                                openContextModal("project-member", selectedOrg);
                            }}
                            addProjectMember={(subjectType) => {
                                setProjectMemberSubjectType(subjectType);
                                openContextModal("project-member", activeProject || selectedProject);
                            }}
                            createGroup={(context) => openContextModal("group", context)}
                            editRole={(role, context) => setEditingRole({ role, context })}
                            deleteRole={(role) => mutateWithToast(() => deleteRole(role))}
                            manageGroupMembers={(group) => {
                                setGroupMembersContext(group);
                                openContextModal("group-members");
                            }}
                            updateOrganizationMemberRole={(membership, roleID) =>
                                mutateWithToast(async () => {
                                    if (!selectedOrg || !roleID) {
                                        return;
                                    }
                                    await apiFetch(`/api/v1/organizations/${selectedOrg.id}/memberships/${membership.id}`, {
                                        method: "PATCH",
                                        body: JSON.stringify({ roleID })
                                    });
                                    await reloadOrgMemberships();
                                    showToast("Organization role updated", "success");
                                })
                            }
                            updateProjectMemberRole={(membership, roleID) =>
                                mutateWithToast(async () => {
                                    if (!activeProject || !roleID) {
                                        return;
                                    }
                                    await apiFetch(`/api/v1/projects/${activeProject.id}/memberships/${membership.id}`, {
                                        method: "PATCH",
                                        body: JSON.stringify({ roleID })
                                    });
                                    await reloadMemberships();
                                    showToast("Project role updated", "success");
                                })
                            }
                            deleteOrganizationMember={(membership) =>
                                mutateWithToast(async () => {
                                    if (!selectedOrg || !window.confirm("Remove this organization member?")) {
                                        return;
                                    }
                                    await apiFetch(`/api/v1/organizations/${selectedOrg.id}/memberships/${membership.id}`, { method: "DELETE" });
                                    await reloadOrgMemberships();
                                    showToast("Organization member removed", "success");
                                })
                            }
                            deleteProjectMember={(membership) =>
                                mutateWithToast(async () => {
                                    if (!activeProject || !window.confirm("Remove this project member?")) {
                                        return;
                                    }
                                    await apiFetch(`/api/v1/projects/${activeProject.id}/memberships/${membership.id}`, { method: "DELETE" });
                                    await reloadMemberships();
                                    showToast("Project member removed", "success");
                                })
                            }
                        />
                    )}
                    {view === "people" && summary?.capabilities.canViewUsers && <PeopleView users={users} canImport={Boolean(summary.capabilities.canManageUsers)} openImport={() => openContextModal("import-users")} />}
                    {view === "identity" && <IdentityView summary={summary} myAccess={myAccess} />}
                </main>
            </div>

            <ToastStack toasts={toasts} />
            {modal === "org" && (
                <OrgModal
                    context={modalContext as Organization | null}
                    orgs={tree?.organizations || []}
                    onClose={() => setModal(null)}
                    onSubmit={(values) => submitModalMutation(() => createOrg(values))}
                />
            )}
            {modal === "project" && (
                <ProjectModal
                    context={modalContext as Organization | null}
                    orgs={tree?.organizations || []}
                    onClose={() => setModal(null)}
                    onSubmit={(values) => submitModalMutation(() => createProject(values))}
                />
            )}
            {modal === "import-users" && (
                <ImportUsersModal
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        try {
                            return await importUsers(values);
                        } catch (error) {
                            toastError(error, "User import failed");
                            throw error;
                        }
                    }}
                />
            )}
            {modal === "group" && (
                <GroupModal
                    context={modalContext}
                    onClose={() => setModal(null)}
                    onSubmit={(values) => submitModalMutation(() => createGroup(values))}
                />
            )}
            {modal === "role" && (
                <RoleModal
                    context={modalContext}
                    onClose={() => setModal(null)}
                    onSubmit={(values) => submitModalMutation(() => createRole(values))}
                />
            )}
            {modal === "project-member" && (
                <ProjectMemberModal
                    defaultSubjectType={projectMemberSubjectType}
                    scopeLabel={modalContext && "parent_org_id" in modalContext ? "Organization" : "Project"}
                    roles={modalContext && "parent_org_id" in modalContext ? orgRoles : projectRoles}
                    onClose={() => setModal(null)}
                    onSubmit={(values) =>
                        submitModalMutation(async () => {
                            if (modalContext && "parent_org_id" in modalContext) {
                                await addOrganizationMember(values);
                            } else {
                                await addProjectMember(values);
                            }
                        })
                    }
                />
            )}
            {modal === "group-members" && groupMembersContext && (
                <GroupMembersModal
                    group={groupMembersContext}
                    onError={(message) => showToast(message, "warning")}
                    onClose={() => setModal(null)}
                />
            )}
            {editingRole && (
                <RolePermissionModal
                    role={editingRole.role}
                    editable={roleIsLocalToContext(editingRole.role, editingRole.context)}
                    onClose={() => setEditingRole(null)}
                    saveRole={saveRole}
                />
            )}
        </div>
    );
}

function roleIsLocalToContext(role: Role, context: Organization | Project): boolean {
    if (role.is_system_role) {
        return false;
    }
    if ("organization_id" in context) {
        return String(role.owner_scope_label || "").toLowerCase() === "project" && role.owner_scope_id === context.id;
    }
    return String(role.owner_scope_label || "").toLowerCase() === "org" && role.owner_scope_id === context.id;
}

const mount = document.getElementById("dashboard-root");
if (mount) {
    createRoot(mount).render(
        <BrowserRouter>
            <QueryClientProvider client={queryClient}>
                <DashboardApp />
            </QueryClientProvider>
        </BrowserRouter>
    );
}

const homeMount = document.getElementById("home-root");
if (homeMount) {
    const currentYear = homeMount.dataset.currentYear || new Date().getFullYear().toString();

    function HomeRoot() {
        const [theme, setTheme] = useState<ThemeKey>(readStoredTheme);

        useEffect(() => {
            applyTheme(theme);
        }, [theme]);

        return <HomePage currentYear={currentYear} theme={theme} setTheme={setTheme} />;
    }

    createRoot(homeMount).render(<HomeRoot />);
}

const loginMount = document.getElementById("login-root");
if (loginMount) {
    createRoot(loginMount).render(<LoginPage redirect={loginMount.dataset.redirect || ""} loginError={loginMount.dataset.loginError || ""} />);
}

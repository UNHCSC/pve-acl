import "@fontsource-variable/public-sans/index.css";
import "@fontsource/ibm-plex-mono/400.css";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { apiFetch } from "./api";
import { ThemeSettings } from "./components/ThemeSettings";
import { ToastStack } from "./components/common";
import { GrantModal, GroupModal, ImportUsersModal, OrgModal, ProjectMemberModal, ProjectModal, RoleModal } from "./components/modals";
import { useDashboardData } from "./hooks/useDashboardData";
import { useDirectorySelection } from "./hooks/useDirectorySelection";
import { useProjectMemberships } from "./hooks/useProjectMemberships";
import { useToasts } from "./hooks/useToasts";
import "./styles/site.css";
import type { Group, ModalKey, Organization, Project, Role, RoleBinding, RolePermissionGrant, ThemeKey, UserImportResponse, ViewKey } from "./types";
import { applyTheme, readStoredTheme } from "./theme";
import { viewTitles } from "./types";
import { classNames, displayUser, initialView, initials } from "./ui-helpers";
import { AccessView } from "./views/AccessView";
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

function viewIsAllowed(view: ViewKey, summary: { capabilities: { canViewUsers?: boolean; canViewAccess?: boolean } } | null) {
    if (view === "people") {
        return Boolean(summary?.capabilities.canViewUsers);
    }
    if (view === "access") {
        return Boolean(summary?.capabilities.canViewAccess);
    }
    return true;
}

function allowedViews(summary: { capabilities: { canViewUsers?: boolean; canViewAccess?: boolean } } | null): ViewKey[] {
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

    const { toasts, showToast } = useToasts();
    const { access, loadAccess, loadMyAccess, loadSummary, loadTree, loadUsers, myAccess, summary, tree, users } = useDashboardData((message) => showToast(message, "warning"));
    const { expanded, orgTree, selectedOrg, selectedProject, selection, setSelection, toggleOrg } = useDirectorySelection(tree);
    const { activeProject, loadingProject, memberships, reloadMemberships } = useProjectMemberships(selectedProject, (message) => showToast(message, "warning"));
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
        await apiFetch<Group>("/api/v1/groups", {
            method: "POST",
            body: JSON.stringify({ ...values, groupType: "custom", syncSource: "local", syncMembership: false })
        });
        await Promise.all([loadAccess(), loadSummary()]);
        showToast("Group created", "success");
    };

    const createRole = async (values: { name: string; description: string }) => {
        await apiFetch<Role>("/api/v1/roles", { method: "POST", body: JSON.stringify(values) });
        await Promise.all([loadAccess(), loadSummary()]);
        showToast("Role created", "success");
    };

    const saveRolePermissions = async (role: Role, permissionIDs: number[]) => {
        const currentGrants = await apiFetch<RolePermissionGrant[]>(`/api/v1/roles/${role.id}/permissions`);
        const currentIDs = new Set(currentGrants.map((grant) => grant.permission_id));
        const nextIDs = new Set(permissionIDs);
        const additions = permissionIDs.filter((permissionID) => !currentIDs.has(permissionID));
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
        await Promise.all([loadAccess(), loadMyAccess(), loadSummary()]);
        showToast("Role permissions updated", "success");
    };

    const createGrant = async (values: { roleID: number; subjectType: string; subjectRef: string; scopeType: string; scopeID: number | null }) => {
        await apiFetch<RoleBinding>("/api/v1/role-bindings", { method: "POST", body: JSON.stringify(values) });
        await Promise.all([loadAccess(), loadMyAccess(), loadSummary()]);
        showToast("Role binding created", "success");
    };

    const addProjectMember = async (values: { subjectType: string; subjectRef: string }) => {
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
            showToast(error instanceof Error ? error.message : "Action failed", "warning");
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
                    {view === "overview" && <OverviewView counts={counts} tree={tree} access={access} capabilities={summary?.capabilities || {}} setView={setView} selectProject={setSelection} />}
                    {view === "directory" && (
                        <DirectoryView
                            orgTree={orgTree}
                            expanded={expanded}
                            selection={selection}
                            selectedOrg={selectedOrg}
                            selectedProject={activeProject || selectedProject}
                            memberships={memberships}
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
                            addProjectMember={(subjectType) => {
                                setProjectMemberSubjectType(subjectType);
                                openContextModal("project-member", activeProject || selectedProject);
                            }}
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
                    {view === "access" && summary?.capabilities.canViewAccess && (
                        <AccessView
                            access={access}
                            capabilities={summary.capabilities}
                            openGroup={() => openContextModal("group")}
                            openRole={() => openContextModal("role")}
                            openGrant={() => openContextModal("grant")}
                            saveRolePermissions={saveRolePermissions}
                            deleteGrant={(grant) =>
                                mutateWithToast(async () => {
                                    if (!window.confirm("Delete this role binding?")) {
                                        return;
                                    }
                                    await apiFetch(`/api/v1/role-bindings/${grant.id}`, { method: "DELETE" });
                                    await Promise.all([loadAccess(), loadMyAccess(), loadSummary()]);
                                    showToast("Role binding deleted", "success");
                                })
                            }
                        />
                    )}
                </main>
            </div>

            <ToastStack toasts={toasts} />
            {modal === "org" && (
                <OrgModal
                    context={modalContext as Organization | null}
                    orgs={tree?.organizations || []}
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        await createOrg(values);
                        setModal(null);
                    }}
                />
            )}
            {modal === "project" && (
                <ProjectModal
                    context={modalContext as Organization | null}
                    orgs={tree?.organizations || []}
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        await createProject(values);
                        setModal(null);
                    }}
                />
            )}
            {modal === "import-users" && (
                <ImportUsersModal
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        return importUsers(values);
                    }}
                />
            )}
            {modal === "group" && (
                <GroupModal
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        await createGroup(values);
                        setModal(null);
                    }}
                />
            )}
            {modal === "role" && (
                <RoleModal
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        await createRole(values);
                        setModal(null);
                    }}
                />
            )}
            {modal === "grant" && (
                <GrantModal
                    access={access}
                    orgs={tree?.organizations || []}
                    projects={tree?.projects || []}
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        await createGrant(values);
                        setModal(null);
                    }}
                />
            )}
            {modal === "project-member" && (
                <ProjectMemberModal
                    defaultSubjectType={projectMemberSubjectType}
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        await addProjectMember(values);
                        setModal(null);
                    }}
                />
            )}
        </div>
    );
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

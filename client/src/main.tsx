import "@fontsource-variable/public-sans/index.css";
import "@fontsource/ibm-plex-mono/400.css";
import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { apiFetch } from "./api";
import { ToastStack } from "./components/common";
import { GrantModal, GroupModal, MoveOrgModal, MoveProjectModal, OrgModal, ProjectMemberModal, ProjectModal, RoleModal, UserModal } from "./components/modals";
import { useDashboardData } from "./hooks/useDashboardData";
import { useDirectorySelection } from "./hooks/useDirectorySelection";
import { useProjectMemberships } from "./hooks/useProjectMemberships";
import { useToasts } from "./hooks/useToasts";
import "./styles/site.css";
import type { Group, ModalKey, Organization, Project, Role, RoleBinding, User, ViewKey } from "./types";
import { viewTitles } from "./types";
import { classNames, displayUser, initialView, initials } from "./ui-helpers";
import { AccessView } from "./views/AccessView";
import { DirectoryView } from "./views/DirectoryView";
import { HomePage } from "./views/HomePage";
import { IdentityView } from "./views/IdentityView";
import { LoginPage } from "./views/LoginPage";
import { OverviewView } from "./views/OverviewView";
import { PeopleView } from "./views/PeopleView";

function DashboardApp() {
    const [view, setViewState] = useState<ViewKey>(initialView);
    const [modal, setModal] = useState<ModalKey>(null);
    const [modalContext, setModalContext] = useState<Organization | Project | null>(null);
    const [openMenu, setOpenMenu] = useState<string | null>(null);
    const [accountOpen, setAccountOpen] = useState(false);

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

    const setView = (nextView: ViewKey) => {
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

    const createUser = async (values: { username: string; displayName: string; email: string }) => {
        await apiFetch<User>("/api/v1/users", { method: "POST", body: JSON.stringify(values) });
        await Promise.all([loadUsers(), loadSummary()]);
        showToast("User created", "success");
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

    const createGrant = async (values: { roleID: number; subjectType: string; subjectRef: string; scopeType: string; scopeID: number | null }) => {
        await apiFetch<RoleBinding>("/api/v1/role-bindings", { method: "POST", body: JSON.stringify(values) });
        await Promise.all([loadAccess(), loadMyAccess(), loadSummary()]);
        showToast("Role binding created", "success");
    };

    const addProjectMember = async (values: { subjectType: string; subjectRef: string; projectRole: string }) => {
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

    return (
        <div className="dashboard-shell">
            <aside className="dashboard-sidebar">
                <a href="/" className="brand-mark">
                    <span className="brand-icon">PC</span>
                    <span>PVE Cloud</span>
                </a>
                <nav className="dashboard-nav" aria-label="Dashboard navigation">
                    {(Object.keys(viewTitles) as ViewKey[]).map((key) => (
                        <button key={key} type="button" className={view === key ? "is-active" : ""} onClick={() => setView(key)}>
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
                    <div className="dashboard-actions">
                        {summary ? (
                            <div className="account-menu">
                                <button className="account-button" type="button" onClick={() => setAccountOpen((open) => !open)}>
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
                    </div>
                </header>

                <main className="dashboard-content">
                    {view === "overview" && <OverviewView counts={counts} tree={tree} access={access} setView={setView} selectProject={setSelection} />}
                    {view === "directory" && (
                        <DirectoryView
                            orgTree={orgTree}
                            tree={tree}
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
                            moveOrg={(org) => openContextModal("move-org", org)}
                            moveProject={(project) => openContextModal("move-project", project)}
                            deleteOrg={(org) => mutateWithToast(() => deleteOrg(org))}
                            deleteProject={(project) => mutateWithToast(() => deleteProject(project))}
                            addProjectMember={() => openContextModal("project-member", activeProject || selectedProject)}
                            updateProjectMember={(membership, nextRole) =>
                                mutateWithToast(async () => {
                                    if (!activeProject) {
                                        return;
                                    }
                                    await apiFetch(`/api/v1/projects/${activeProject.id}/memberships/${membership.id}`, {
                                        method: "PATCH",
                                        body: JSON.stringify({ projectRole: nextRole })
                                    });
                                    await reloadMemberships();
                                    showToast("Project member updated", "success");
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
                    {view === "people" && <PeopleView users={users} openCreate={() => openContextModal("user")} />}
                    {view === "identity" && <IdentityView summary={summary} myAccess={myAccess} />}
                    {view === "access" && (
                        <AccessView
                            access={access}
                            openGroup={() => openContextModal("group")}
                            openRole={() => openContextModal("role")}
                            openGrant={() => openContextModal("grant")}
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
            {modal === "user" && (
                <UserModal
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        await createUser(values);
                        setModal(null);
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
                    groups={access.groups}
                    users={users}
                    onClose={() => setModal(null)}
                    onSubmit={async (values) => {
                        await addProjectMember(values);
                        setModal(null);
                    }}
                />
            )}
            {modal === "move-org" && modalContext && "parent_org_id" in modalContext && (
                <MoveOrgModal
                    org={modalContext}
                    orgTree={orgTree}
                    onClose={() => setModal(null)}
                    onSubmit={async (parentOrgID) => {
                        await moveOrg(modalContext, parentOrgID);
                        setModal(null);
                    }}
                />
            )}
            {modal === "move-project" && modalContext && "organization_id" in modalContext && (
                <MoveProjectModal
                    project={modalContext}
                    orgTree={orgTree}
                    onClose={() => setModal(null)}
                    onSubmit={async (organizationID) => {
                        await moveProject(modalContext, organizationID);
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
            <DashboardApp />
        </BrowserRouter>
    );
}

const homeMount = document.getElementById("home-root");
if (homeMount) {
    createRoot(homeMount).render(<HomePage currentYear={homeMount.dataset.currentYear || new Date().getFullYear().toString()} />);
}

const loginMount = document.getElementById("login-root");
if (loginMount) {
    createRoot(loginMount).render(<LoginPage redirect={loginMount.dataset.redirect || ""} loginError={loginMount.dataset.loginError || ""} />);
}

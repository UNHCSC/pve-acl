const dashboard = document.querySelector("[data-dashboard]");
const projectModal = document.querySelector("[data-project-modal]");
const userModal = document.querySelector("[data-user-modal]");
const groupModal = document.querySelector("[data-group-modal]");
const groupMembersModal = document.querySelector("[data-group-members-modal]");
const roleModal = document.querySelector("[data-role-modal]");
const grantModal = document.querySelector("[data-grant-modal]");
const rolePermissionsModal = document.querySelector("[data-role-permissions-modal]");
const projectMemberPopover = document.querySelector("[data-project-member-popover]");
const accountMenu = document.querySelector("[data-account-menu]");
const accountDropdown = document.querySelector("[data-account-dropdown]");
const toastStack = document.querySelector("[data-toast-stack]");

const viewTitles = {
    overview: "Overview",
    projects: "Projects",
    people: "People",
    identity: "Identity",
    access: "Access"
};
let recentsKey = "pve-cloud.recents.anonymous";
let cachedProjects = [];
let cachedUsers = [];
let cachedGroups = [];
let cachedRoles = [];
let cachedPermissions = [];
let cachedRoleBindings = [];
let currentProjectMemberships = [];
let selectedProject = null;
let selectedGroup = null;
let selectedRole = null;
let currentView = "overview";
let lastAuthorizedView = "overview";
let selectedProjectAccessType = "user";

class APIError extends Error {
    constructor(message, status) {
        super(message);
        this.status = status;
    }
}

async function apiFetch(path, options = {}) {
    const response = await fetch(path, {
        credentials: "same-origin",
        headers: {
            "Accept": "application/json",
            ...(options.body ? { "Content-Type": "application/json" } : {}),
            ...(options.headers || {})
        },
        ...options
    });

    const contentType = response.headers.get("content-type") || "";
    const data = contentType.includes("application/json") ? await response.json() : null;

    if (!response.ok) {
        const message = data && data.error ? data.error : `Request failed with ${response.status}`;
        throw new APIError(message, response.status);
    }

    return data;
}

function setText(selector, value) {
    if (!dashboard) {
        return;
    }
    dashboard.querySelectorAll(selector).forEach((element) => {
        element.textContent = value;
    });
}

function showToast(message, tone = "info") {
    if (!toastStack || !message) {
        return;
    }
    const toast = document.createElement("div");
    toast.className = `toast-message is-${tone}`;
    toast.textContent = message;
    toastStack.append(toast);
    window.setTimeout(() => toast.remove(), 5200);
}

function consumeInitialToast() {
    const params = new URLSearchParams(window.location.search);
    const toast = params.get("toast");
    if (toast) {
        showToast(toast, params.get("tone") || "info");
        params.delete("toast");
        params.delete("tone");
        const query = params.toString();
        history.replaceState(null, "", `${window.location.pathname}${query ? `?${query}` : ""}${window.location.hash}`);
    }
}

function initialsFor(username) {
    const clean = (username || "").trim();
    if (!clean) {
        return "--";
    }
    return clean.split(/\s+/).map((part) => part[0]).join("").slice(0, 2).toUpperCase();
}

function dashboardURL(params = {}) {
    const url = new URL(window.location.href);
    Object.entries(params).forEach(([key, value]) => {
        if (value === undefined || value === null || value === "") {
            url.searchParams.delete(key);
        } else {
            url.searchParams.set(key, value);
        }
    });
    const query = url.searchParams.toString();
    return `${url.pathname}${query ? `?${query}` : ""}`;
}

function switchView(viewName, options = {}) {
    if (!dashboard || !viewTitles[viewName]) {
        return;
    }

    dashboard.querySelectorAll("[data-view]").forEach((view) => {
        const active = view.dataset.view === viewName;
        view.hidden = !active;
        view.classList.toggle("is-active", active);
    });

    document.querySelectorAll("[data-view-tab]").forEach((tab) => {
        tab.classList.toggle("is-active", tab.dataset.viewTab === viewName);
    });

    setText("[data-view-title]", viewTitles[viewName]);
    currentView = viewName;
    if (options.authorized !== false) {
        lastAuthorizedView = viewName;
    }
    if (options.updateURL !== false) {
        history.pushState(null, "", dashboardURL({
            view: viewName,
            project: options.projectSlug || null
        }));
    }
    if (viewName !== "overview") {
        addRecent({
            kind: "View",
            label: viewTitles[viewName],
            detail: "Opened dashboard section",
            target: { view: viewName }
        });
        renderRecents();
    }
}

function bindTabs() {
    document.querySelectorAll("[data-view-tab]").forEach((tab) => {
        tab.addEventListener("click", () => switchView(tab.dataset.viewTab));
    });
}

function renderSummary(summary) {
    const counts = summary.counts || {};
    for (const [key, value] of Object.entries(counts)) {
        setText(`[data-count="${key}"]`, value);
    }

    const user = summary.currentUser || {};
    const label = user.displayName || user.username || "Signed in";
    const userKey = user.id || user.username || "anonymous";
    recentsKey = `pve-cloud.recents.${userKey}`;
    setText("[data-user-initials]", initialsFor(label));
    setText("[data-user-name]", label);
    setText("[data-user-meta]", `${user.username || "unknown"} · ${user.authSource || "local"}`);
    setText("[data-account-label]", user.username || label);
    setText("[data-account-email]", user.email || "No email synced");
    setText("[data-user-groups]", user.groupCount ?? 0);
    setText("[data-user-admin]", user.isSiteAdmin ? "Site admin" : "Standard user");

    const canCreate = Boolean(summary.capabilities && summary.capabilities.canCreateProjects);
    document.querySelectorAll("[data-open-project-modal], [data-project-form] input, [data-project-form] button[type='submit']").forEach((control) => {
        control.disabled = !canCreate;
    });
    document.querySelectorAll("[data-open-user-modal], [data-user-form] input, [data-user-form] button[type='submit']").forEach((control) => {
        control.disabled = !(summary.capabilities && summary.capabilities.canManageUsers);
    });
    document.querySelectorAll("[data-open-group-modal], [data-group-form] input, [data-group-form] select, [data-group-form] button[type='submit']").forEach((control) => {
        control.disabled = !(summary.capabilities && summary.capabilities.canManageGroups);
    });
    document.querySelectorAll("[data-open-role-modal], [data-role-form] input, [data-role-form] button[type='submit']").forEach((control) => {
        control.disabled = !(summary.capabilities && summary.capabilities.canManageRoles);
    });
    document.querySelectorAll("[data-open-grant-modal], [data-grant-form] input, [data-grant-form] select, [data-grant-form] button[type='submit']").forEach((control) => {
        control.disabled = !(summary.capabilities && summary.capabilities.canManageRoles);
    });

    const signInAction = dashboard.querySelector("[data-sign-in-action]");
    if (signInAction) {
        signInAction.hidden = true;
    }
    if (accountMenu) {
        accountMenu.hidden = false;
    }
}

function renderProjects(projects) {
    cachedProjects = projects || [];
    const list = dashboard.querySelector("[data-project-list]");
    if (!list) {
        return;
    }

    list.replaceChildren();
    if (!projects || projects.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-state";
        empty.textContent = "No local projects yet.";
        list.append(empty);
        return;
    }

    for (const project of projects) {
        const row = document.createElement("div");
        row.className = "data-table-row";
        row.dataset.projectSlug = project.slug;

        const name = document.createElement("div");
        const title = document.createElement("strong");
        title.textContent = project.name || project.slug;
        const description = document.createElement("span");
        description.textContent = project.description || "No description";
        name.append(title, description);

        const slug = document.createElement("span");
        slug.textContent = project.slug;

        const status = document.createElement("span");
        status.className = project.is_active ? "status-pill is-running" : "status-pill is-muted";
        status.textContent = project.is_active ? "Active" : "Archived";

        const actions = document.createElement("div");
        actions.className = "row-actions";
        const menuButton = document.createElement("button");
        menuButton.className = "icon-button row-menu-button";
        menuButton.type = "button";
        menuButton.textContent = "⋯";
        menuButton.title = "Project actions";
        const menu = document.createElement("div");
        menu.className = "row-menu";
        menu.hidden = true;
        const openAction = document.createElement("button");
        openAction.type = "button";
        openAction.textContent = "Open";
        openAction.addEventListener("click", () => navigateTo({
            view: "projects",
            resourceType: "project",
            resourceID: project.slug,
            label: project.name || project.slug
        }));
        const deleteAction = document.createElement("button");
        deleteAction.type = "button";
        deleteAction.className = "danger-action";
        deleteAction.textContent = "Delete";
        deleteAction.addEventListener("click", () => deleteProject(project));
        menuButton.addEventListener("click", (event) => {
            event.stopPropagation();
            toggleFloatingMenu(menuButton, menu);
        });
        menu.addEventListener("click", (event) => event.stopPropagation());
        menu.append(openAction, deleteAction);
        actions.append(menuButton, menu);

        row.append(name, slug, status, actions);
        row.addEventListener("click", () => navigateTo({
            view: "projects",
            resourceType: "project",
            resourceID: project.slug,
            label: project.name || project.slug
        }));
        list.append(row);
    }
}

function closeRowMenus(except = null) {
    document.querySelectorAll(".row-menu").forEach((menu) => {
        if (menu !== except) {
            menu.hidden = true;
            menu.dataset.open = "false";
        }
    });
}

function positionFloatingElement(anchor, element, options = {}) {
    if (!anchor || !element) {
        return;
    }
    const rect = anchor.getBoundingClientRect();
    const margin = 8;
    const width = element.offsetWidth || options.width || 180;
    const align = options.align || "end";
    let left = align === "start" ? rect.left : rect.right - width;
    left = Math.max(margin, Math.min(left, window.innerWidth - width - margin));
    let top = rect.bottom + margin;

    element.style.left = `${left}px`;
    element.style.right = "auto";
    element.style.top = `${top}px`;
    element.style.position = "fixed";

    const elementRect = element.getBoundingClientRect();
    if (elementRect.bottom > window.innerHeight - margin) {
        top = Math.max(margin, rect.top - elementRect.height - margin);
        element.style.top = `${top}px`;
    }
}

function toggleFloatingMenu(anchor, menu) {
    if (!anchor || !menu) {
        return;
    }
    const willOpen = menu.hidden || menu.dataset.open !== "true";
    closeRowMenus(menu);
    if (!willOpen) {
        menu.hidden = true;
        menu.dataset.open = "false";
        return;
    }
    menu.hidden = false;
    menu.dataset.open = "true";
    positionFloatingElement(anchor, menu, { width: 180 });
}

function readRecents() {
    try {
        const stored = JSON.parse(localStorage.getItem(recentsKey) || "[]");
        return Array.isArray(stored) ? stored : [];
    } catch {
        return [];
    }
}

function writeRecents(recents) {
    try {
        localStorage.setItem(recentsKey, JSON.stringify(recents.slice(0, 8)));
    } catch {
        // Browser storage may be disabled; the dashboard still works without it.
    }
}

function addRecent(item) {
    const now = new Date().toISOString();
    const recents = readRecents().filter((recent) => `${recent.kind}:${recent.label}` !== `${item.kind}:${item.label}`);
    recents.unshift({
        ...item,
        at: now
    });
    writeRecents(recents);
}

function formatRecentTime(value) {
    if (!value) {
        return "";
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return "";
    }
    return date.toLocaleString([], {
        month: "short",
        day: "numeric",
        hour: "numeric",
        minute: "2-digit"
    });
}

function formatDateTime(value) {
    if (!value) {
        return "--";
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return "--";
    }
    return date.toLocaleString([], {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "numeric",
        minute: "2-digit"
    });
}

function projectTypeLabel(value) {
    const labels = ["Admin", "Club", "Competition", "Course", "Student", "Group", "Lab", "Custom"];
    return labels[value] || `Type ${value}`;
}

function projectRoleLabel(value) {
    const labels = ["Viewer", "Operator", "Developer", "Manager", "Owner"];
    return labels[value] || `Role ${value}`;
}

function projectSubjectLabel(value) {
    return value === 1 ? "Group" : "User";
}

function groupTypeLabel(value) {
    const labels = ["admin", "club", "competition", "course", "course_role", "student_group", "project", "custom"];
    return labels[value] || "custom";
}

function membershipRoleLabel(value) {
    const labels = ["member", "manager", "owner"];
    return labels[value] || "member";
}

function projectSubjectTypeValue(value) {
    return Number(value) === 1 ? "group" : "user";
}

function subjectDisplay(subject, fallback) {
    if (!subject) {
        return fallback;
    }
    return subject.label || subject.display_name || subject.name || subject.username || subject.slug || fallback;
}

function renderProjectMemberships(memberships) {
    currentProjectMemberships = memberships || [];
    const list = dashboard.querySelector("[data-project-membership-list]");
    if (!list) {
        return;
    }

    const visibleMemberships = currentProjectMemberships.filter((membership) => projectSubjectTypeValue(membership.subject_type) === selectedProjectAccessType);
    list.replaceChildren();
    if (visibleMemberships.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-state";
        empty.textContent = selectedProjectAccessType === "group" ? "No groups assigned to this project." : "No users assigned to this project.";
        list.append(empty);
        return;
    }

    for (const membership of visibleMemberships) {
        const row = document.createElement("div");
        row.className = "compact-list-row action-list-row";

        const text = document.createElement("div");
        const primary = document.createElement("strong");
        primary.textContent = subjectDisplay(membership.subject, `${projectSubjectLabel(membership.subject_type)} ${membership.subject_id}`);
        const secondary = document.createElement("span");
        secondary.textContent = `${projectRoleLabel(membership.project_role)} · ${membership.subject?.meta || projectSubjectLabel(membership.subject_type)}`;
        text.append(primary, secondary);

        const actions = document.createElement("div");
        actions.className = "row-actions";
        const menuButton = document.createElement("button");
        menuButton.className = "icon-button row-menu-button";
        menuButton.type = "button";
        menuButton.textContent = "⋯";
        menuButton.title = "Member actions";
        const menu = document.createElement("div");
        menu.className = "row-menu";
        menu.hidden = true;
        for (const roleName of ["viewer", "operator", "developer", "manager", "owner"]) {
            const roleAction = document.createElement("button");
            roleAction.type = "button";
            roleAction.textContent = `Set ${roleName}`;
            roleAction.disabled = projectRoleLabel(membership.project_role).toLowerCase() === roleName;
            roleAction.addEventListener("click", () => updateProjectMembershipRole(membership, roleName));
            menu.append(roleAction);
        }
        const removeAction = document.createElement("button");
        removeAction.type = "button";
        removeAction.className = "danger-action";
        removeAction.textContent = "Remove";
        removeAction.addEventListener("click", () => deleteProjectMembership(membership));
        menuButton.addEventListener("click", (event) => {
            event.stopPropagation();
            toggleFloatingMenu(menuButton, menu);
        });
        menu.addEventListener("click", (event) => event.stopPropagation());
        menu.append(removeAction);
        actions.append(menuButton, menu);

        row.append(text, actions);
        list.append(row);
    }
}

function populateProjectSubjectOptions() {
    const userOptions = document.querySelector("[data-project-user-options]");
    const userInput = document.querySelector("[data-project-user-input]");
    const groupSelect = document.querySelector("[data-project-group-select]");
    const userFields = document.querySelector("[data-project-user-fields]");
    const groupFields = document.querySelector("[data-project-group-fields]");
    const hiddenType = document.querySelector("[data-project-subject-type]");
    const kicker = document.querySelector("[data-project-member-kicker]");
    const title = document.querySelector("[data-project-member-title]");
    const copy = document.querySelector("[data-project-member-copy]");
    if (!userOptions || !groupSelect || !hiddenType || !userFields || !groupFields) {
        return;
    }

    hiddenType.value = selectedProjectAccessType;
    userFields.hidden = selectedProjectAccessType !== "user";
    groupFields.hidden = selectedProjectAccessType !== "group";
    if (kicker) {
        kicker.textContent = selectedProjectAccessType === "group" ? "Project group access" : "Direct user access";
    }
    if (title) {
        title.textContent = selectedProjectAccessType === "group" ? "Add group" : "Add user";
    }
    if (copy) {
        copy.textContent = selectedProjectAccessType === "group"
            ? "Assign a local group to this project, then manage that group membership from Access."
            : "Resolve a local user or sync an IPA user by username, email, or display name.";
    }
    if (userInput) {
        userInput.value = "";
    }
    userOptions.replaceChildren();
    groupSelect.replaceChildren();

    for (const item of cachedUsers) {
        const option = document.createElement("option");
        option.value = item.username;
        option.label = `${item.displayName || item.display_name || item.username}${item.email ? ` · ${item.email}` : ""}`;
        userOptions.append(option);
    }

    for (const group of cachedGroups) {
        const option = document.createElement("option");
        option.value = group.slug || group.name;
        option.textContent = `${group.name || group.slug} (${group.slug})`;
        groupSelect.append(option);
    }
}

function setProjectAccessType(type) {
    selectedProjectAccessType = type === "group" ? "group" : "user";
    document.querySelectorAll("[data-project-access-tab]").forEach((tab) => {
        tab.classList.toggle("is-active", tab.dataset.projectAccessTab === selectedProjectAccessType);
    });
    const label = selectedProjectAccessType === "group" ? "group" : "user";
    document.querySelectorAll("[data-open-project-member-popover], [data-project-member-submit]").forEach((button) => {
        button.textContent = `Add ${label}`;
    });
    closeProjectMemberPopover();
    populateProjectSubjectOptions();
    renderProjectMemberships(currentProjectMemberships);
}

function renderRecents() {
    const list = dashboard && dashboard.querySelector("[data-recent-list]");
    if (!list) {
        return;
    }

    const seen = new Set();
    const recents = readRecents()
        .filter((item) => {
            const key = `${item.kind}:${item.label}`;
            if (seen.has(key)) {
                return false;
            }
            seen.add(key);
            return true;
        })
        .slice(0, 5);

    list.replaceChildren();
    if (recents.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-state";
        empty.textContent = "Open a tab or create a project to populate recents.";
        list.append(empty);
        return;
    }

    for (const recent of recents) {
        const row = document.createElement("div");
        row.className = "recent-row";
        row.tabIndex = 0;
        row.role = "button";

        const text = document.createElement("div");
        const title = document.createElement("strong");
        title.textContent = recent.label;
        const detail = document.createElement("span");
        detail.textContent = `${recent.kind} · ${recent.detail || "Dashboard"}`;
        text.append(title, detail);

        const time = document.createElement("time");
        time.textContent = formatRecentTime(recent.at);
        if (recent.at) {
            time.dateTime = recent.at;
        }

        row.append(text, time);
        row.addEventListener("click", () => navigateTo(recent.target || { view: recent.kind === "Project" ? "projects" : recent.label.toLowerCase() }));
        row.addEventListener("keydown", (event) => {
            if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                row.click();
            }
        });
        list.append(row);
    }
}

async function selectProject(slug, options = {}) {
    if (!slug) {
        return;
    }
    try {
        const project = await apiFetch(`/api/v1/projects/${encodeURIComponent(slug)}`);
        selectedProject = project;
        const memberships = await apiFetch(`/api/v1/projects/${project.id}/memberships`);

        dashboard.querySelector("[data-project-detail]").hidden = false;
        setText("[data-project-detail-name]", project.name || project.slug);
        setText("[data-project-detail-slug]", project.slug);
        setText("[data-project-detail-description]", project.description || "No description");
        setText("[data-project-detail-status]", project.is_active ? "Active" : "Archived");
        setText("[data-project-detail-id]", project.id);
        setText("[data-project-detail-uuid]", project.uuid || "--");
        setText("[data-project-detail-org]", project.organization_id ? `Organization #${project.organization_id}` : "--");
        setText("[data-project-detail-type]", projectTypeLabel(project.project_type));
        setText("[data-project-detail-created]", formatDateTime(project.created_at));
        setText("[data-project-detail-updated]", formatDateTime(project.updated_at));
        setText("[data-project-member-count]", memberships.length);
        populateProjectSubjectOptions();
        renderProjectMemberships(memberships);

        dashboard.querySelectorAll("[data-project-list] .data-table-row").forEach((row) => {
            row.classList.toggle("is-selected", row.dataset.projectSlug === project.slug);
        });

        if (options.updateURL !== false) {
            history.pushState(null, "", dashboardURL({ view: "projects", project: project.slug }));
        }
        addRecent({
            kind: "Project",
            label: project.name || project.slug,
            detail: project.slug,
            target: { view: "projects", resourceType: "project", resourceID: project.slug }
        });
    } catch (error) {
        if (error.status === 401) {
            redirectToLogin();
            return;
        }
        const fallbackView = viewTitles[lastAuthorizedView] ? lastAuthorizedView : "overview";
        switchView(fallbackView, { updateURL: false, authorized: false });
        history.replaceState(null, "", dashboardURL({ view: fallbackView, project: null }));
        const label = error.status === 404 ? `Project ${slug} was not found` : `Unauthorized to view project ${slug}`;
        showToast(label, "warning");
    }
}

function navigateTo(target = {}) {
    const view = target.view || "overview";
    if (!viewTitles[view]) {
        switchView(lastAuthorizedView || "overview", { authorized: false });
        showToast(`Unauthorized to view ${target.label || "resource"}`, "warning");
        return;
    }
    switchView(view, {
        projectSlug: target.resourceType === "project" ? target.resourceID : null
    });
    if (target.resourceType === "project") {
        selectProject(target.resourceID);
    }
}

function closeProjectDetail() {
    const detail = dashboard && dashboard.querySelector("[data-project-detail]");
    if (!detail) {
        return;
    }
    detail.hidden = true;
    selectedProject = null;
    dashboard.querySelectorAll("[data-project-list] .data-table-row").forEach((row) => {
        row.classList.remove("is-selected");
    });
    if (currentView === "projects") {
        history.pushState(null, "", dashboardURL({ view: "projects", project: null }));
    }
}

function openProjectMemberPopover(anchor) {
    if (!projectMemberPopover) {
        return;
    }
    const message = projectMemberPopover.querySelector("[data-project-member-message]");
    if (message) {
        message.textContent = "";
    }
    populateProjectSubjectOptions();
    projectMemberPopover.hidden = false;
    positionFloatingElement(anchor, projectMemberPopover, { width: 340 });
    if (selectedProjectAccessType === "group") {
        projectMemberPopover.querySelector("[data-project-group-select]")?.focus();
    } else {
        projectMemberPopover.querySelector("[data-project-user-input]")?.focus();
    }
}

function closeProjectMemberPopover() {
    if (projectMemberPopover) {
        projectMemberPopover.hidden = true;
        const message = projectMemberPopover.querySelector("[data-project-member-message]");
        if (message) {
            message.textContent = "";
        }
    }
}

function renderCompactList(selector, items, getPrimary, getSecondary) {
    const list = dashboard.querySelector(selector);
    if (!list) {
        return;
    }

    list.replaceChildren();
    if (!items || items.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-state";
        empty.textContent = "No data loaded.";
        list.append(empty);
        return;
    }

    for (const item of items.slice(0, 12)) {
        const row = document.createElement("div");
        row.className = "compact-list-row";

        const primary = document.createElement("strong");
        primary.textContent = getPrimary(item);
        const secondary = document.createElement("span");
        secondary.textContent = getSecondary(item);

        row.append(primary, secondary);
        list.append(row);
    }
}

function renderPermissions(permissions) {
    const list = dashboard.querySelector("[data-permission-list]");
    if (!list) {
        return;
    }

    list.replaceChildren();
    for (const permission of (permissions || []).slice(0, 48)) {
        const pill = document.createElement("span");
        pill.className = "permission-pill";
        pill.textContent = permission.name;
        list.append(pill);
    }
}

function renderAccess(access) {
    cachedGroups = access.groups || [];
    cachedRoles = access.roles || [];
    cachedPermissions = access.permissions || [];
    cachedRoleBindings = access.roleBindings || [];
    renderGroups(cachedGroups);
    renderRoles(cachedRoles);
    renderAccessGrants(cachedRoleBindings);
    renderPermissions(cachedPermissions);
    populateGroupParentSelect();
    populateAccessGrantOptions();
}

function renderRoles(roles) {
    const list = dashboard.querySelector("[data-role-list]");
    if (!list) {
        return;
    }

    list.replaceChildren();
    if (!roles || roles.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-state";
        empty.textContent = "No roles loaded.";
        list.append(empty);
        return;
    }

    for (const role of roles) {
        const row = document.createElement("div");
        row.className = "compact-list-row action-list-row";

        const text = document.createElement("div");
        const primary = document.createElement("strong");
        primary.textContent = role.name;
        const secondary = document.createElement("span");
        const bits = [
            role.description || (role.is_system_role ? "System role" : "Custom role"),
            role.is_system_role ? "system" : "custom",
            `${role.permission_count || 0} permissions`
        ];
        secondary.textContent = bits.filter(Boolean).join(" · ");
        text.append(primary, secondary);

        const actions = document.createElement("div");
        actions.className = "row-actions";
        const menuButton = document.createElement("button");
        menuButton.className = "icon-button row-menu-button";
        menuButton.type = "button";
        menuButton.textContent = "⋯";
        menuButton.title = "Role actions";
        const menu = document.createElement("div");
        menu.className = "row-menu";
        menu.hidden = true;
        const manageAction = document.createElement("button");
        manageAction.type = "button";
        manageAction.textContent = role.is_system_role ? "View permissions" : "Manage permissions";
        manageAction.addEventListener("click", () => openRolePermissionsModal(role));
        menuButton.addEventListener("click", (event) => {
            event.stopPropagation();
            toggleFloatingMenu(menuButton, menu);
        });
        menu.addEventListener("click", (event) => event.stopPropagation());
        menu.append(manageAction);
        actions.append(menuButton, menu);

        row.append(text, actions);
        list.append(row);
    }
}

function renderGroups(groups) {
    const list = dashboard.querySelector("[data-group-list]");
    if (!list) {
        return;
    }

    list.replaceChildren();
    if (!groups || groups.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-state";
        empty.textContent = "No groups loaded.";
        list.append(empty);
        return;
    }

    for (const group of groups) {
        const row = document.createElement("div");
        row.className = "compact-list-row action-list-row";

        const text = document.createElement("div");
        const primary = document.createElement("strong");
        primary.textContent = group.name || group.slug;
        const secondary = document.createElement("span");
        const syncLabel = group.sync_source === "ldap"
            ? `LDAP${group.sync_membership ? " synced" : " linked"}${group.external_id ? `: ${group.external_id}` : ""}`
            : "local";
        const bits = [
            group.group_type_label || groupTypeLabel(group.group_type),
            group.slug,
            syncLabel,
            `${group.member_count || 0} members`,
            `${group.role_binding_count || 0} role bindings`
        ];
        secondary.textContent = bits.filter(Boolean).join(" · ");
        text.append(primary, secondary);

        const actions = document.createElement("div");
        actions.className = "row-actions";
        const menuButton = document.createElement("button");
        menuButton.className = "icon-button row-menu-button";
        menuButton.type = "button";
        menuButton.textContent = "⋯";
        menuButton.title = "Group actions";
        const menu = document.createElement("div");
        menu.className = "row-menu";
        menu.hidden = true;
        const manageAction = document.createElement("button");
        manageAction.type = "button";
        manageAction.textContent = "Manage members";
        manageAction.addEventListener("click", () => openGroupMembersModal(group));
        menuButton.addEventListener("click", (event) => {
            event.stopPropagation();
            toggleFloatingMenu(menuButton, menu);
        });
        menu.addEventListener("click", (event) => event.stopPropagation());
        menu.append(manageAction);
        actions.append(menuButton, menu);

        row.append(text, actions);
        list.append(row);
    }
}

function populateGroupParentSelect() {
    const select = document.querySelector("[data-group-parent-select]");
    if (!select) {
        return;
    }
    const current = select.value;
    select.replaceChildren();
    const empty = document.createElement("option");
    empty.value = "";
    empty.textContent = "None";
    select.append(empty);
    for (const group of cachedGroups) {
        const option = document.createElement("option");
        option.value = group.id;
        option.textContent = `${group.name || group.slug} (${group.slug})`;
        select.append(option);
    }
    select.value = current;
}

function populateAccessGrantOptions() {
    const roleSelect = document.querySelector("[data-grant-role-select]");
    if (roleSelect) {
        const current = roleSelect.value;
        roleSelect.replaceChildren();
        for (const role of cachedRoles) {
            const option = document.createElement("option");
            option.value = role.id;
            option.textContent = role.name;
            roleSelect.append(option);
        }
        roleSelect.value = current;
    }

    const groupSelect = document.querySelector("[data-grant-group-select]");
    if (groupSelect) {
        const current = groupSelect.value;
        groupSelect.replaceChildren();
        for (const group of cachedGroups) {
            const option = document.createElement("option");
            option.value = group.id;
            option.textContent = `${group.name || group.slug} (${group.slug})`;
            groupSelect.append(option);
        }
        groupSelect.value = current;
    }

    const userSelect = document.querySelector("[data-grant-user-select]");
    if (userSelect) {
        const current = userSelect.value;
        userSelect.replaceChildren();
        for (const user of cachedUsers) {
            const option = document.createElement("option");
            option.value = user.id;
            option.textContent = `${user.displayName || user.display_name || user.username} (${user.username})`;
            userSelect.append(option);
        }
        userSelect.value = current;
    }
    updateGrantSubjectFields();
}

function renderAccessGrants(bindings) {
    const list = dashboard.querySelector("[data-access-grant-list]");
    if (!list) {
        return;
    }
    list.replaceChildren();
    if (!bindings || bindings.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-state";
        empty.textContent = "No access grants assigned.";
        list.append(empty);
        return;
    }
    for (const binding of bindings) {
        const row = document.createElement("div");
        row.className = "compact-list-row action-list-row";

        const text = document.createElement("div");
        const primary = document.createElement("strong");
        primary.textContent = binding.role?.name || `Role ${binding.role_id}`;
        const secondary = document.createElement("span");
        secondary.textContent = `${bindingSubjectLabel(binding)} · ${bindingScopeLabel(binding)}`;
        text.append(primary, secondary);

        const actions = document.createElement("div");
        actions.className = "row-actions";
        const button = document.createElement("button");
        button.className = "icon-button row-menu-button";
        button.type = "button";
        button.textContent = "⋯";
        button.title = "Access grant actions";
        const menu = document.createElement("div");
        menu.className = "row-menu";
        menu.hidden = true;
        const remove = document.createElement("button");
        remove.type = "button";
        remove.className = "danger-action";
        remove.textContent = "Remove";
        remove.addEventListener("click", () => deleteAccessGrant(binding));
        button.addEventListener("click", (event) => {
            event.stopPropagation();
            toggleFloatingMenu(button, menu);
        });
        menu.addEventListener("click", (event) => event.stopPropagation());
        menu.append(remove);
        actions.append(button, menu);

        row.append(text, actions);
        list.append(row);
    }
}

function roleByID(roles = []) {
    return new Map(roles.map((role) => [role.id, role]));
}

function bindingScopeLabel(binding) {
    if (binding.scope_type_label) {
        const label = binding.scope_type_label[0].toUpperCase() + binding.scope_type_label.slice(1);
        return binding.scope_id ? `${label} #${binding.scope_id}` : label;
    }
    const scopeNames = ["Global", "Organization", "Course", "Project", "Group", "Resource"];
    const name = scopeNames[binding.scope_type] || "Scoped";
    return binding.scope_id ? `${name} #${binding.scope_id}` : name;
}

function bindingSubjectLabel(binding) {
    if (binding.subject) {
        return subjectDisplay(binding.subject, `${binding.subject_type_label || "Subject"} #${binding.subject_id}`);
    }
    return binding.subject_type === 1 ? `Group #${binding.subject_id}` : `User #${binding.subject_id}`;
}

function renderMyAccess(access) {
    const groups = access.groups || [];
    const roles = access.roles || [];
    const bindings = access.roleBindings || [];
    const rolesByID = roleByID(roles);

    setText("[data-user-groups]", groups.length);
    setText("[data-user-role-count]", roles.length);
    setText("[data-user-binding-count]", bindings.length);
    renderCompactList(
        "[data-my-group-list]",
        groups,
        (group) => group.name || group.slug,
        (group) => group.slug || `Group #${group.id}`
    );
    renderCompactList(
        "[data-my-role-list]",
        roles,
        (role) => role.name,
        (role) => role.description || "Assigned role"
    );
    renderCompactList(
        "[data-my-binding-list]",
        bindings,
        (binding) => rolesByID.get(binding.role_id)?.name || `Role #${binding.role_id}`,
        (binding) => `${bindingSubjectLabel(binding)} · ${bindingScopeLabel(binding)}`
    );
}

function renderUsers(users) {
    cachedUsers = users || [];
    const list = dashboard.querySelector("[data-user-list]");
    if (!list) {
        return;
    }
    list.replaceChildren();
    if (!users || users.length === 0) {
        const empty = document.createElement("div");
        empty.className = "empty-state";
        empty.textContent = "No users loaded.";
        list.append(empty);
        return;
    }
    for (const user of users) {
        const row = document.createElement("div");
        row.className = "data-table-row people-table-row";
        const name = document.createElement("div");
        const title = document.createElement("strong");
        title.textContent = user.displayName || user.username;
        const meta = document.createElement("span");
        meta.textContent = user.username;
        name.append(title, meta);
        const email = document.createElement("span");
        email.textContent = user.email || "No email";
        const source = document.createElement("span");
        source.textContent = user.authSource || user.auth_source || "local";
        row.append(name, email, source);
        list.append(row);
    }
    populateAccessGrantOptions();
}

function redirectToLogin() {
    const params = new URLSearchParams({
        redirect: `${window.location.pathname}${window.location.search}`,
        toast: "Please sign in first",
        tone: "warning"
    });
    window.location.href = `/login?${params.toString()}`;
}

async function loadDashboard() {
    if (!dashboard) {
        return;
    }

    try {
        const [summary, projects, access, myAccess, users] = await Promise.all([
            apiFetch("/api/v1/system/summary"),
            apiFetch("/api/v1/projects"),
            apiFetch("/api/v1/system/access"),
            apiFetch("/api/v1/users/me/access"),
            apiFetch("/api/v1/users")
        ]);
        renderSummary(summary);
        renderProjects(projects);
        renderAccess(access);
        renderMyAccess(myAccess);
        renderUsers(users);
        renderRecents(projects);
        applyURLState();
    } catch (error) {
        if (error.status === 401) {
            redirectToLogin();
            return;
        }
        setText("[data-user-name]", "Not signed in");
        setText("[data-user-meta]", "Open a session to load live control-plane data.");
        const signInAction = dashboard.querySelector("[data-sign-in-action]");
        if (signInAction) {
            signInAction.hidden = false;
        }
        if (accountMenu) {
            accountMenu.hidden = true;
        }
        document.querySelectorAll("[data-open-project-modal], [data-project-form] input, [data-project-form] button[type='submit']").forEach((control) => {
            control.disabled = true;
        });
        document.querySelectorAll("[data-open-role-modal], [data-role-form] input, [data-role-form] button[type='submit']").forEach((control) => {
            control.disabled = true;
        });
        document.querySelectorAll("[data-open-grant-modal], [data-grant-form] input, [data-grant-form] select, [data-grant-form] button[type='submit']").forEach((control) => {
            control.disabled = true;
        });
    }
}

function applyURLState() {
    const params = new URLSearchParams(window.location.search);
    const view = params.get("view") || "overview";
    switchView(viewTitles[view] ? view : "overview", { updateURL: false });
    if (params.get("project")) {
        selectProject(params.get("project"), { updateURL: false });
    }
}

function openProjectModal() {
    if (!projectModal) {
        return;
    }
    projectModal.hidden = false;
    projectModal.querySelector("input")?.focus();
}

function closeProjectModal() {
    if (!projectModal) {
        return;
    }
    projectModal.hidden = true;
}

function bindProjectModal() {
    document.querySelectorAll("[data-open-project-modal]").forEach((button) => {
        button.addEventListener("click", openProjectModal);
    });

    document.querySelectorAll("[data-close-project-modal]").forEach((button) => {
        button.addEventListener("click", closeProjectModal);
    });

    projectModal?.addEventListener("click", (event) => {
        if (event.target === projectModal) {
            closeProjectModal();
        }
    });

    document.querySelectorAll("[data-close-project-detail]").forEach((button) => {
        button.addEventListener("click", closeProjectDetail);
    });

    document.querySelectorAll("[data-project-access-tab]").forEach((button) => {
        button.addEventListener("click", () => setProjectAccessType(button.dataset.projectAccessTab));
    });

    document.querySelector("[data-open-project-member-popover]")?.addEventListener("click", (event) => {
        event.stopPropagation();
        openProjectMemberPopover(event.currentTarget);
    });

    document.querySelector("[data-close-project-member-popover]")?.addEventListener("click", closeProjectMemberPopover);
    projectMemberPopover?.addEventListener("click", (event) => event.stopPropagation());

    const detailMenuToggle = document.querySelector("[data-project-actions-toggle]");
    const detailMenu = document.querySelector("[data-project-actions-menu]");
    detailMenuToggle?.addEventListener("click", (event) => {
        event.stopPropagation();
        toggleFloatingMenu(detailMenuToggle, detailMenu);
    });
    detailMenu?.addEventListener("click", (event) => event.stopPropagation());

    document.querySelector("[data-project-delete]")?.addEventListener("click", () => {
        if (selectedProject) {
            deleteProject(selectedProject);
        }
    });

    document.addEventListener("keydown", (event) => {
        if (event.key === "Escape" && dashboard?.querySelector("[data-project-detail]")?.hidden === false) {
            if (projectMemberPopover && !projectMemberPopover.hidden) {
                closeProjectMemberPopover();
                return;
            }
            closeProjectDetail();
        }
    });
}

function bindGenericModal(openSelector, closeSelector, modal) {
    if (!modal) {
        return;
    }
    document.querySelectorAll(openSelector).forEach((button) => {
        button.addEventListener("click", () => {
            modal.hidden = false;
            modal.querySelector("input")?.focus();
        });
    });
    document.querySelectorAll(closeSelector).forEach((button) => {
        button.addEventListener("click", () => {
            modal.hidden = true;
        });
    });
    modal?.addEventListener("click", (event) => {
        if (event.target === modal) {
            modal.hidden = true;
        }
    });
}

function updateGroupLDAPFields() {
    const select = document.querySelector("[data-group-sync-source]");
    const fields = document.querySelector("[data-group-ldap-fields]");
    if (!select || !fields) {
        return;
    }
    fields.hidden = select.value !== "ldap";
}

function updateGrantSubjectFields() {
    const type = document.querySelector("[data-grant-subject-type]")?.value || "group";
    const groupFields = document.querySelector("[data-grant-group-fields]");
    const userFields = document.querySelector("[data-grant-user-fields]");
    if (groupFields) {
        groupFields.hidden = type !== "group";
    }
    if (userFields) {
        userFields.hidden = type !== "user";
    }
}

function bindAccountMenu() {
    const toggle = document.querySelector("[data-account-toggle]");
    const logout = document.querySelector("[data-logout-action]");

    toggle?.addEventListener("click", () => {
        if (accountDropdown) {
            accountDropdown.hidden = !accountDropdown.hidden;
        }
    });

    document.addEventListener("click", (event) => {
        if (!accountMenu || accountMenu.hidden || accountMenu.contains(event.target)) {
            return;
        }
        if (accountDropdown) {
            accountDropdown.hidden = true;
        }
    });

    logout?.addEventListener("click", async () => {
        await fetch("/api/v1/auth/logout", {
            method: "POST",
            credentials: "same-origin"
        });
        window.location.href = "/login";
    });
}

async function deleteProject(project) {
    if (!project || !window.confirm(`Delete project "${project.name || project.slug}"?`)) {
        return;
    }
    try {
        await apiFetch(`/api/v1/projects/${encodeURIComponent(project.slug)}`, {
            method: "DELETE"
        });
        showToast("Project deleted", "success");
        if (selectedProject && selectedProject.slug === project.slug) {
            closeProjectDetail();
        }
        await loadDashboard();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

async function deleteProjectMembership(membership) {
    if (!selectedProject || !membership) {
        return;
    }
    try {
        await apiFetch(`/api/v1/projects/${selectedProject.id}/memberships/${membership.id}`, {
            method: "DELETE"
        });
        showToast("Project member removed", "success");
        await selectProject(selectedProject.slug, { updateURL: false });
    } catch (error) {
        showToast(error.message, "warning");
    }
}

async function updateProjectMembershipRole(membership, projectRole) {
    if (!selectedProject || !membership || !projectRole) {
        return;
    }
    try {
        await apiFetch(`/api/v1/projects/${selectedProject.id}/memberships/${membership.id}`, {
            method: "PATCH",
            body: JSON.stringify({ projectRole })
        });
        showToast("Project member role updated", "success");
        await selectProject(selectedProject.slug, { updateURL: false });
    } catch (error) {
        showToast(error.message, "warning");
    }
}

function selectedOptionValues(select) {
    return Array.from(select?.selectedOptions || []).map((option) => Number(option.value)).filter(Boolean);
}

async function loadGroupMembershipModal() {
    if (!selectedGroup || !groupMembersModal) {
        return;
    }

    const memberships = await apiFetch(`/api/v1/groups/${selectedGroup.id}/memberships`);
    selectedGroup.memberships = memberships;
    const memberUserIDs = new Set(memberships.map((membership) => membership.user_id));

    const available = groupMembersModal.querySelector("[data-available-users]");
    const assigned = groupMembersModal.querySelector("[data-group-users]");
    available.replaceChildren();
    assigned.replaceChildren();

    for (const user of cachedUsers) {
        const option = document.createElement("option");
        option.value = user.id;
        option.textContent = `${user.displayName || user.display_name || user.username} (${user.username})`;
        if (memberUserIDs.has(user.id)) {
            const membership = memberships.find((item) => item.user_id === user.id);
            option.dataset.membershipID = membership?.id || "";
            option.textContent = `${option.textContent} · ${membership?.membership_role_label || membershipRoleLabel(membership?.membership_role)}`;
            assigned.append(option);
        } else {
            available.append(option);
        }
    }
}

async function openGroupMembersModal(group) {
    selectedGroup = group;
    if (!groupMembersModal) {
        return;
    }
    const title = groupMembersModal.querySelector("[data-group-members-title]");
    if (title) {
        title.textContent = `Manage ${group.name || group.slug}`;
    }
    groupMembersModal.hidden = false;
    try {
        await loadGroupMembershipModal();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

function closeGroupMembersModal() {
    if (groupMembersModal) {
        groupMembersModal.hidden = true;
    }
    selectedGroup = null;
}

async function addUsersToSelectedGroup(userIDs) {
    if (!selectedGroup || userIDs.length === 0) {
        return;
    }
    const membershipRole = groupMembersModal.querySelector("[data-group-membership-role]")?.value || "member";
    try {
        await Promise.all(userIDs.map((userID) => apiFetch(`/api/v1/groups/${selectedGroup.id}/memberships`, {
            method: "POST",
            body: JSON.stringify({ userID, membershipRole })
        })));
        await loadGroupMembershipModal();
        showToast("Group members updated", "success");
        await loadDashboard();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

async function removeUsersFromSelectedGroup(userIDs) {
    if (!selectedGroup || userIDs.length === 0) {
        return;
    }
    const memberships = selectedGroup.memberships || [];
    const membershipIDs = memberships
        .filter((membership) => userIDs.includes(membership.user_id))
        .map((membership) => membership.id);
    try {
        await Promise.all(membershipIDs.map((membershipID) => apiFetch(`/api/v1/groups/${selectedGroup.id}/memberships/${membershipID}`, {
            method: "DELETE"
        })));
        await loadGroupMembershipModal();
        showToast("Group members updated", "success");
        await loadDashboard();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

async function updateSelectedGroupMemberRoles(userIDs) {
    if (!selectedGroup || userIDs.length === 0) {
        return;
    }
    const memberships = selectedGroup.memberships || [];
    const membershipRole = groupMembersModal.querySelector("[data-group-member-edit-role]")?.value || "member";
    const membershipIDs = memberships
        .filter((membership) => userIDs.includes(membership.user_id))
        .map((membership) => membership.id);
    try {
        await Promise.all(membershipIDs.map((membershipID) => apiFetch(`/api/v1/groups/${selectedGroup.id}/memberships/${membershipID}`, {
            method: "PATCH",
            body: JSON.stringify({ membershipRole })
        })));
        await loadGroupMembershipModal();
        showToast("Group member roles updated", "success");
        await loadDashboard();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

async function deleteAccessGrant(binding) {
    if (!binding) {
        return;
    }
    try {
        await apiFetch(`/api/v1/role-bindings/${binding.id}`, {
            method: "DELETE"
        });
        showToast("Access grant removed", "success");
        await loadDashboard();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

function bindGroupMembersModal() {
    if (!groupMembersModal) {
        return;
    }
    groupMembersModal.querySelector("[data-close-group-members-modal]")?.addEventListener("click", closeGroupMembersModal);
    groupMembersModal.addEventListener("click", (event) => {
        if (event.target === groupMembersModal) {
            closeGroupMembersModal();
        }
    });
    groupMembersModal.querySelector("[data-transfer-add]")?.addEventListener("click", () => {
        addUsersToSelectedGroup(selectedOptionValues(groupMembersModal.querySelector("[data-available-users]")));
    });
    groupMembersModal.querySelector("[data-transfer-add-all]")?.addEventListener("click", () => {
        addUsersToSelectedGroup(Array.from(groupMembersModal.querySelector("[data-available-users]")?.options || []).map((option) => Number(option.value)));
    });
    groupMembersModal.querySelector("[data-transfer-remove]")?.addEventListener("click", () => {
        removeUsersFromSelectedGroup(selectedOptionValues(groupMembersModal.querySelector("[data-group-users]")));
    });
    groupMembersModal.querySelector("[data-transfer-remove-all]")?.addEventListener("click", () => {
        removeUsersFromSelectedGroup(Array.from(groupMembersModal.querySelector("[data-group-users]")?.options || []).map((option) => Number(option.value)));
    });
    groupMembersModal.querySelector("[data-group-member-role-update]")?.addEventListener("click", () => {
        updateSelectedGroupMemberRoles(selectedOptionValues(groupMembersModal.querySelector("[data-group-users]")));
    });
}

async function loadRolePermissionsModal() {
    if (!selectedRole || !rolePermissionsModal) {
        return;
    }

    const grants = await apiFetch(`/api/v1/roles/${selectedRole.id}/permissions`);
    selectedRole.permissionGrants = grants;
    const assignedPermissionIDs = new Set(grants.map((grant) => grant.permission_id));

    const available = rolePermissionsModal.querySelector("[data-available-permissions]");
    const assigned = rolePermissionsModal.querySelector("[data-role-permissions]");
    available.replaceChildren();
    assigned.replaceChildren();

    for (const permission of cachedPermissions) {
        const option = document.createElement("option");
        option.value = permission.id;
        option.textContent = permission.name;
        if (assignedPermissionIDs.has(permission.id)) {
            assigned.append(option);
        } else {
            available.append(option);
        }
    }
}

async function openRolePermissionsModal(role) {
    selectedRole = role;
    if (!rolePermissionsModal) {
        return;
    }
    const title = rolePermissionsModal.querySelector("[data-role-permissions-title]");
    if (title) {
        title.textContent = `${role.is_system_role ? "View" : "Manage"} ${role.name}`;
    }
    const readOnly = Boolean(role.is_system_role);
    rolePermissionsModal.querySelectorAll("[data-permission-add], [data-permission-add-all], [data-permission-remove], [data-permission-remove-all]").forEach((button) => {
        button.disabled = readOnly;
    });
    rolePermissionsModal.hidden = false;
    try {
        await loadRolePermissionsModal();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

function closeRolePermissionsModal() {
    if (rolePermissionsModal) {
        rolePermissionsModal.hidden = true;
    }
    selectedRole = null;
}

async function addPermissionsToSelectedRole(permissionIDs) {
    if (!selectedRole || selectedRole.is_system_role || permissionIDs.length === 0) {
        return;
    }
    try {
        await Promise.all(permissionIDs.map((permissionID) => apiFetch(`/api/v1/roles/${selectedRole.id}/permissions`, {
            method: "POST",
            body: JSON.stringify({ permissionID })
        })));
        await loadRolePermissionsModal();
        showToast("Role permissions updated", "success");
        await loadDashboard();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

async function removePermissionsFromSelectedRole(permissionIDs) {
    if (!selectedRole || selectedRole.is_system_role || permissionIDs.length === 0) {
        return;
    }
    try {
        await Promise.all(permissionIDs.map((permissionID) => apiFetch(`/api/v1/roles/${selectedRole.id}/permissions/${permissionID}`, {
            method: "DELETE"
        })));
        await loadRolePermissionsModal();
        showToast("Role permissions updated", "success");
        await loadDashboard();
    } catch (error) {
        showToast(error.message, "warning");
    }
}

function bindRolePermissionsModal() {
    if (!rolePermissionsModal) {
        return;
    }
    rolePermissionsModal.querySelector("[data-close-role-permissions-modal]")?.addEventListener("click", closeRolePermissionsModal);
    rolePermissionsModal.addEventListener("click", (event) => {
        if (event.target === rolePermissionsModal) {
            closeRolePermissionsModal();
        }
    });
    rolePermissionsModal.querySelector("[data-permission-add]")?.addEventListener("click", () => {
        addPermissionsToSelectedRole(selectedOptionValues(rolePermissionsModal.querySelector("[data-available-permissions]")));
    });
    rolePermissionsModal.querySelector("[data-permission-add-all]")?.addEventListener("click", () => {
        addPermissionsToSelectedRole(Array.from(rolePermissionsModal.querySelector("[data-available-permissions]")?.options || []).map((option) => Number(option.value)));
    });
    rolePermissionsModal.querySelector("[data-permission-remove]")?.addEventListener("click", () => {
        removePermissionsFromSelectedRole(selectedOptionValues(rolePermissionsModal.querySelector("[data-role-permissions]")));
    });
    rolePermissionsModal.querySelector("[data-permission-remove-all]")?.addEventListener("click", () => {
        removePermissionsFromSelectedRole(Array.from(rolePermissionsModal.querySelector("[data-role-permissions]")?.options || []).map((option) => Number(option.value)));
    });
}

function bindProjectForm() {
    const form = document.querySelector("[data-project-form]");
    if (!form) {
        return;
    }

    form.addEventListener("submit", async (event) => {
        event.preventDefault();

        const message = dashboard.querySelector("[data-project-message]");
        if (message) {
            message.textContent = "Creating project...";
        }

        const formData = new FormData(form);
        const payload = {
            name: formData.get("name"),
            slug: formData.get("slug"),
            description: formData.get("description")
        };

        try {
            const project = await apiFetch("/api/v1/projects", {
                method: "POST",
                body: JSON.stringify(payload)
            });
            addRecent({
                kind: "Project",
                label: project.name || payload.name,
                detail: project.slug || "Created project",
                target: {
                    view: "projects",
                    resourceType: "project",
                    resourceID: project.slug
                }
            });
            form.reset();
            closeProjectModal();
            if (message) {
                message.textContent = "Project created.";
            }
            await loadDashboard();
            navigateTo({
                view: "projects",
                resourceType: "project",
                resourceID: project.slug,
                label: project.name || project.slug
            });
        } catch (error) {
            if (message) {
                message.textContent = error.message;
            }
        }
    });
}

function bindManagementForms() {
    document.querySelector("[data-group-sync-source]")?.addEventListener("change", updateGroupLDAPFields);
    updateGroupLDAPFields();
    document.querySelector("[data-grant-subject-type]")?.addEventListener("change", updateGrantSubjectFields);
    updateGrantSubjectFields();

    document.querySelector("[data-user-form]")?.addEventListener("submit", async (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const data = new FormData(form);
        try {
            await apiFetch("/api/v1/users", {
                method: "POST",
                body: JSON.stringify({
                    username: data.get("username"),
                    displayName: data.get("displayName"),
                    email: data.get("email")
                })
            });
            form.reset();
            userModal.hidden = true;
            showToast("User created", "success");
            await loadDashboard();
        } catch (error) {
            showToast(error.message, "warning");
        }
    });

    document.querySelector("[data-group-form]")?.addEventListener("submit", async (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const data = new FormData(form);
        try {
            await apiFetch("/api/v1/groups", {
                method: "POST",
                body: JSON.stringify({
                    name: data.get("name"),
                    slug: data.get("slug"),
                    description: data.get("description"),
                    groupType: data.get("groupType"),
                    parentGroupID: data.get("parentGroupID") ? Number(data.get("parentGroupID")) : null,
                    syncSource: data.get("syncSource"),
                    externalID: data.get("externalID"),
                    syncMembership: data.get("syncSource") === "ldap" && data.get("syncMembership") === "on"
                })
            });
            form.reset();
            updateGroupLDAPFields();
            groupModal.hidden = true;
            showToast("Group created", "success");
            await loadDashboard();
        } catch (error) {
            showToast(error.message, "warning");
        }
    });

    document.querySelector("[data-role-form]")?.addEventListener("submit", async (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const data = new FormData(form);
        try {
            const role = await apiFetch("/api/v1/roles", {
                method: "POST",
                body: JSON.stringify({
                    name: data.get("name"),
                    description: data.get("description")
                })
            });
            form.reset();
            roleModal.hidden = true;
            showToast("Role created", "success");
            await loadDashboard();
            openRolePermissionsModal(role);
        } catch (error) {
            showToast(error.message, "warning");
        }
    });

    document.querySelector("[data-grant-form]")?.addEventListener("submit", async (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const data = new FormData(form);
        const subjectType = data.get("subjectType") || "group";
        const subjectID = subjectType === "user" ? Number(data.get("userID")) : Number(data.get("groupID"));
        const rawScopeID = data.get("scopeID");
        try {
            await apiFetch("/api/v1/role-bindings", {
                method: "POST",
                body: JSON.stringify({
                    roleID: Number(data.get("roleID")),
                    subjectType,
                    subjectID,
                    scopeType: data.get("scopeType"),
                    scopeID: rawScopeID ? Number(rawScopeID) : null
                })
            });
            form.reset();
            updateGrantSubjectFields();
            grantModal.hidden = true;
            showToast("Access grant added", "success");
            await loadDashboard();
        } catch (error) {
            showToast(error.message, "warning");
        }
    });

    document.querySelector("[data-project-membership-form]")?.addEventListener("submit", async (event) => {
        event.preventDefault();
        if (!selectedProject) {
            showToast("Choose a project first", "warning");
            return;
        }
        const form = event.currentTarget;
        const data = new FormData(form);
        const subjectRef = selectedProjectAccessType === "group" ? data.get("groupRef") : data.get("userRef");
        const message = form.querySelector("[data-project-member-message]");
        if (message) {
            const subjectKind = selectedProjectAccessType === "group" ? "group" : "user";
            message.textContent = selectedProjectAccessType === "user" ? "Resolving local user or IPA account..." : `Resolving ${subjectKind}...`;
        }
        try {
            await apiFetch(`/api/v1/projects/${selectedProject.id}/memberships`, {
                method: "POST",
                body: JSON.stringify({
                    subjectType: data.get("subjectType"),
                    subjectRef,
                    projectRole: data.get("projectRole")
                })
            });
            form.reset();
            closeProjectMemberPopover();
            showToast("Project member added", "success");
            await selectProject(selectedProject.slug, { updateURL: false });
        } catch (error) {
            if (message) {
                message.textContent = error.message;
            }
            showToast(error.message, "warning");
        }
    });
}

if (dashboard) {
    consumeInitialToast();
    bindTabs();
    bindAccountMenu();
    bindProjectModal();
    bindGenericModal("[data-open-user-modal]", "[data-close-user-modal]", userModal);
    bindGenericModal("[data-open-group-modal]", "[data-close-group-modal]", groupModal);
    bindGenericModal("[data-open-role-modal]", "[data-close-role-modal]", roleModal);
    bindGenericModal("[data-open-grant-modal]", "[data-close-grant-modal]", grantModal);
    bindGroupMembersModal();
    bindRolePermissionsModal();
    bindProjectForm();
    bindManagementForms();
    document.addEventListener("click", () => {
        closeRowMenus();
        closeProjectMemberPopover();
    });
    window.addEventListener("popstate", applyURLState);
    loadDashboard();
} else {
    consumeInitialToast();
}

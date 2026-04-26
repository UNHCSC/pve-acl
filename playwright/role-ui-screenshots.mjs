import { mkdir } from "node:fs/promises";
import { join } from "node:path";
import { chromium } from "@playwright/test";

const baseURL = process.env.BASE_URL || "http://127.0.0.1:8080";
const outputDir = join(process.cwd(), "screenshots");

const organizations = [
    { id: 1, name: "Lab", slug: "lab", parent_org_id: null, description: "Root organization" },
    { id: 2, name: "Courses", slug: "courses", parent_org_id: 1, description: "Academic course projects" },
    { id: 3, name: "Club", slug: "club", parent_org_id: 1, description: "Club projects" }
];

const it666 = {
    id: 1,
    organization_id: 2,
    name: "IT666",
    slug: "it666",
    description: "Applied lab project with student and group VM assignments.",
    is_active: true,
    organization: organizations[1]
};

const roles = [
    {
        id: 10,
        name: "IT666 Project Instructor",
        description: "Manage memberships, groups, roles, and student lab resources.",
        is_system_role: false,
        owner_scope_label: "project",
        owner_scope_id: 1,
        permission_count: 9
    },
    {
        id: 11,
        name: "IT666 VM User",
        description: "Operate specifically assigned IT666 VMs.",
        is_system_role: false,
        owner_scope_label: "project",
        owner_scope_id: 1,
        permission_count: 5
    },
    {
        id: 1,
        name: "Admin",
        description: "System administrator role managed by setup.",
        is_system_role: true,
        owner_scope_label: "global",
        owner_scope_id: null,
        permission_count: 38
    }
];

const permissions = [
    "project.manage",
    "group.manage",
    "role.manage",
    "vm.read",
    "vm.create",
    "vm.start",
    "vm.stop",
    "vm.reboot",
    "vm.console",
    "vm.delete",
    "audit.read"
].map((name, index) => ({ id: index + 1, name }));

const rolePermissionIDs = new Map([
    [10, [1, 2, 3, 4, 5, 6, 7, 8, 9]],
    [11, [4, 6, 7, 8, 9]],
    [1, permissions.map((permission) => permission.id)]
]);

await mkdir(outputDir, { recursive: true });

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });

await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;

    if (path === "/api/v1/system/summary") {
        return fulfillJSON(route, {
            counts: { organizations: 3, projects: 3, users: 7, groups: 8, roles: 9 },
            currentUser: { id: 1, username: "alice", displayName: "Alice", email: "alice@example.test", groupCount: 1, isSiteAdmin: true },
            capabilities: { canCreateProjects: true, canManageUsers: true, canManageGroups: true, canManageRoles: true, canManageOrgs: true, canViewUsers: true }
        });
    }

    if (path === "/api/v1/projects/tree") {
        return fulfillJSON(route, {
            organizations,
            projects: [
                it666,
                { id: 2, organization_id: 2, name: "CS527", slug: "cs527", description: "Security engineering", is_active: true },
                { id: 3, organization_id: 3, name: "NECCDC Training", slug: "neccdc-training", description: "Club training", is_active: true }
            ]
        });
    }

    if (path === "/api/v1/users/me/access") {
        return fulfillJSON(route, { groups: [], roles: [], roleBindings: [], isSiteAdmin: true });
    }

    if (path === "/api/v1/users") {
        return fulfillJSON(route, []);
    }

    if (path === "/api/v1/projects/it666") {
        return fulfillJSON(route, it666);
    }

    if (path === "/api/v1/projects/1/memberships") {
        return fulfillJSON(route, [
            { id: 1, project_id: 1, subject_type: "group", subject_id: 21, access_role_id: 10, access_role_name: "IT666 Project Instructor", subject: { label: "IT666 Instructors", meta: "project-owned group" } },
            { id: 2, project_id: 1, subject_type: "group", subject_id: 22, access_role_id: 11, access_role_name: "IT666 VM User", subject: { label: "IT666 Group 01", meta: "group project assets" } },
            { id: 3, project_id: 1, subject_type: "user", subject_id: 31, access_role_id: 11, access_role_name: "IT666 VM User", subject: { label: "Charlie", username: "charlie", meta: "direct VM assignment" } }
        ]);
    }

    if (path === "/api/v1/projects/1/groups") {
        return fulfillJSON(route, [
            { id: 21, name: "IT666 Instructors", slug: "it666-instructors", member_count: 1, sync_membership: false },
            { id: 22, name: "IT666 Group 01", slug: "it666-group-01", member_count: 2, sync_membership: false }
        ]);
    }

    if (path === "/api/v1/projects/1/roles") {
        return fulfillJSON(route, roles);
    }

    if (path === "/api/v1/permissions") {
        return fulfillJSON(route, permissions);
    }

    const rolePermissionMatch = path.match(/^\/api\/v1\/roles\/(\d+)\/permissions$/);
    if (rolePermissionMatch) {
        const roleID = Number(rolePermissionMatch[1]);
        const permissionIDs = rolePermissionIDs.get(roleID) || [];
        return fulfillJSON(route, permissionIDs.map((permissionID, index) => ({
            id: index + 1,
            role_id: roleID,
            permission_id: permissionID,
            permission: permissions.find((permission) => permission.id === permissionID)
        })));
    }

    return fulfillJSON(route, []);
});

await page.goto(`${baseURL}/dashboard?view=directory`);
await page.getByText("Project roles").waitFor();
await page.screenshot({ path: join(outputDir, "directory-roles.png"), fullPage: true });

await openRoleMenu(page, "IT666 Project Instructor");
await page.screenshot({ path: join(outputDir, "role-actions-menu.png"), fullPage: true });

await page.getByRole("menuitem", { name: "Edit role" }).click();
await page.getByRole("dialog", { name: "Edit role" }).waitFor();
await page.screenshot({ path: join(outputDir, "role-editor-editable.png"), fullPage: true });
await page.getByRole("button", { name: "Close", exact: true }).click();

await openRoleMenu(page, "Admin");
await page.getByRole("menuitem", { name: "View role" }).click();
await page.getByRole("dialog", { name: "Role details" }).waitFor();
await page.screenshot({ path: join(outputDir, "role-editor-system-readonly.png"), fullPage: true });

await browser.close();

console.log(`Screenshots written to ${outputDir}`);

async function openRoleMenu(page, roleName) {
    await page.evaluate((name) => {
        const rows = Array.from(document.querySelectorAll(".access-list-row"));
        const row = rows.find((element) => element.querySelector('button[aria-label="Role actions"]') && element.textContent?.includes(name));
        const button = row?.querySelector('button[aria-label="Role actions"]');
        if (!(button instanceof HTMLButtonElement)) {
            const rowTexts = rows.map((element) => element.textContent?.trim()).join(" | ");
            throw new Error(`Role action button not found for ${name}; rows: ${rowTexts}`);
        }
        button.click();
    }, roleName);
    await page.locator(".role-inline-menu").first().waitFor();
    await page.waitForTimeout(180);
}

async function fulfillJSON(route, body) {
    await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify(body)
    });
}

import { createReadStream } from "node:fs";
import { createServer } from "node:http";
import { extname, join } from "node:path";
import { test, expect } from "@playwright/test";

const root = join(import.meta.dirname, "..");
let server;
let baseURL;

test.beforeAll(async () => {
    server = createServer((request, response) => {
        if (request.url === "/dashboard" || request.url?.startsWith("/dashboard?")) {
            response.setHeader("Content-Type", "text/html");
            response.end('<!doctype html><html><head><meta charset="utf-8"></head><body class="dashboard-page"><div id="dashboard-root"></div><script type="module" src="/static/build/site.js"></script></body></html>');
            return;
        }
        if (request.url?.startsWith("/static/")) {
            const file = join(root, "client", request.url);
            response.setHeader("Content-Type", extname(file) === ".css" ? "text/css" : extname(file) === ".js" ? "text/javascript" : "application/octet-stream");
            createReadStream(file).on("error", () => { response.statusCode = 404; response.end(); }).pipe(response);
            return;
        }
        response.statusCode = 404;
        response.end();
    });
    await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
    baseURL = `http://127.0.0.1:${server.address().port}`;
});

test.afterAll(async () => {
    await new Promise((resolve) => server.close(resolve));
});

test("assigned user operates a VM, sees job progress, and opens a popup console", async ({ page }) => {
    let powerState = 1;
    let jobPoll = 0;
    await mockDashboard(page, () => [{
        id: 2, project_id: 1, name: "Student VM", slug: "student-vm", resource_type: 0,
        resource_type_label: "vm", status: 0, status_label: "ready", power_state: powerState,
        proxmox_vmid: 121, proxmox_node: "tungsten", assignment_count: 1, asset_group_count: 1,
        can_start: true, can_stop: true, can_reboot: true, can_console: true
    }]);
    await page.route("**/api/v1/resources/2/actions/start", async (route) => {
        await fulfill(route, job(9, 0, 0));
    });
    await page.route("**/api/v1/jobs/9", async (route) => {
        jobPoll++;
        if (jobPoll > 1) powerState = 0;
        await fulfill(route, job(9, jobPoll > 1 ? 2 : 1, jobPoll > 1 ? 100 : 45));
    });
    await page.route("**/api/v1/resources/2/console-sessions", async (route) => {
        await fulfill(route, { websocket_path: "/api/v1/console-sessions/test/websocket", console_password: "scoped-ticket" }, 201);
    });

    await page.goto(`${baseURL}/dashboard?view=directory`);
    await expect(page.getByText("Student VM")).toBeVisible();
    await page.getByRole("button", { name: "Student VM power actions" }).click();
    await expect(page.getByRole("menuitem", { name: "Start" })).toBeEnabled();
    await page.getByRole("menuitem", { name: "Start" }).click();
    await expect(page.getByText("vm.start: running · 45%")).toBeVisible();
    await expect(page.getByText("vm.start: succeeded · 100%")).toBeVisible({ timeout: 5000 });
    await expect(page.getByRole("button", { name: "Student VM power actions" })).toContainText("Running");

    await page.getByRole("button", { name: "Student VM power actions" }).click();
    const popupPromise = page.waitForEvent("popup");
    await page.getByRole("menuitem", { name: "Open console" }).click();
    const popup = await popupPromise;
    await expect.poll(() => popup.isClosed()).toBe(false);
    await popup.close();
});

test("capability-limited and unassigned users cannot discover unavailable actions or resources", async ({ page }) => {
    await mockDashboard(page, () => [{
        id: 2, project_id: 1, name: "Limited VM", slug: "limited-vm", resource_type: 0,
        resource_type_label: "vm", status: 0, status_label: "ready", power_state: 0,
        proxmox_vmid: 121, proxmox_node: "tungsten", can_start: false, can_stop: false,
        can_reboot: false, can_console: true
    }]);
    await page.goto(`${baseURL}/dashboard?view=directory`);
    await page.getByRole("button", { name: "Limited VM power actions" }).click();
    await expect(page.getByRole("menuitem", { name: "Open console" })).toBeVisible();
    await expect(page.getByRole("menuitem", { name: "Reboot" })).toHaveCount(0);
    await expect(page.getByRole("menuitem", { name: "Hard stop" })).toHaveCount(0);

    await page.unrouteAll();
    await mockDashboard(page, () => []);
    await page.reload();
    await expect(page.getByText("Limited VM")).toHaveCount(0);
    await expect(page.getByText("No local resources have been created.")).toBeVisible();
});

test("instructor publishes a runner-backed blueprint and previews group deployments", async ({ page }) => {
    let blueprints = [];
    await page.route("**/api/v1/**", async (route) => {
        const path = new URL(route.request().url()).pathname;
        if (path === "/api/v1/system/summary") return fulfill(route, { counts: {}, currentUser: { id: 1, username: "instructor", isSiteAdmin: true }, capabilities: { canManageProxmox: true } });
        if (path === "/api/v1/projects/tree") return fulfill(route, { organizations: [{ id: 1, name: "Course", slug: "course", parent_org_id: null }], projects: [{ id: 1, organization_id: 1, name: "Class Lab", slug: "class-lab", is_active: true }] });
        if (path === "/api/v1/users/me/access") return fulfill(route, { groups: [], roles: [], roleBindings: [], isSiteAdmin: true });
        if (path === "/api/v1/projects/1/blueprints" && route.request().method() === "GET") return fulfill(route, blueprints);
        if (path === "/api/v1/projects/1/blueprints") {
            blueprints = [{ id: 4, uuid: "blueprint", project_id: 1, name: "Generic Lab", slug: "generic-lab", versions: [] }];
            return fulfill(route, blueprints[0], 201);
        }
        if (path === "/api/v1/blueprints/4/versions") {
            blueprints[0].versions = [{ id: 7, uuid: "version", version: 1, document_digest: "sha256:test", document: {}, created_at: new Date().toISOString() }];
            return fulfill(route, blueprints[0].versions[0], 201);
        }
        if (path === "/api/v1/projects/1/deployment-previews") return fulfill(route, { blueprint: { name: "Generic Lab", version: 1 }, runner: { opentofu_module: "module", ansible_project: "playbooks" }, deployments: [{ name: "class-lab-g01", resources: [{ name: "class-lab-g01-server" }] }], mutates: false });
        return fulfill(route, []);
    });

    await page.goto(`${baseURL}/dashboard?view=blueprints`);
    await page.getByLabel("Name").fill("Generic Lab");
    await page.getByLabel("Slug").fill("generic-lab");
    await page.getByRole("button", { name: "Create blueprint" }).click();
    await expect(page.getByText("Generic Lab", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Publish current document" }).click();
    await expect(page.getByText("Version 1")).toBeVisible();
    await page.getByLabel("Preview group IDs, comma-separated").fill("12, 13, 14");
    await page.getByRole("button", { name: "Preview" }).click();
    await expect(page.getByText("Deployment preview")).toBeVisible();
    await expect(page.getByText(/class-lab-g01-server/)).toBeVisible();
});

async function mockDashboard(page, resources) {
    const organization = { id: 1, name: "Course", slug: "course", parent_org_id: null };
    const project = { id: 1, organization_id: 1, name: "Class Lab", slug: "class-lab", is_active: true };
    await page.route("**/api/v1/**", async (route) => {
        const path = new URL(route.request().url()).pathname;
        if (path === "/api/v1/system/summary") return fulfill(route, { counts: {}, currentUser: { id: 7, username: "student", isSiteAdmin: false }, capabilities: {} });
        if (path === "/api/v1/projects/tree") return fulfill(route, { organizations: [organization], projects: [project] });
        if (path === "/api/v1/projects/class-lab") return fulfill(route, project);
        if (path === "/api/v1/projects/1/resources") return fulfill(route, resources());
        if (path === "/api/v1/projects/1/quota") return fulfill(route, null);
        if (path === "/api/v1/users/me/access") return fulfill(route, { groups: [], roles: [], roleBindings: [], isSiteAdmin: false });
        return fulfill(route, []);
    });
}

function job(id, status, progress) {
    return { id, uuid: `job-${id}`, status, operation: "vm.start", operation_key: "test", resource_id: 2, progress, attempt_count: status ? 1 : 0, max_attempts: 1, created_at: new Date().toISOString(), updated_at: new Date().toISOString() };
}

async function fulfill(route, body, status = 200) {
    await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

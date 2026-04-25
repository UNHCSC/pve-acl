import type { User, ViewKey } from "./types";
import { viewTitles } from "./types";

export function classNames(...parts: Array<string | false | null | undefined>): string {
    return parts.filter(Boolean).join(" ");
}

export function initialView(): ViewKey {
    const requested = new URLSearchParams(window.location.search).get("view");
    if (requested === "projects") {
        return "directory";
    }
    if (requested && Object.hasOwn(viewTitles, requested)) {
        return requested as ViewKey;
    }
    return "overview";
}

export function displayUser(user?: Partial<User> | null): string {
    if (!user) {
        return "Unknown user";
    }
    return user.displayName || user.display_name || user.username || "Unknown user";
}

export function userMeta(user?: Partial<User> | null): string {
    if (!user) {
        return "";
    }
    return user.email || user.authSource || user.auth_source || "";
}

export function initials(name: string): string {
    const letters = name
        .split(/\s+/)
        .filter(Boolean)
        .slice(0, 2)
        .map((part) => part[0]?.toUpperCase())
        .join("");
    return letters || "--";
}

export function numberValue(value: unknown): number {
    if (typeof value === "number") {
        return value;
    }
    if (typeof value === "string") {
        const parsed = Number(value);
        return Number.isFinite(parsed) ? parsed : 0;
    }
    return 0;
}

export function roleLabel(value: number | string | undefined): string {
    const normalized = String(value ?? "").toLowerCase();
    const labels: Record<string, string> = {
        "1": "viewer",
        "2": "operator",
        "3": "developer",
        "4": "manager",
        "5": "owner"
    };
    return labels[normalized] || normalized || "viewer";
}

export function subjectTypeLabel(value: number | string | undefined): "user" | "group" {
    const normalized = String(value ?? "").toLowerCase();
    return normalized === "2" || normalized === "group" ? "group" : "user";
}

export function scopeTypeLabel(value: number | string | undefined): string {
    const normalized = String(value ?? "").toLowerCase();
    const labels: Record<string, string> = {
        "1": "global",
        "2": "organization",
        "3": "project",
        global: "global",
        org: "organization",
        project: "project"
    };
    return labels[normalized] || normalized || "global";
}

export function formatCount(value: unknown): string {
    return numberValue(value).toLocaleString();
}

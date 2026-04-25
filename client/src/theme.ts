import type { ThemeKey } from "./types";

export const themeOptions: Array<{ key: ThemeKey; label: string }> = [
    { key: "light", label: "Light" },
    { key: "dark", label: "Dark" },
    { key: "proxmox-light", label: "Proxmox Light" },
    { key: "proxmox-dark", label: "Proxmox Dark" }
];

export function readStoredTheme(): ThemeKey {
    try {
        const theme = window.localStorage.getItem("organesson-theme") || window.localStorage.getItem("pve-acl-theme");
        if (theme === "proxmox") {
            return "proxmox-light";
        }
        return theme === "dark" || theme === "proxmox-light" || theme === "proxmox-dark" ? theme : "light";
    } catch {
        return "light";
    }
}

export function applyTheme(theme: ThemeKey) {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme === "dark" || theme === "proxmox-dark" ? "dark" : "light";
    try {
        window.localStorage.setItem("organesson-theme", theme);
    } catch {
        // Theme still applies for this page when persistent storage is unavailable.
    }
}

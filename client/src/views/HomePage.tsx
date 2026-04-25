import { useEffect, useState } from "react";
import { ThemeSettings } from "../components/ThemeSettings";
import type { ThemeKey } from "../types";

export function HomePage({ currentYear, theme, setTheme }: { currentYear: string; theme: ThemeKey; setTheme: (theme: ThemeKey) => void }) {
    const [settingsOpen, setSettingsOpen] = useState(false);

    useEffect(() => {
        if (!settingsOpen) {
            return;
        }
        const closeSettings = (event: MouseEvent) => {
            const target = event.target;
            if (target instanceof Element && target.closest("[data-topbar-menu]")) {
                return;
            }
            setSettingsOpen(false);
        };
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                setSettingsOpen(false);
            }
        };
        window.addEventListener("click", closeSettings);
        window.addEventListener("keydown", closeOnEscape);
        return () => {
            window.removeEventListener("click", closeSettings);
            window.removeEventListener("keydown", closeOnEscape);
        };
    }, [settingsOpen]);

    return (
        <div className="page-frame">
            <header className="site-header">
                <div className="section-shell flex h-16 items-center justify-between gap-6">
                    <a href="/" className="brand-mark">
                        <img className="brand-logo" src="/static/logo.svg" alt="" aria-hidden="true" />
                        <span>Organesson Cloud</span>
                    </a>
                    <ThemeSettings open={settingsOpen} setOpen={setSettingsOpen} theme={theme} setTheme={setTheme} />
                </div>
            </header>

            <main>
                <section className="section-shell home-hero">
                    <div className="hero-copy-block">
                        <p className="eyebrow">Proxmox access control</p>
                        <h1 className="hero-title">A control plane for shared virtualization infrastructure.</h1>
                        <p className="hero-copy">
                            Organesson Cloud organizes organizations, sub-organizations, projects, memberships, and delegated permissions above Proxmox so administrators can expose useful self-service access without handing out cluster-wide control.
                        </p>
                        <div className="hero-actions">
                            <a href="/dashboard" className="button-primary">Dashboard</a>
                        </div>
                    </div>

                    <div className="mission-panel" aria-label="Organesson Cloud mission summary">
                        <div>
                            <span className="panel-label">Tenancy</span>
                            <strong>Org trees, teams, projects</strong>
                        </div>
                        <div>
                            <span className="panel-label">Delegation</span>
                            <strong>RBAC, memberships, quotas</strong>
                        </div>
                        <div>
                            <span className="panel-label">Backend</span>
                            <strong>Proxmox stays the source of compute</strong>
                        </div>
                    </div>
                </section>

                <section className="section-shell feature-strip" aria-label="Capabilities">
                    <article>
                        <h2>Nested organizations</h2>
                        <p>Model departments, teams, tenants, and delegated spaces as one permission-aware tree.</p>
                    </article>
                    <article>
                        <h2>Project ownership</h2>
                        <p>Attach projects to the tree and keep direct memberships separate from inherited access.</p>
                    </article>
                    <article>
                        <h2>Audited delegation</h2>
                        <p>Keep roles, grants, and project actions visible without spreading raw Proxmox permissions.</p>
                    </article>
                </section>

                <section className="section-shell cta-band">
                    <div>
                        <h2>Designed for multi-tenant Proxmox operations.</h2>
                        <p>Start with identity, organization structure, and project access; connect resource workflows behind those boundaries.</p>
                    </div>
                </section>
            </main>

            <footer className="site-footer">
                <div className="section-shell flex flex-col gap-3 py-6 text-sm text-ink-muted sm:flex-row sm:items-center sm:justify-between">
                    <p>&copy; {currentYear} Organesson Cloud</p>
                </div>
            </footer>
        </div>
    );
}

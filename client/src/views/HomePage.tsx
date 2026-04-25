export function HomePage({ currentYear }: { currentYear: string }) {
    return (
        <div className="page-frame">
            <header className="site-header">
                <div className="section-shell flex h-16 items-center justify-between gap-6">
                    <a href="/" className="brand-mark">
                        <span className="brand-icon">PC</span>
                        <span>PVE Cloud</span>
                    </a>
                    <nav className="hidden items-center gap-8 text-sm font-medium text-ink-muted sm:flex">
                        <a href="/dashboard" className="nav-link">Dashboard</a>
                        <a href="/login" className="nav-link">Sign in</a>
                    </nav>
                </div>
            </header>

            <main>
                <section className="section-shell home-hero">
                    <div className="hero-copy-block">
                        <p className="eyebrow">Cyber lab cloud manager</p>
                        <h1 className="hero-title">Self-service lab infrastructure without handing out cluster admin.</h1>
                        <p className="hero-copy">
                            PVE Cloud organizes students, teams, organizations, projects, quotas, and VM ownership above Proxmox so trusted admins keep the cluster while users get the access they actually need.
                        </p>
                        <div className="hero-actions">
                            <a href="/dashboard" className="button-primary">View dashboard</a>
                            <a href="/login" className="button-secondary">Sign in</a>
                        </div>
                    </div>

                    <div className="mission-panel" aria-label="PVE Cloud mission summary">
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
                        <h2>Teaching labs</h2>
                        <p>Give instructors and TAs bounded control over student environments.</p>
                    </article>
                    <article>
                        <h2>Team spaces</h2>
                        <p>Separate club, competition, and research infrastructure cleanly.</p>
                    </article>
                    <article>
                        <h2>Audited access</h2>
                        <p>Track ownership and privileged actions outside Proxmox ACL sprawl.</p>
                    </article>
                </section>

                <section className="section-shell cta-band">
                    <div>
                        <h2>Built for shared cybersecurity infrastructure.</h2>
                        <p>Create the local control plane first, then attach Proxmox workflows behind it.</p>
                    </div>
                    <a href="/dashboard" className="button-primary">Open console</a>
                </section>
            </main>

            <footer className="site-footer">
                <div className="section-shell flex flex-col gap-3 py-6 text-sm text-ink-muted sm:flex-row sm:items-center sm:justify-between">
                    <p>&copy; {currentYear} PVE Cloud</p>
                    <div className="flex gap-6">
                        <a href="/dashboard" className="nav-link">Dashboard</a>
                    </div>
                </div>
            </footer>
        </div>
    );
}

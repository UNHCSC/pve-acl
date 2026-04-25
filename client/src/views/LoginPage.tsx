export function LoginPage({ redirect, loginError }: { redirect: string; loginError: string }) {
    return (
        <main className="section-shell flex min-h-screen items-center justify-center py-16">
            <section className="login-shell">
                <div className="space-y-4">
                    <p className="eyebrow">Console</p>
                    <h1 className="section-title">Sign in to Organesson Cloud.</h1>
                    <p className="section-copy">Use your directory credentials to manage projects, quotas, and access.</p>
                </div>

                {loginError && (
                    <div className="rounded-2xl border border-[#d9b7af] bg-[#fff4f1] px-4 py-3 text-sm text-[#8c4030]" role="alert">
                        {loginError}
                    </div>
                )}

                <form action="/api/v1/auth/login" method="post" className="space-y-5">
                    <input type="hidden" name="redirect" value={redirect} />

                    <label className="field-group">
                        <span className="field-label">Username</span>
                        <input className="field-input" type="text" name="username" autoComplete="username" required />
                    </label>

                    <label className="field-group">
                        <span className="field-label">Password</span>
                        <input className="field-input" type="password" name="password" autoComplete="current-password" required />
                    </label>

                    <button type="submit" className="button-primary w-full justify-center">Sign In</button>
                </form>
            </section>
        </main>
    );
}

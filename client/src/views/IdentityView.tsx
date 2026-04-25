import { CompactList, Detail, PanelHeading } from "../components/common";
import type { MyAccess, Summary } from "../types";
import { displayUser } from "../ui-helpers";

export function IdentityView({ summary, myAccess }: { summary: Summary | null; myAccess: MyAccess | null }) {
    return (
        <section className="dashboard-view is-active">
            <div className="dashboard-grid">
                <article className="dashboard-panel">
                    <PanelHeading label="Current account" title={displayUser(summary?.currentUser)} />
                    <dl className="detail-list">
                        <Detail label="Username">{summary?.currentUser.username || "-"}</Detail>
                        <Detail label="Email">{summary?.currentUser.email || "-"}</Detail>
                        <Detail label="Site admin">{summary?.currentUser.isSiteAdmin ? "yes" : "no"}</Detail>
                    </dl>
                </article>
                <article className="dashboard-panel">
                    <PanelHeading label="Membership" title="Your groups" />
                    <CompactList items={myAccess?.groups || []} render={(group) => <><strong>{group.name}</strong><span>{group.slug}</span></>} />
                </article>
                <article className="dashboard-panel wide-panel">
                    <PanelHeading label="Effective roles" title="Role bindings" />
                    <CompactList items={myAccess?.roles || []} render={(role) => <><strong>{role.name}</strong><span>{role.description || `${role.permission_count || 0} permissions`}</span></>} />
                </article>
            </div>
        </section>
    );
}

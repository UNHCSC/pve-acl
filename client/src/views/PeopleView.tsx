import { EmptyState, PanelHeading } from "../components/common";
import type { User } from "../types";
import { displayUser, userMeta } from "../ui-helpers";

export function PeopleView({ users, openCreate }: { users: User[]; openCreate: () => void }) {
    return (
        <section className="dashboard-view is-active">
            <article className="dashboard-panel">
                <PanelHeading label="Identity" title="Users" action={<button className="button-primary compact-button" type="button" onClick={openCreate}>New user</button>} />
                <div className="data-table-head people-table-head">
                    <span>User</span>
                    <span>Email</span>
                    <span>Source</span>
                </div>
                {users.length === 0 && <EmptyState>No users are loaded.</EmptyState>}
                {users.map((user) => (
                    <div className="data-table-row people-table-row" key={user.id}>
                        <div>
                            <strong>{displayUser(user)}</strong>
                            <span>{user.username}</span>
                        </div>
                        <span>{userMeta(user) || "-"}</span>
                        <span>{user.authSource || user.auth_source || "local"}</span>
                    </div>
                ))}
            </article>
        </section>
    );
}

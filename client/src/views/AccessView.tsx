import { CompactList, EmptyState, PanelHeading } from "../components/common";
import type { AccessData, RoleBinding } from "../types";
import { scopeTypeLabel } from "../ui-helpers";

export function AccessView({
    access,
    openGroup,
    openRole,
    openGrant,
    deleteGrant
}: {
    access: AccessData;
    openGroup: () => void;
    openRole: () => void;
    openGrant: () => void;
    deleteGrant: (grant: RoleBinding) => void;
}) {
    return (
        <section className="dashboard-view is-active">
            <div className="access-grid">
                <article className="dashboard-panel">
                    <PanelHeading label="Groups" title="Cloud groups" action={<button className="button-primary compact-button" type="button" onClick={openGroup}>New group</button>} />
                    <CompactList items={access.groups} render={(group) => <><strong>{group.name}</strong><span>{group.slug} / {group.member_count || 0} members</span></>} />
                </article>
                <article className="dashboard-panel">
                    <PanelHeading label="Roles" title="Permission sets" action={<button className="button-primary compact-button" type="button" onClick={openRole}>New role</button>} />
                    <CompactList items={access.roles} render={(role) => <><strong>{role.name}</strong><span>{role.description || `${role.permission_count || 0} permissions`}</span></>} />
                </article>
                <article className="dashboard-panel access-grants-panel">
                    <PanelHeading label="Grants" title="Role bindings" action={<button className="button-primary compact-button" type="button" onClick={openGrant}>New binding</button>} />
                    <div className="compact-list">
                        {access.roleBindings.length === 0 && <EmptyState>No role bindings found.</EmptyState>}
                        {access.roleBindings.map((grant) => (
                            <div className="compact-list-row action-list-row" key={grant.id}>
                                <div>
                                    <strong>{grant.role?.name || `Role ${grant.role_id}`}</strong>
                                    <span>
                                        {grant.subject?.label || grant.subject?.name || `Subject ${grant.subject_id}`} / {scopeTypeLabel(grant.scope_type)} {grant.scope_id || ""}
                                    </span>
                                </div>
                                <button className="button-secondary compact-button danger-button" type="button" onClick={() => deleteGrant(grant)}>
                                    Delete
                                </button>
                            </div>
                        ))}
                    </div>
                </article>
                <article className="dashboard-panel permissions-panel">
                    <PanelHeading label="Permissions" title="Registered actions" />
                    <div className="permission-cloud">
                        {access.permissions.map((permission) => (
                            <span className="permission-pill" key={permission.id}>
                                {permission.name}
                            </span>
                        ))}
                    </div>
                </article>
            </div>
        </section>
    );
}

import { useEffect, useState, type FormEvent } from "react";
import { apiFetch } from "../api";
import { Field, ModalFrame, RowActionMenu, Select, SimpleFormModal, Textarea } from "./common";
import type { AssetGroup, GroupMembership, Organization, Project, ProjectResource, Role, UserImportResponse } from "../types";
import { classNames, displayUser } from "../ui-helpers";

export function OrgModal({
    context,
    orgs,
    onSubmit,
    onClose
}: {
    context: Organization | null;
    orgs: Organization[];
    onSubmit: (values: { name: string; slug: string; description: string; parentOrgID: number | null }) => Promise<void>;
    onClose: () => void;
}) {
    const defaultParentID = context?.id ?? orgs[0]?.id ?? "";
    return (
        <SimpleFormModal
            title="New organization"
            label="Directory"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    name: String(data.get("name") || ""),
                    slug: String(data.get("slug") || ""),
                    description: String(data.get("description") || ""),
                    parentOrgID: data.get("parentOrgID") ? Number(data.get("parentOrgID")) : null
                })
            }
        >
            <Field name="name" label="Name" required />
            <Field name="slug" label="Slug" required />
            <Textarea name="description" label="Description" />
            <Select name="parentOrgID" label="Parent organization" required={orgs.length > 0} defaultValue={defaultParentID}>
                {orgs.length === 0 && <option value="">Root level</option>}
                {orgs.map((org) => (
                    <option key={org.id} value={org.id}>
                        {org.name}
                    </option>
                ))}
            </Select>
        </SimpleFormModal>
    );
}

export function ProjectModal({
    context,
    orgs,
    onSubmit,
    onClose
}: {
    context: Organization | null;
    orgs: Organization[];
    onSubmit: (values: { name: string; slug: string; description: string; organizationID: number }) => Promise<void>;
    onClose: () => void;
}) {
    return (
        <SimpleFormModal
            title="New project"
            label="Directory"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    name: String(data.get("name") || ""),
                    slug: String(data.get("slug") || ""),
                    description: String(data.get("description") || ""),
                    organizationID: Number(data.get("organizationID"))
                })
            }
        >
            <Field name="name" label="Name" required />
            <Field name="slug" label="Slug" required />
            <Textarea name="description" label="Description" />
            <Select name="organizationID" label="Organization" required defaultValue={context?.id || orgs[0]?.id || ""}>
                {orgs.map((org) => (
                    <option key={org.id} value={org.id}>
                        {org.name}
                    </option>
                ))}
            </Select>
        </SimpleFormModal>
    );
}

export function ImportUsersModal({
    onSubmit,
    onClose
}: {
    onSubmit: (values: { entries: string }) => Promise<UserImportResponse>;
    onClose: () => void;
}) {
    const [submitting, setSubmitting] = useState(false);
    const [result, setResult] = useState<UserImportResponse | null>(null);
    const [error, setError] = useState("");

    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        setSubmitting(true);
        setError("");
        try {
            const nextResult = await onSubmit({ entries: String(new FormData(event.currentTarget).get("entries") || "") });
            setResult(nextResult);
        } catch (submitError) {
            setError(submitError instanceof Error ? submitError.message : "User import failed");
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <ModalFrame
            title="Import users"
            label="Identity"
            onClose={onClose}
        >
            <form className="modal-form import-users-form" onSubmit={submit}>
                <label className="field-group">
                    <span className="field-label">FreeIPA users</span>
                    <textarea className="field-input import-users-input" name="entries" rows={7} placeholder={"alice\nbob@example.edu\ncarol"} required />
                </label>
                {error && <p className="form-message is-warning">{error}</p>}
                {result && (
                    <div className="import-results" role="status">
                        <div className="import-summary">
                            <strong>{result.imported}</strong>
                            <span>imported or already present</span>
                            <strong>{result.failed}</strong>
                            <span>failed</span>
                        </div>
                        <div className="import-result-list">
                            {result.results.map((item) => (
                                <div className={classNames("import-result-row", item.status === "failed" ? "is-failed" : "is-imported")} key={item.query}>
                                    <div>
                                        <strong>{item.user ? displayUser(item.user) : item.query}</strong>
                                        <span>{item.email || item.error || item.status}</span>
                                    </div>
                                    <span>{item.status === "already-imported" ? "already imported" : item.status}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}
                <div className="modal-actions">
                    <button type="button" className="button-secondary" onClick={onClose} disabled={submitting}>
                        Close
                    </button>
                    <button type="submit" className="button-primary" disabled={submitting}>
                        Import
                    </button>
                </div>
            </form>
        </ModalFrame>
    );
}

export function GroupModal({
    context,
    onSubmit,
    onClose
}: {
    context?: Organization | Project | null;
    onSubmit: (values: { name: string; slug: string; description: string }) => Promise<void>;
    onClose: () => void;
}) {
    const scopedTitle = context ? `New group for ${context.name}` : "New group";
    return (
        <SimpleFormModal
            title={scopedTitle}
            label="Access"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    name: String(data.get("name") || ""),
                    slug: String(data.get("slug") || ""),
                    description: String(data.get("description") || "")
                })
            }
        >
            <Field name="name" label="Name" required />
            <Field name="slug" label="Slug" required />
            <Textarea name="description" label="Description" />
        </SimpleFormModal>
    );
}

export function RoleModal({
    context,
    onSubmit,
    onClose
}: {
    context?: Organization | Project | null;
    onSubmit: (values: { name: string; description: string }) => Promise<void>;
    onClose: () => void;
}) {
    const scopedTitle = context ? `New role for ${context.name}` : "New role";
    return (
        <SimpleFormModal
            title={scopedTitle}
            label="Access"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    name: String(data.get("name") || ""),
                    description: String(data.get("description") || "")
                })
            }
        >
            <Field name="name" label="Name" required />
            <Textarea name="description" label="Description" />
        </SimpleFormModal>
    );
}

export function ProjectMemberModal({
    defaultSubjectType = "user",
    scopeLabel = "Project",
    roles,
    onSubmit,
    onClose
}: {
    defaultSubjectType?: "user" | "group";
    scopeLabel?: string;
    roles?: { id: number; name: string }[];
    onSubmit: (values: { subjectType: string; subjectRef: string; roleID?: number }) => Promise<void>;
    onClose: () => void;
}) {
    const subjectType = defaultSubjectType;
    const isGroup = subjectType === "group";
    const defaultRoleID = roles?.[0]?.id || "";

    return (
        <SimpleFormModal
            title={isGroup ? `Add ${scopeLabel.toLowerCase()} group` : `Add ${scopeLabel.toLowerCase()} user`}
            label={isGroup ? "Group access" : "User access"}
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    subjectType,
                    subjectRef: String(data.get("subjectRef") || ""),
                    roleID: data.get("roleID") ? Number(data.get("roleID")) : undefined
                })
            }
        >
            <Field name="subjectRef" label={isGroup ? "Group slug" : "Username or email"} required />
            {roles && roles.length > 0 && (
                <Select name="roleID" label={`${scopeLabel} role`} defaultValue={defaultRoleID} required>
                    {roles.map((role) => (
                        <option key={role.id} value={role.id}>
                            {role.name}
                        </option>
                    ))}
                </Select>
            )}
        </SimpleFormModal>
    );
}

export function ResourceModal({
    project,
    onSubmit,
    onClose
}: {
    project: Project;
    onSubmit: (values: { name: string; slug: string; resourceType: string }) => Promise<void>;
    onClose: () => void;
}) {
    return (
        <SimpleFormModal
            title={`New resource for ${project.name}`}
            label="Inventory"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    name: String(data.get("name") || ""),
                    slug: String(data.get("slug") || ""),
                    resourceType: String(data.get("resourceType") || "vm")
                })
            }
        >
            <Field name="name" label="Name" required />
            <Field name="slug" label="Slug" />
            <Select name="resourceType" label="Type" defaultValue="vm" required>
                <option value="vm">Virtual machine</option>
                <option value="container">Container</option>
                <option value="network">Network</option>
            </Select>
        </SimpleFormModal>
    );
}

export function AssetGroupModal({
    project,
    group,
    onSubmit,
    onClose
}: {
    project: Project;
    group?: AssetGroup;
    onSubmit: (values: { name: string; slug: string; description: string }) => Promise<void>;
    onClose: () => void;
}) {
    return (
        <SimpleFormModal
            title={group ? `Edit ${group.name}` : `New asset group for ${project.name}`}
            label="Inventory"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    name: String(data.get("name") || ""),
                    slug: String(data.get("slug") || ""),
                    description: String(data.get("description") || "")
                })
            }
        >
            <Field name="name" label="Name" defaultValue={group?.name} required />
            <Field name="slug" label="Slug" defaultValue={group?.slug} />
            <Textarea name="description" label="Description" defaultValue={group?.description} />
        </SimpleFormModal>
    );
}

export function AssetGroupResourcesModal({
    group,
    resources,
    onAdd,
    onRemove,
    onClose
}: {
    group: AssetGroup;
    resources: ProjectResource[];
    onAdd: (resourceID: number) => Promise<void>;
    onRemove: (resourceID: number) => Promise<void>;
    onClose: () => void;
}) {
    const attachedIDs = new Set((group.resources || []).map((resource) => resource.id));
    const available = resources.filter((resource) => !attachedIDs.has(resource.id));
    const [selectedResourceID, setSelectedResourceID] = useState<number | "">(available[0]?.id || "");
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState("");

    useEffect(() => {
        setSelectedResourceID((current) => {
            if (current && available.some((resource) => resource.id === current)) {
                return current;
            }
            return available[0]?.id || "";
        });
    }, [group.id, resources.length, group.resources?.length]);

    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!selectedResourceID) {
            return;
        }
        setSaving(true);
        setError("");
        try {
            await onAdd(Number(selectedResourceID));
        } catch (submitError) {
            setError(submitError instanceof Error ? submitError.message : "Failed to add resource");
        } finally {
            setSaving(false);
        }
    };

    const remove = async (resourceID: number) => {
        setSaving(true);
        setError("");
        try {
            await onRemove(resourceID);
        } catch (submitError) {
            setError(submitError instanceof Error ? submitError.message : "Failed to remove resource");
        } finally {
            setSaving(false);
        }
    };

    return (
        <ModalFrame title={`${group.name} resources`} label="Asset group" onClose={onClose}>
            <div className="group-members-modal asset-group-resources-modal">
                <form className="modal-form group-member-form" onSubmit={submit}>
                    <div className="modal-section-heading">
                        <span className="panel-label">Attach resource</span>
                        <strong>{available.length}</strong>
                    </div>
                    <label className="field-group">
                        <span className="field-label">Resource</span>
                        <select
                            className="field-input"
                            name="resourceID"
                            value={selectedResourceID}
                            disabled={saving || available.length === 0}
                            onChange={(event) => setSelectedResourceID(event.currentTarget.value ? Number(event.currentTarget.value) : "")}
                        >
                            {available.length === 0 && <option value="">No available resources</option>}
                            {available.map((resource) => (
                                <option key={resource.id} value={resource.id}>
                                    {resource.name} ({resource.resource_type_label || "resource"})
                                </option>
                            ))}
                        </select>
                    </label>
                    {error && <p className="form-message is-warning">{error}</p>}
                    <div className="modal-actions">
                        <button type="submit" className="button-primary" disabled={saving || available.length === 0}>
                            Add resource
                        </button>
                    </div>
                </form>
                <section className="group-member-list-panel">
                    <div className="modal-section-heading">
                        <span className="panel-label">Attached</span>
                        <strong>{group.resources?.length || 0}</strong>
                    </div>
                    {(!group.resources || group.resources.length === 0) && <p className="form-message">No resources attached.</p>}
                    {group.resources && group.resources.length > 0 && (
                        <div className="compact-list">
                            {group.resources.map((resource) => (
                                <div className="compact-list-row action-list-row group-member-row" key={resource.id}>
                                    <div className="access-row-subject">
                                        <div>
                                            <strong>{resource.name}</strong>
                                            <span>{resource.slug} / {resource.resource_type_label || "resource"}</span>
                                        </div>
                                    </div>
                                    <div className="project-access-actions">
                                        <span className="access-pill">{resource.status_label || "ready"}</span>
                                        <RowActionMenu ariaLabel={`${resource.name} actions`} className="tree-actions access-row-actions" menuClassName="tree-inline-menu">
                                            <button type="button" role="menuitem" className="danger-action" disabled={saving} onClick={() => remove(resource.id)}>
                                                Remove resource
                                            </button>
                                        </RowActionMenu>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </section>
            </div>
        </ModalFrame>
    );
}

export function AssetAssignmentModal({
    project,
    resources,
    assetGroups,
    roles,
    onSubmit,
    onClose
}: {
    project: Project;
    resources: ProjectResource[];
    assetGroups: AssetGroup[];
    roles: Role[];
    onSubmit: (values: { targetType: string; resourceID?: number; assetGroupID?: number; subjectType: string; subjectRef: string; roleID: number }) => Promise<void>;
    onClose: () => void;
}) {
    const [targetType, setTargetType] = useState<"resource" | "assetGroup">("resource");
    const [subjectType, setSubjectType] = useState<"user" | "group">("user");
    const [submitting, setSubmitting] = useState(false);
    const [error, setError] = useState("");
    const targetsAvailable = targetType === "resource" ? resources.length > 0 : assetGroups.length > 0;

    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        setSubmitting(true);
        setError("");
        try {
            await onSubmit({
                targetType,
                resourceID: targetType === "resource" ? Number(data.get("resourceID")) : undefined,
                assetGroupID: targetType === "assetGroup" ? Number(data.get("assetGroupID")) : undefined,
                subjectType,
                subjectRef: String(data.get("subjectRef") || ""),
                roleID: Number(data.get("roleID"))
            });
        } catch (submitError) {
            setError(submitError instanceof Error ? submitError.message : "Failed to create assignment");
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <ModalFrame title={`Assign assets in ${project.name}`} label="Inventory" onClose={onClose}>
            <form className="modal-form" onSubmit={submit}>
                <label className="field-group">
                    <span className="field-label">Target type</span>
                    <select className="field-input" value={targetType} onChange={(event) => setTargetType(event.currentTarget.value as "resource" | "assetGroup")}>
                        <option value="resource">Resource</option>
                        <option value="assetGroup">Asset group</option>
                    </select>
                </label>
                {targetType === "resource" ? (
                    <Select name="resourceID" label="Resource" required defaultValue={resources[0]?.id || ""}>
                        {resources.length === 0 && <option value="">No resources</option>}
                        {resources.map((resource) => (
                            <option key={resource.id} value={resource.id}>
                                {resource.name} ({resource.resource_type_label || "resource"})
                            </option>
                        ))}
                    </Select>
                ) : (
                    <Select name="assetGroupID" label="Asset group" required defaultValue={assetGroups[0]?.id || ""}>
                        {assetGroups.length === 0 && <option value="">No asset groups</option>}
                        {assetGroups.map((group) => (
                            <option key={group.id} value={group.id}>
                                {group.name} ({group.resource_count || 0} resources)
                            </option>
                        ))}
                    </Select>
                )}
                <label className="field-group">
                    <span className="field-label">Subject type</span>
                    <select className="field-input" value={subjectType} onChange={(event) => setSubjectType(event.currentTarget.value as "user" | "group")}>
                        <option value="user">User</option>
                        <option value="group">Group</option>
                    </select>
                </label>
                <Field name="subjectRef" label={subjectType === "group" ? "Group slug" : "Username or email"} required />
                <Select name="roleID" label="Role" required defaultValue={roles[0]?.id || ""}>
                    {roles.length === 0 && <option value="">No assignable roles</option>}
                    {roles.map((role) => (
                        <option key={role.id} value={role.id}>
                            {role.name}
                        </option>
                    ))}
                </Select>
                {error && <p className="form-message is-warning">{error}</p>}
                <div className="modal-actions">
                    <button type="button" className="button-secondary" onClick={onClose} disabled={submitting}>
                        Cancel
                    </button>
                    <button type="submit" className="button-primary" disabled={submitting || !targetsAvailable || roles.length === 0}>
                        Save
                    </button>
                </div>
            </form>
        </ModalFrame>
    );
}

export function GroupMembersModal({
    group,
    onError,
    onClose
}: {
    group: { id: number; name: string };
    onError?: (message: string) => void;
    onClose: () => void;
}) {
    const [memberships, setMemberships] = useState<GroupMembership[]>([]);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState("");

    const reportError = (caughtError: unknown, fallback: string) => {
        const message = caughtError instanceof Error ? caughtError.message : fallback;
        setError(message);
        onError?.(message);
    };

    const loadMemberships = async () => {
        setLoading(true);
        setError("");
        try {
            setMemberships(await apiFetch<GroupMembership[]>(`/api/v1/groups/${group.id}/memberships`));
        } catch (loadError) {
            reportError(loadError, "Failed to load group members");
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        void loadMemberships();
    }, [group.id]);

    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        setSaving(true);
        setError("");
        const data = new FormData(event.currentTarget);
        try {
            await apiFetch(`/api/v1/groups/${group.id}/memberships`, {
                method: "POST",
                body: JSON.stringify({
                    userRef: String(data.get("userRef") || ""),
                    membershipRole: String(data.get("membershipRole") || "member")
                })
            });
            event.currentTarget.reset();
            await loadMemberships();
        } catch (saveError) {
            reportError(saveError, "Failed to add group member");
        } finally {
            setSaving(false);
        }
    };

    const updateRole = async (membership: GroupMembership, membershipRole: string) => {
        setSaving(true);
        setError("");
        try {
            await apiFetch(`/api/v1/groups/${group.id}/memberships/${membership.id}`, {
                method: "PATCH",
                body: JSON.stringify({ membershipRole })
            });
            await loadMemberships();
        } catch (saveError) {
            reportError(saveError, "Failed to update group member");
        } finally {
            setSaving(false);
        }
    };

    const remove = async (membership: GroupMembership) => {
        if (!window.confirm("Remove this group member?")) {
            return;
        }
        setSaving(true);
        setError("");
        try {
            await apiFetch(`/api/v1/groups/${group.id}/memberships/${membership.id}`, { method: "DELETE" });
            await loadMemberships();
        } catch (saveError) {
            reportError(saveError, "Failed to remove group member");
        } finally {
            setSaving(false);
        }
    };

    return (
        <ModalFrame title={`${group.name} members`} label="Group" onClose={onClose}>
            <div className="group-members-modal">
                <form className="modal-form group-member-form" onSubmit={submit}>
                    <div className="modal-section-heading">
                        <span className="panel-label">Add user</span>
                        <strong>{group.name}</strong>
                    </div>
                    <Field name="userRef" label="Username or email" required />
                    <Select name="membershipRole" label="Group role" defaultValue="member">
                        <option value="member">Member</option>
                        <option value="manager">Manager</option>
                        <option value="owner">Owner</option>
                    </Select>
                    <div className="modal-actions">
                        <button type="submit" className="button-primary" disabled={saving}>
                            Add member
                        </button>
                    </div>
                </form>
                <section className="group-member-list-panel">
                    <div className="modal-section-heading">
                        <span className="panel-label">Current members</span>
                        <strong>{memberships.length}</strong>
                    </div>
                    {error && <p className="form-message is-warning">{error}</p>}
                    {loading && <p className="form-message">Loading members...</p>}
                    {!loading && memberships.length === 0 && <p className="form-message">No local members.</p>}
                    {!loading && memberships.length > 0 && (
                        <div className="compact-list">
                            {memberships.map((membership) => (
                                <div className="compact-list-row action-list-row group-member-row" key={membership.id}>
                                    <div className="access-row-subject">
                                        <div>
                                            <strong>{membership.user?.label || membership.user?.username || `User ${membership.user_id}`}</strong>
                                            <span>{membership.user?.email || membership.user?.username || "local member"}</span>
                                        </div>
                                    </div>
                                    <div className="project-access-actions">
                                        <select
                                            className="field-input compact-select access-role-select"
                                            value={String(membership.membership_role_label || membership.membership_role || "member")}
                                            disabled={saving}
                                            aria-label="Group membership role"
                                            onChange={(event) => updateRole(membership, event.currentTarget.value)}
                                        >
                                            <option value="member">Member</option>
                                            <option value="manager">Manager</option>
                                            <option value="owner">Owner</option>
                                        </select>
                                        <RowActionMenu ariaLabel={`${membership.user?.label || membership.user?.username || "Member"} actions`} className="tree-actions access-row-actions" menuClassName="tree-inline-menu">
                                            <button type="button" role="menuitem" className="danger-action" disabled={saving} onClick={() => remove(membership)}>
                                                Remove member
                                            </button>
                                        </RowActionMenu>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </section>
            </div>
        </ModalFrame>
    );
}

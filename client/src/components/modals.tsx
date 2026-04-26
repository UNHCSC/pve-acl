import { useState, type FormEvent } from "react";
import { Field, ModalFrame, Select, SimpleFormModal, Textarea } from "./common";
import type { AccessData, Organization, Project, UserImportResponse } from "../types";
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
            <Select name="parentOrgID" label="Parent organization" defaultValue={context?.id ?? ""}>
                <option value="">Root level</option>
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
    onSubmit,
    onClose
}: {
    onSubmit: (values: { name: string; slug: string; description: string }) => Promise<void>;
    onClose: () => void;
}) {
    return (
        <SimpleFormModal
            title="New group"
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
    onSubmit,
    onClose
}: {
    onSubmit: (values: { name: string; description: string }) => Promise<void>;
    onClose: () => void;
}) {
    return (
        <SimpleFormModal
            title="New role"
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

export function GrantModal({
    access,
    orgs,
    projects,
    onSubmit,
    onClose
}: {
    access: AccessData;
    orgs: Organization[];
    projects: Project[];
    onSubmit: (values: { roleID: number; subjectType: string; subjectRef: string; scopeType: string; scopeID: number | null }) => Promise<void>;
    onClose: () => void;
}) {
    const [scopeType, setScopeType] = useState("global");

    return (
        <SimpleFormModal
            title="New role binding"
            label="Access"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    roleID: Number(data.get("roleID")),
                    subjectType: String(data.get("subjectType") || "user"),
                    subjectRef: String(data.get("subjectRef") || ""),
                    scopeType,
                    scopeID: scopeType === "global" ? null : Number(data.get("scopeID"))
                })
            }
        >
            <Select name="roleID" label="Role" required>
                {access.roles.map((role) => (
                    <option key={role.id} value={role.id}>
                        {role.name}
                    </option>
                ))}
            </Select>
            <Select name="subjectType" label="Subject type">
                <option value="user">User</option>
                <option value="group">Group</option>
            </Select>
            <Field name="subjectRef" label="Subject username or group slug" required />
            <label className="field-group">
                <span className="field-label">Scope</span>
                <select className="field-input" value={scopeType} onChange={(event) => setScopeType(event.target.value)}>
                    <option value="global">Global</option>
                    <option value="org">Organization</option>
                    <option value="project">Project</option>
                </select>
            </label>
            {scopeType === "org" && (
                <Select name="scopeID" label="Organization" required>
                    {orgs.map((org) => (
                        <option key={org.id} value={org.id}>
                            {org.name}
                        </option>
                    ))}
                </Select>
            )}
            {scopeType === "project" && (
                <Select name="scopeID" label="Project" required>
                    {projects.map((project) => (
                        <option key={project.id} value={project.id}>
                            {project.name}
                        </option>
                    ))}
                </Select>
            )}
        </SimpleFormModal>
    );
}

export function ProjectMemberModal({
    defaultSubjectType = "user",
    onSubmit,
    onClose
}: {
    defaultSubjectType?: "user" | "group";
    onSubmit: (values: { subjectType: string; subjectRef: string }) => Promise<void>;
    onClose: () => void;
}) {
    const subjectType = defaultSubjectType;
    const isGroup = subjectType === "group";

    return (
        <SimpleFormModal
            title={isGroup ? "Add project group" : "Add project user"}
            label={isGroup ? "Group access" : "User access"}
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    subjectType,
                    subjectRef: String(data.get("subjectRef") || "")
                })
            }
        >
            <Field name="subjectRef" label={isGroup ? "Group slug" : "Username or email"} required />
        </SimpleFormModal>
    );
}

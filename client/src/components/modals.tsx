import { useState } from "react";
import { Field, Select, SimpleFormModal, Textarea } from "./common";
import type { AccessData, Organization, Project } from "../types";

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

export function UserModal({
    onSubmit,
    onClose
}: {
    onSubmit: (values: { username: string; displayName: string; email: string }) => Promise<void>;
    onClose: () => void;
}) {
    return (
        <SimpleFormModal
            title="New user"
            label="Identity"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    username: String(data.get("username") || ""),
                    displayName: String(data.get("displayName") || ""),
                    email: String(data.get("email") || "")
                })
            }
        >
            <Field name="username" label="Username" required />
            <Field name="displayName" label="Display name" />
            <Field name="email" label="Email" type="email" />
        </SimpleFormModal>
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
    onSubmit: (values: { subjectType: string; subjectRef: string; projectRole: string }) => Promise<void>;
    onClose: () => void;
}) {
    const [subjectType, setSubjectType] = useState<"user" | "group">(defaultSubjectType);

    return (
        <SimpleFormModal
            title="Add project member"
            label="Project access"
            onClose={onClose}
            onSubmit={(data) =>
                onSubmit({
                    subjectType,
                    subjectRef: String(data.get("subjectRef") || ""),
                    projectRole: String(data.get("projectRole") || "viewer")
                })
            }
        >
            <label className="field-group">
                <span className="field-label">Subject type</span>
                <select className="field-input" value={subjectType} onChange={(event) => setSubjectType(event.target.value as "user" | "group")}>
                    <option value="user">User</option>
                    <option value="group">Group</option>
                </select>
            </label>
            <Field name="subjectRef" label={subjectType === "user" ? "Username" : "Group slug"} required />
            <Select name="projectRole" label="Role" defaultValue="viewer">
                {["viewer", "operator", "developer", "manager", "owner"].map((role) => (
                    <option key={role} value={role}>
                        {role}
                    </option>
                ))}
            </Select>
        </SimpleFormModal>
    );
}

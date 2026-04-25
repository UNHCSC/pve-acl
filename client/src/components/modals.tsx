import { useState } from "react";
import { Field, Select, SimpleFormModal, Textarea } from "./common";
import { flattenOrgTree, findOrg, orgContains } from "../tree";
import type { AccessData, Group, OrgNode, Organization, Project, User } from "../types";
import { displayUser } from "../ui-helpers";

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
    users,
    groups,
    onSubmit,
    onClose
}: {
    users: User[];
    groups: Group[];
    onSubmit: (values: { subjectType: string; subjectRef: string; projectRole: string }) => Promise<void>;
    onClose: () => void;
}) {
    const [subjectType, setSubjectType] = useState("user");

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
                <select className="field-input" value={subjectType} onChange={(event) => setSubjectType(event.target.value)}>
                    <option value="user">User</option>
                    <option value="group">Group</option>
                </select>
            </label>
            <Select name="subjectRef" label={subjectType === "user" ? "User" : "Group"} required>
                {(subjectType === "user" ? users : groups).map((subject) => (
                    <option key={subject.id} value={"username" in subject ? subject.username : subject.slug}>
                        {"username" in subject ? displayUser(subject) : subject.name}
                    </option>
                ))}
            </Select>
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

export function MoveOrgModal({
    org,
    orgTree,
    onClose,
    onSubmit
}: {
    org: Organization;
    orgTree: OrgNode[];
    onClose: () => void;
    onSubmit: (parentOrgID: number | null) => Promise<void>;
}) {
    const currentParent = org.parent_org_id ?? null;
    const [targetID, setTargetID] = useState<number | null>(currentParent);
    const sourceNode = findOrg(orgTree, org.id);
    const options = flattenOrgTree(orgTree).filter((option) => {
        if (option.id === org.id) {
            return false;
        }
        return !sourceNode || !orgContains(sourceNode, option.id);
    });

    return (
        <OrgPickerModal
            title={`Move ${org.name}`}
            label="Organization"
            selectedID={targetID}
            allowRoot
            orgs={options}
            rootLabel="Root level"
            onSelect={setTargetID}
            onClose={onClose}
            onSubmit={() => onSubmit(targetID)}
        />
    );
}

export function MoveProjectModal({
    project,
    orgTree,
    onClose,
    onSubmit
}: {
    project: Project;
    orgTree: OrgNode[];
    onClose: () => void;
    onSubmit: (organizationID: number) => Promise<void>;
}) {
    const [targetID, setTargetID] = useState<number | null>(project.organization_id);
    const options = flattenOrgTree(orgTree);

    return (
        <OrgPickerModal
            title={`Move ${project.name}`}
            label="Project"
            selectedID={targetID}
            orgs={options}
            onSelect={setTargetID}
            onClose={onClose}
            onSubmit={() => {
                if (targetID) {
                    return onSubmit(targetID);
                }
                return Promise.resolve();
            }}
        />
    );
}

function OrgPickerModal({
    title,
    label,
    orgs,
    selectedID,
    allowRoot = false,
    rootLabel,
    onSelect,
    onSubmit,
    onClose
}: {
    title: string;
    label: string;
    orgs: Array<Organization & { depth: number }>;
    selectedID: number | null;
    allowRoot?: boolean;
    rootLabel?: string;
    onSelect: (id: number | null) => void;
    onSubmit: () => Promise<void>;
    onClose: () => void;
}) {
    return (
        <SimpleFormModal title={title} label={label} onClose={onClose} onSubmit={() => onSubmit()}>
            <div className="org-picker-list" role="radiogroup" aria-label="Organization destination">
                {allowRoot && (
                    <button type="button" className={selectedID === null ? "is-selected" : ""} onClick={() => onSelect(null)}>
                        <span className="org-picker-indent" />
                        <span>
                            <strong>{rootLabel || "No parent"}</strong>
                            <small>Top of the organization tree</small>
                        </span>
                    </button>
                )}
                {orgs.map((org) => (
                    <button key={org.id} type="button" className={selectedID === org.id ? "is-selected" : ""} onClick={() => onSelect(org.id)}>
                        <span className="org-picker-indent" style={{ width: `${org.depth * 18}px` }} />
                        <span>
                            <strong>{org.name}</strong>
                            <small>{org.slug}</small>
                        </span>
                    </button>
                ))}
            </div>
        </SimpleFormModal>
    );
}

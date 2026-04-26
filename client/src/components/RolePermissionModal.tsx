import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../api";
import type { Permission, Role, RolePermissionGrant } from "../types";
import { EmptyState, ModalFrame } from "./common";

export function RolePermissionModal({
    role,
    editable,
    saveRole,
    onClose
}: {
    role: Role;
    editable: boolean;
    saveRole: (role: Role, values: { name: string; description: string; permissionIDs: number[]; updateDetails: boolean; updatePermissions: boolean }) => Promise<Role>;
    onClose: () => void;
}) {
    const [workingRole, setWorkingRole] = useState(role);
    const [roleName, setRoleName] = useState(role.name);
    const [roleDescription, setRoleDescription] = useState(role.description || "");
    const [selectedPermissionIDs, setSelectedPermissionIDs] = useState<Set<number>>(new Set());
    const [savingRole, setSavingRole] = useState(false);
    const [roleError, setRoleError] = useState("");
    const permissionsQuery = useQuery({
        queryKey: ["permissions"],
        queryFn: () => apiFetch<Permission[]>("/api/v1/permissions")
    });
    const rolePermissionsQuery = useQuery({
        queryKey: ["roles", role.id, "permissions"],
        queryFn: () => apiFetch<RolePermissionGrant[]>(`/api/v1/roles/${role.id}/permissions`)
    });
    const permissionTree = useMemo(() => buildPermissionTree(permissionsQuery.data || []), [permissionsQuery.data]);
    const persistedPermissionIDs = useMemo(
        () => new Set((rolePermissionsQuery.data || []).map((grant) => grant.permission_id)),
        [rolePermissionsQuery.data]
    );
    const permissionsDirty = !setsEqual(persistedPermissionIDs, selectedPermissionIDs);
    const canEdit = editable && !workingRole.is_system_role;
    const roleDetailsDirty = roleName.trim() !== workingRole.name || roleDescription.trim() !== (workingRole.description || "");
    const dirty = roleDetailsDirty || permissionsDirty;

    useEffect(() => {
        setWorkingRole(role);
        setRoleName(role.name);
        setRoleDescription(role.description || "");
        setRoleError("");
    }, [role.id, role.name, role.description]);

    useEffect(() => {
        setSelectedPermissionIDs(new Set((rolePermissionsQuery.data || []).map((grant) => grant.permission_id)));
    }, [rolePermissionsQuery.data, role.id]);

    const togglePermissions = (permissionIDs: number[], checked: boolean) => {
        setSelectedPermissionIDs((current) => {
            const next = new Set(current);
            for (const permissionID of permissionIDs) {
                if (checked) {
                    next.add(permissionID);
                } else {
                    next.delete(permissionID);
                }
            }
            return next;
        });
    };

    const saveChanges = async () => {
        if (!roleName.trim()) {
            setRoleError("Role name is required.");
            return;
        }
        setSavingRole(true);
        setRoleError("");
        try {
            const updatedRole = await saveRole(workingRole, {
                name: roleName,
                description: roleDescription,
                permissionIDs: Array.from(selectedPermissionIDs),
                updateDetails: roleDetailsDirty,
                updatePermissions: permissionsDirty
            });
            setWorkingRole(updatedRole);
            setRoleName(updatedRole.name);
            setRoleDescription(updatedRole.description || "");
            await rolePermissionsQuery.refetch();
        } catch (error) {
            setRoleError(error instanceof Error ? error.message : "Failed to update role");
        } finally {
            setSavingRole(false);
        }
    };

    return (
        <ModalFrame title={canEdit ? "Edit role" : "Role details"} label="Access" onClose={onClose}>
            <div className="role-permission-modal">
                <div className="role-editor-summary">
                    <div>
                        <strong>{workingRole.name}</strong>
                        <span>{workingRole.description || `${workingRole.permission_count || 0} permissions`}</span>
                    </div>
                    <span>{workingRole.is_system_role ? "system role" : workingRole.owner_scope_label || "custom role"}</span>
                </div>
                <div className="role-editor-fields">
                    <label className="field-group">
                        <span className="field-label">Name</span>
                        <input className="field-input" value={roleName} disabled={!canEdit || savingRole} onChange={(event) => setRoleName(event.currentTarget.value)} />
                    </label>
                    <label className="field-group">
                        <span className="field-label">Description</span>
                        <textarea className="field-input" value={roleDescription} disabled={!canEdit || savingRole} rows={3} onChange={(event) => setRoleDescription(event.currentTarget.value)} />
                    </label>
                </div>
                {workingRole.is_system_role && <p className="form-message">System role permissions are managed by setup. You can view them here.</p>}
                {!editable && !workingRole.is_system_role && <p className="form-message">This role is inherited from another scope and is read-only here.</p>}
                {roleError && <p className="form-message is-warning">{roleError}</p>}
                {(permissionsQuery.isLoading || rolePermissionsQuery.isLoading) && <EmptyState>Loading permissions...</EmptyState>}
                {(permissionsQuery.error || rolePermissionsQuery.error) && <p className="form-message is-warning">Failed to load role permissions.</p>}
                {!permissionsQuery.isLoading && !rolePermissionsQuery.isLoading && !permissionsQuery.error && !rolePermissionsQuery.error && (
                    <PermissionTree
                        editable={canEdit}
                        nodes={permissionTree}
                        saving={savingRole}
                        selectedPermissionIDs={selectedPermissionIDs}
                        togglePermissions={togglePermissions}
                    />
                )}
                <div className="modal-actions">
                    <button type="button" className="button-secondary" onClick={onClose} disabled={savingRole}>
                        Close
                    </button>
                    {canEdit && (
                        <button className="button-primary" type="button" disabled={!dirty || savingRole || rolePermissionsQuery.isFetching} onClick={saveChanges}>
                            Save role
                        </button>
                    )}
                </div>
            </div>
        </ModalFrame>
    );
}

function PermissionTree({
    nodes,
    selectedPermissionIDs,
    editable,
    saving,
    togglePermissions
}: {
    nodes: PermissionTreeNode[];
    selectedPermissionIDs: Set<number>;
    editable: boolean;
    saving: boolean;
    togglePermissions: (permissionIDs: number[], checked: boolean) => void;
}) {
    if (nodes.length === 0) {
        return <EmptyState>No permissions registered.</EmptyState>;
    }

    return (
        <ul className="permission-tree">
            {nodes.map((node) => (
                <PermissionTreeItem
                    key={node.path}
                    node={node}
                    depth={0}
                    editable={editable}
                    saving={saving}
                    selectedPermissionIDs={selectedPermissionIDs}
                    togglePermissions={togglePermissions}
                />
            ))}
        </ul>
    );
}

function PermissionTreeItem({
    node,
    depth,
    selectedPermissionIDs,
    editable,
    saving,
    togglePermissions
}: {
    node: PermissionTreeNode;
    depth: number;
    selectedPermissionIDs: Set<number>;
    editable: boolean;
    saving: boolean;
    togglePermissions: (permissionIDs: number[], checked: boolean) => void;
}) {
    const permissionIDs = collectPermissionIDs(node);
    const selectedCount = permissionIDs.filter((permissionID) => selectedPermissionIDs.has(permissionID)).length;
    const checked = permissionIDs.length > 0 && selectedCount === permissionIDs.length;
    const indeterminate = selectedCount > 0 && selectedCount < permissionIDs.length;

    if (node.permission && node.children.length === 0) {
        return (
            <li className={`permission-tree-item ${depth > 0 ? "has-parent" : ""}`} style={{ marginLeft: depth > 0 ? "24px" : 0 }}>
                <label className="permission-tree-leaf">
                    <input
                        type="checkbox"
                        checked={selectedPermissionIDs.has(node.permission.id)}
                        disabled={!editable || saving}
                        onChange={(event) => togglePermissions([node.permission!.id], event.currentTarget.checked)}
                    />
                    <span>{node.label}</span>
                    <small>{node.path}</small>
                </label>
            </li>
        );
    }

    return (
        <li className={`permission-tree-branch permission-tree-item ${depth > 0 ? "has-parent" : ""}`} style={{ marginLeft: depth > 0 ? "24px" : 0 }}>
            <details open>
                <summary>
                    <TreeCheckbox
                        checked={checked}
                        disabled={!editable || saving}
                        indeterminate={indeterminate}
                        onChange={(checkedValue) => togglePermissions(permissionIDs, checkedValue)}
                    />
                    <span>{node.label}</span>
                    <small>{selectedCount}/{permissionIDs.length}</small>
                </summary>
                <ul className="permission-tree-children">
                    {node.permission && (
                        <PermissionTreeItem
                            node={{ children: [], label: node.path, path: node.path, permission: node.permission }}
                            depth={depth + 1}
                            editable={editable}
                            saving={saving}
                            selectedPermissionIDs={selectedPermissionIDs}
                            togglePermissions={togglePermissions}
                        />
                    )}
                    {node.children.map((child) => (
                        <PermissionTreeItem
                            key={child.path}
                            node={child}
                            depth={depth + 1}
                            editable={editable}
                            saving={saving}
                            selectedPermissionIDs={selectedPermissionIDs}
                            togglePermissions={togglePermissions}
                        />
                    ))}
                </ul>
            </details>
        </li>
    );
}

function TreeCheckbox({
    checked,
    disabled,
    indeterminate,
    onChange
}: {
    checked: boolean;
    disabled: boolean;
    indeterminate: boolean;
    onChange: (checked: boolean) => void;
}) {
    const ref = useRef<HTMLInputElement>(null);

    useEffect(() => {
        if (ref.current) {
            ref.current.indeterminate = indeterminate;
        }
    }, [indeterminate]);

    return (
        <input
            ref={ref}
            type="checkbox"
            checked={checked}
            disabled={disabled}
            onClick={(event) => event.stopPropagation()}
            onChange={(event) => onChange(event.currentTarget.checked)}
        />
    );
}

type PermissionTreeNode = {
    children: PermissionTreeNode[];
    label: string;
    path: string;
    permission?: Permission;
};

type MutablePermissionTreeNode = {
    children: Map<string, MutablePermissionTreeNode>;
    label: string;
    path: string;
    permission?: Permission;
};

function buildPermissionTree(permissions: Permission[]): PermissionTreeNode[] {
    const roots = new Map<string, MutablePermissionTreeNode>();
    const sortedPermissions = [...permissions].sort((left, right) => left.name.localeCompare(right.name));

    for (const permission of sortedPermissions) {
        const segments = permission.name.split(".").filter(Boolean);
        if (segments.length === 0) {
            continue;
        }
        let level = roots;
        let node: MutablePermissionTreeNode | undefined;
        const path: string[] = [];
        for (const segment of segments) {
            path.push(segment);
            node = level.get(segment);
            if (!node) {
                node = {
                    children: new Map(),
                    label: segment,
                    path: path.join(".")
                };
                level.set(segment, node);
            }
            level = node.children;
        }
        if (node) {
            node.permission = permission;
        }
    }

    return Array.from(roots.values()).map(finalizePermissionTreeNode);
}

function finalizePermissionTreeNode(node: MutablePermissionTreeNode): PermissionTreeNode {
    return {
        children: Array.from(node.children.values()).map(finalizePermissionTreeNode),
        label: node.label,
        path: node.path,
        permission: node.permission
    };
}

function collectPermissionIDs(node: PermissionTreeNode): number[] {
    return [
        ...(node.permission ? [node.permission.id] : []),
        ...node.children.flatMap(collectPermissionIDs)
    ];
}

function setsEqual(left: Set<number>, right: Set<number>) {
    if (left.size !== right.size) {
        return false;
    }
    for (const value of left) {
        if (!right.has(value)) {
            return false;
        }
    }
    return true;
}

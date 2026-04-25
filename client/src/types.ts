export type ToastKind = "info" | "success" | "warning";
export type ViewKey = "overview" | "directory" | "people" | "identity" | "access";
export type ThemeKey = "light" | "dark" | "proxmox-light" | "proxmox-dark";
export type ModalKey = "org" | "project" | "user" | "group" | "role" | "grant" | "project-member" | null;
export type Selection =
    | { type: "org"; id: number }
    | { type: "project"; id: number; slug: string }
    | null;

export type Summary = {
    counts: Record<string, number>;
    currentUser: {
        id: number;
        username: string;
        displayName?: string;
        email?: string;
        authSource?: string;
        groupCount?: number;
        isSiteAdmin?: boolean;
    };
    capabilities: {
        canCreateProjects?: boolean;
        canManageUsers?: boolean;
        canManageGroups?: boolean;
        canManageRoles?: boolean;
        canManageOrgs?: boolean;
        canViewUsers?: boolean;
        canViewAccess?: boolean;
    };
};

export type Organization = {
    id: number;
    uuid?: string;
    name: string;
    slug: string;
    description?: string;
    parent_org_id: number | null;
    created_at?: string;
    updated_at?: string;
};

export type Project = {
    id: number;
    uuid?: string;
    organization_id: number;
    name: string;
    slug: string;
    project_type?: number | string;
    description?: string;
    is_active?: boolean;
    created_at?: string;
    updated_at?: string;
    organization?: Organization;
};

export type ProjectTree = {
    organizations: Organization[];
    projects: Project[];
};

export type User = {
    id: number;
    username: string;
    displayName?: string;
    display_name?: string;
    email?: string;
    authSource?: string;
    auth_source?: string;
};

export type Group = {
    id: number;
    name: string;
    slug: string;
    description?: string;
    group_type?: number | string;
    group_type_label?: string;
    member_count?: number;
    role_binding_count?: number;
};

export type Role = {
    id: number;
    name: string;
    description?: string;
    is_system_role?: boolean;
    permission_count?: number;
};

export type Permission = {
    id: number;
    name: string;
    description?: string;
};

export type RoleBinding = {
    id: number;
    role_id: number;
    role?: Role;
    subject_type: number | string;
    subject_type_label?: string;
    subject_id: number;
    subject?: { label?: string; name?: string; username?: string; slug?: string; meta?: string };
    scope_type: number | string;
    scope_type_label?: string;
    scope_id?: number | null;
};

export type AccessData = {
    groups: Group[];
    roles: Role[];
    permissions: Permission[];
    roleBindings: RoleBinding[];
};

export type MyAccess = {
    groups: Group[];
    roles: Role[];
    roleBindings: RoleBinding[];
    isSiteAdmin?: boolean;
};

export type ProjectMembership = {
    id: number;
    project_id: number;
    subject_type: number | string;
    subject_id: number;
    subject?: { label?: string; meta?: string; username?: string; slug?: string; name?: string };
    project_role: number | string;
    project_role_label?: string;
};

export type OrgNode = Organization & { children: OrgNode[]; projects: Project[] };

export const viewTitles: Record<ViewKey, string> = {
    overview: "Overview",
    directory: "Directory",
    people: "People",
    identity: "Identity",
    access: "Access"
};

import type { OrgNode, Organization, Project, ProjectTree, Selection } from "./types";

export function buildOrgTree(tree: ProjectTree | null): OrgNode[] {
    if (!tree) {
        return [];
    }
    const nodes = new Map<number, OrgNode>();
    for (const org of tree.organizations) {
        nodes.set(org.id, { ...org, children: [], projects: [] });
    }
    for (const project of tree.projects) {
        nodes.get(project.organization_id)?.projects.push(project);
    }
    const roots: OrgNode[] = [];
    for (const node of nodes.values()) {
        if (node.parent_org_id && nodes.has(node.parent_org_id)) {
            nodes.get(node.parent_org_id)?.children.push(node);
        } else {
            roots.push(node);
        }
    }
    const sortNodes = (items: OrgNode[]) => {
        items.sort((a, b) => a.name.localeCompare(b.name));
        for (const item of items) {
            item.children.sort((a, b) => a.name.localeCompare(b.name));
            item.projects.sort((a, b) => a.name.localeCompare(b.name));
            sortNodes(item.children);
        }
    };
    sortNodes(roots);
    return roots;
}

export function findOrg(nodes: OrgNode[], id: number): OrgNode | null {
    for (const node of nodes) {
        if (node.id === id) {
            return node;
        }
        const child = findOrg(node.children, id);
        if (child) {
            return child;
        }
    }
    return null;
}

export function findProject(tree: ProjectTree | null, id: number): Project | null {
    return tree?.projects.find((project) => project.id === id) || null;
}

export function flattenOrgTree(nodes: OrgNode[]): Array<Organization & { depth: number }> {
    const items: Array<Organization & { depth: number }> = [];
    const visit = (node: OrgNode, depth: number) => {
        items.push({ ...node, depth });
        for (const child of node.children) {
            visit(child, depth + 1);
        }
    };
    for (const node of nodes) {
        visit(node, 0);
    }
    return items;
}

export function orgContains(root: OrgNode, orgID: number): boolean {
    return root.id === orgID || root.children.some((child) => orgContains(child, orgID));
}

export function defaultSelection(tree: ProjectTree): Selection {
    const firstProject = tree.projects[0];
    if (firstProject) {
        return { type: "project", id: firstProject.id, slug: firstProject.slug };
    }
    const firstOrg = tree.organizations[0];
    return firstOrg ? { type: "org", id: firstOrg.id } : null;
}

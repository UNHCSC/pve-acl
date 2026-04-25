import { useEffect, useMemo, useState } from "react";
import { buildOrgTree, defaultSelection, findOrg, findProject } from "../tree";
import type { ProjectTree, Selection } from "../types";

export function useDirectorySelection(tree: ProjectTree | null) {
    const [selection, setSelection] = useState<Selection>(null);
    const [expanded, setExpanded] = useState<Set<number>>(new Set());
    const orgTree = useMemo(() => buildOrgTree(tree), [tree]);
    const selectedOrg = selection?.type === "org" ? findOrg(orgTree, selection.id) : null;
    const selectedProject = selection?.type === "project" ? findProject(tree, selection.id) : null;

    useEffect(() => {
        if (!tree) {
            return;
        }
        setExpanded((previous) => {
            if (previous.size > 0) {
                return previous;
            }
            return new Set(tree.organizations.map((org) => org.id));
        });
        setSelection((previous) => previous || defaultSelection(tree));
    }, [tree]);

    const toggleOrg = (id: number) => {
        setExpanded((previous) => {
            const next = new Set(previous);
            if (next.has(id)) {
                next.delete(id);
            } else {
                next.add(id);
            }
            return next;
        });
    };

    return {
        expanded,
        orgTree,
        selectedOrg,
        selectedProject,
        selection,
        setSelection,
        toggleOrg
    };
}

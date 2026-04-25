import { useEffect, useState } from "react";
import { apiFetch } from "../api";
import type { AccessData, MyAccess, ProjectTree, Summary, User } from "../types";

export function useDashboardData(showError: (message: string) => void) {
    const [summary, setSummary] = useState<Summary | null>(null);
    const [tree, setTree] = useState<ProjectTree | null>(null);
    const [access, setAccess] = useState<AccessData>({ groups: [], roles: [], permissions: [], roleBindings: [] });
    const [myAccess, setMyAccess] = useState<MyAccess | null>(null);
    const [users, setUsers] = useState<User[]>([]);
    const [loading, setLoading] = useState(true);

    const loadSummary = async () => {
        setSummary(await apiFetch<Summary>("/api/v1/system/summary"));
    };

    const loadTree = async () => {
        const nextTree = await apiFetch<ProjectTree>("/api/v1/projects/tree");
        setTree(nextTree);
        return nextTree;
    };

    const loadAccess = async () => {
        setAccess(await apiFetch<AccessData>("/api/v1/system/access"));
    };

    const loadMyAccess = async () => {
        setMyAccess(await apiFetch<MyAccess>("/api/v1/users/me/access"));
    };

    const loadUsers = async () => {
        setUsers(await apiFetch<User[]>("/api/v1/users"));
    };

    const refreshAll = async () => {
        setLoading(true);
        try {
            await Promise.all([loadSummary(), loadTree(), loadAccess(), loadMyAccess(), loadUsers()]);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        refreshAll().catch((error) => showError(error instanceof Error ? error.message : "Failed to load dashboard"));
    }, []);

    return {
        access,
        loadAccess,
        loadMyAccess,
        loadSummary,
        loadTree,
        loadUsers,
        loading,
        myAccess,
        refreshAll,
        summary,
        tree,
        users
    };
}

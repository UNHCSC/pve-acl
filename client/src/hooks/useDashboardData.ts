import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../api";
import type { MyAccess, ProjectTree, Summary, User } from "../types";

export function useDashboardData(showError: (message: string) => void) {
    const queryClient = useQueryClient();
    const summaryQuery = useQuery({
        queryKey: ["system", "summary"],
        queryFn: () => apiFetch<Summary>("/api/v1/system/summary")
    });
    const summary = summaryQuery.data ?? null;
    const treeQuery = useQuery({
        queryKey: ["projects", "tree"],
        queryFn: () => apiFetch<ProjectTree>("/api/v1/projects/tree"),
        enabled: Boolean(summary)
    });
    const myAccessQuery = useQuery({
        queryKey: ["users", "me", "access"],
        queryFn: () => apiFetch<MyAccess>("/api/v1/users/me/access"),
        enabled: Boolean(summary)
    });
    const usersQuery = useQuery({
        queryKey: ["users"],
        queryFn: () => apiFetch<User[]>("/api/v1/users"),
        enabled: Boolean(summary?.capabilities.canViewUsers)
    });

    const loadSummary = () => queryClient.fetchQuery({ queryKey: ["system", "summary"], queryFn: () => apiFetch<Summary>("/api/v1/system/summary") });
    const loadTree = () => queryClient.fetchQuery({ queryKey: ["projects", "tree"], queryFn: () => apiFetch<ProjectTree>("/api/v1/projects/tree") });
    const loadMyAccess = () => queryClient.fetchQuery({ queryKey: ["users", "me", "access"], queryFn: () => apiFetch<MyAccess>("/api/v1/users/me/access") });
    const loadUsers = () => queryClient.fetchQuery({ queryKey: ["users"], queryFn: () => apiFetch<User[]>("/api/v1/users") });

    const refreshAll = async () => {
        const nextSummary = await loadSummary();
        await Promise.all([
            loadTree(),
            loadMyAccess(),
            nextSummary.capabilities.canViewUsers ? loadUsers() : queryClient.removeQueries({ queryKey: ["users"], exact: true })
        ]);
    };

    useEffect(() => {
        const errors = [summaryQuery.error, treeQuery.error, myAccessQuery.error, usersQuery.error].filter(Boolean);
        if (errors[0]) {
            showError(errors[0] instanceof Error ? errors[0].message : "Failed to load dashboard");
        }
    }, [summaryQuery.error, treeQuery.error, myAccessQuery.error, usersQuery.error]);

    return {
        loadMyAccess,
        loadSummary,
        loadTree,
        loadUsers,
        loading: summaryQuery.isLoading || treeQuery.isLoading || myAccessQuery.isLoading || usersQuery.isLoading,
        myAccess: myAccessQuery.data ?? null,
        refreshAll,
        summary,
        tree: treeQuery.data ?? null,
        users: usersQuery.data ?? []
    };
}

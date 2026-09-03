import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../api";
import type { AssetAssignment, AssetGroup, Project, ProjectResource } from "../types";

export function useProjectAssets(selectedProject: Project | null, showError: (message: string) => void) {
    const queryClient = useQueryClient();
    const resourcesQuery = useQuery({
        queryKey: ["projects", selectedProject?.id, "resources"],
        queryFn: () => apiFetch<ProjectResource[]>(`/api/v1/projects/${selectedProject?.id}/resources`),
        enabled: Boolean(selectedProject?.id)
    });
    const assetGroupsQuery = useQuery({
        queryKey: ["projects", selectedProject?.id, "asset-groups"],
        queryFn: () => apiFetch<AssetGroup[]>(`/api/v1/projects/${selectedProject?.id}/asset-groups`),
        enabled: Boolean(selectedProject?.id)
    });
    const assetAssignmentsQuery = useQuery({
        queryKey: ["projects", selectedProject?.id, "asset-assignments"],
        queryFn: () => apiFetch<AssetAssignment[]>(`/api/v1/projects/${selectedProject?.id}/asset-assignments`),
        enabled: Boolean(selectedProject?.id)
    });

    const reloadProjectResources = async () => {
        if (!selectedProject) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["projects", selectedProject.id, "resources"],
            queryFn: () => apiFetch<ProjectResource[]>(`/api/v1/projects/${selectedProject.id}/resources`)
        });
    };

    const reloadProjectAssetGroups = async () => {
        if (!selectedProject) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["projects", selectedProject.id, "asset-groups"],
            queryFn: () => apiFetch<AssetGroup[]>(`/api/v1/projects/${selectedProject.id}/asset-groups`)
        });
    };

    const reloadProjectAssetAssignments = async () => {
        if (!selectedProject) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["projects", selectedProject.id, "asset-assignments"],
            queryFn: () => apiFetch<AssetAssignment[]>(`/api/v1/projects/${selectedProject.id}/asset-assignments`)
        });
    };

    const reloadProjectAssets = async () => {
        await Promise.all([
            reloadProjectResources(),
            reloadProjectAssetGroups(),
            reloadProjectAssetAssignments()
        ]);
    };

    useEffect(() => {
        const error = resourcesQuery.error || assetGroupsQuery.error || assetAssignmentsQuery.error;
        if (error) {
            showError(error instanceof Error ? error.message : "Failed to load project assets");
        }
    }, [resourcesQuery.error, assetGroupsQuery.error, assetAssignmentsQuery.error]);

    return {
        assetAssignments: assetAssignmentsQuery.data ?? [],
        assetGroups: assetGroupsQuery.data ?? [],
        loadingProjectAssets: resourcesQuery.isLoading || assetGroupsQuery.isLoading || assetAssignmentsQuery.isLoading,
        projectResources: resourcesQuery.data ?? [],
        reloadProjectAssetAssignments,
        reloadProjectAssetGroups,
        reloadProjectAssets,
        reloadProjectResources
    };
}

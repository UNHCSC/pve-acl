import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../api";
import type { Group, Project, ProjectMembership, Role } from "../types";

export function useProjectMemberships(selectedProject: Project | null, showError: (message: string) => void) {
    const queryClient = useQueryClient();
    const projectDetailQuery = useQuery({
        queryKey: ["projects", selectedProject?.slug],
        queryFn: () => apiFetch<Project>(`/api/v1/projects/${encodeURIComponent(selectedProject?.slug || "")}`),
        enabled: Boolean(selectedProject?.slug)
    });
    const membershipsQuery = useQuery({
        queryKey: ["projects", selectedProject?.id, "memberships"],
        queryFn: () => apiFetch<ProjectMembership[]>(`/api/v1/projects/${selectedProject?.id}/memberships`),
        enabled: Boolean(selectedProject?.id)
    });
    const rolesQuery = useQuery({
        queryKey: ["projects", selectedProject?.id, "roles"],
        queryFn: () => apiFetch<Role[]>(`/api/v1/projects/${selectedProject?.id}/roles`),
        enabled: Boolean(selectedProject?.id)
    });
    const groupsQuery = useQuery({
        queryKey: ["projects", selectedProject?.id, "groups"],
        queryFn: () => apiFetch<Group[]>(`/api/v1/projects/${selectedProject?.id}/groups`),
        enabled: Boolean(selectedProject?.id)
    });

    const reloadMemberships = async () => {
        if (!selectedProject) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["projects", selectedProject.id, "memberships"],
            queryFn: () => apiFetch<ProjectMembership[]>(`/api/v1/projects/${selectedProject.id}/memberships`)
        });
    };
    const reloadProjectRoles = async () => {
        if (!selectedProject) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["projects", selectedProject.id, "roles"],
            queryFn: () => apiFetch<Role[]>(`/api/v1/projects/${selectedProject.id}/roles`)
        });
    };
    const reloadProjectGroups = async () => {
        if (!selectedProject) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["projects", selectedProject.id, "groups"],
            queryFn: () => apiFetch<Group[]>(`/api/v1/projects/${selectedProject.id}/groups`)
        });
    };

    useEffect(() => {
        const error = projectDetailQuery.error || membershipsQuery.error || rolesQuery.error || groupsQuery.error;
        if (error) {
            showError(error instanceof Error ? error.message : "Failed to load project");
        }
    }, [projectDetailQuery.error, membershipsQuery.error, rolesQuery.error, groupsQuery.error]);

    const activeProject = selectedProject && projectDetailQuery.data ? { ...selectedProject, ...projectDetailQuery.data } : selectedProject;

    return {
        activeProject,
        loadingProject: projectDetailQuery.isLoading || membershipsQuery.isLoading || rolesQuery.isLoading || groupsQuery.isLoading,
        memberships: membershipsQuery.data ?? [],
        projectGroups: groupsQuery.data ?? [],
        projectRoles: rolesQuery.data ?? [],
        reloadMemberships,
        reloadProjectGroups,
        reloadProjectRoles
    };
}

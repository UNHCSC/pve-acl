import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../api";
import type { Project, ProjectMembership } from "../types";

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

    const reloadMemberships = async () => {
        if (!selectedProject) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["projects", selectedProject.id, "memberships"],
            queryFn: () => apiFetch<ProjectMembership[]>(`/api/v1/projects/${selectedProject.id}/memberships`)
        });
    };

    useEffect(() => {
        const error = projectDetailQuery.error || membershipsQuery.error;
        if (error) {
            showError(error instanceof Error ? error.message : "Failed to load project");
        }
    }, [projectDetailQuery.error, membershipsQuery.error]);

    const activeProject = selectedProject && projectDetailQuery.data ? { ...selectedProject, ...projectDetailQuery.data } : selectedProject;

    return {
        activeProject,
        loadingProject: projectDetailQuery.isLoading || membershipsQuery.isLoading,
        memberships: membershipsQuery.data ?? [],
        reloadMemberships
    };
}

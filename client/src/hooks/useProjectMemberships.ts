import { useEffect, useState } from "react";
import { apiFetch } from "../api";
import type { Project, ProjectMembership } from "../types";

export function useProjectMemberships(selectedProject: Project | null, showError: (message: string) => void) {
    const [activeProject, setActiveProject] = useState<Project | null>(null);
    const [memberships, setMemberships] = useState<ProjectMembership[]>([]);
    const [loadingProject, setLoadingProject] = useState(false);

    const loadProject = async (project: Project) => {
        setLoadingProject(true);
        try {
            const [detail, projectMemberships] = await Promise.all([
                apiFetch<Project>(`/api/v1/projects/${encodeURIComponent(project.slug)}`),
                apiFetch<ProjectMembership[]>(`/api/v1/projects/${project.id}/memberships`)
            ]);
            setActiveProject({ ...project, ...detail });
            setMemberships(projectMemberships);
        } catch (error) {
            showError(error instanceof Error ? error.message : "Failed to load project");
        } finally {
            setLoadingProject(false);
        }
    };

    const reloadMemberships = async () => {
        if (!activeProject) {
            return;
        }
        setMemberships(await apiFetch<ProjectMembership[]>(`/api/v1/projects/${activeProject.id}/memberships`));
    };

    useEffect(() => {
        if (selectedProject) {
            loadProject(selectedProject);
        } else {
            setActiveProject(null);
            setMemberships([]);
        }
    }, [selectedProject?.id]);

    return {
        activeProject,
        loadingProject,
        memberships,
        reloadMemberships
    };
}

import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../api";
import type { Group, OrgNode, OrganizationMembership, Role } from "../types";

export function useOrganizationAccess(selectedOrg: OrgNode | null, showError: (message: string) => void) {
    const queryClient = useQueryClient();
    const membershipsQuery = useQuery({
        queryKey: ["organizations", selectedOrg?.id, "memberships"],
        queryFn: () => apiFetch<OrganizationMembership[]>(`/api/v1/organizations/${selectedOrg?.id}/memberships`),
        enabled: Boolean(selectedOrg?.id)
    });
    const rolesQuery = useQuery({
        queryKey: ["organizations", selectedOrg?.id, "roles"],
        queryFn: () => apiFetch<Role[]>(`/api/v1/organizations/${selectedOrg?.id}/roles`),
        enabled: Boolean(selectedOrg?.id)
    });
    const groupsQuery = useQuery({
        queryKey: ["organizations", selectedOrg?.id, "groups"],
        queryFn: () => apiFetch<Group[]>(`/api/v1/organizations/${selectedOrg?.id}/groups`),
        enabled: Boolean(selectedOrg?.id)
    });

    const reloadOrgMemberships = async () => {
        if (!selectedOrg) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["organizations", selectedOrg.id, "memberships"],
            queryFn: () => apiFetch<OrganizationMembership[]>(`/api/v1/organizations/${selectedOrg.id}/memberships`)
        });
    };
    const reloadOrgRoles = async () => {
        if (!selectedOrg) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["organizations", selectedOrg.id, "roles"],
            queryFn: () => apiFetch<Role[]>(`/api/v1/organizations/${selectedOrg.id}/roles`)
        });
    };
    const reloadOrgGroups = async () => {
        if (!selectedOrg) {
            return;
        }
        await queryClient.fetchQuery({
            queryKey: ["organizations", selectedOrg.id, "groups"],
            queryFn: () => apiFetch<Group[]>(`/api/v1/organizations/${selectedOrg.id}/groups`)
        });
    };

    useEffect(() => {
        const error = membershipsQuery.error || rolesQuery.error || groupsQuery.error;
        if (error) {
            showError(error instanceof Error ? error.message : "Failed to load organization access");
        }
    }, [membershipsQuery.error, rolesQuery.error, groupsQuery.error]);

    return {
        loadingOrg: membershipsQuery.isLoading || rolesQuery.isLoading || groupsQuery.isLoading,
        orgGroups: groupsQuery.data ?? [],
        orgMemberships: membershipsQuery.data ?? [],
        orgRoles: rolesQuery.data ?? [],
        reloadOrgGroups,
        reloadOrgMemberships,
        reloadOrgRoles
    };
}

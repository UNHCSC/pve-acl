import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { apiFetch } from "../api";
import { EmptyState, PanelHeading, TextButton } from "../components/common";
import type { Job, JobLog } from "../types";

const statusNames = ["queued", "running", "succeeded", "failed", "cancelled"];
const timestamp = (value?: string) => value ? new Date(value).toLocaleString() : "—";

export function OperationsView({ isSiteAdmin, showToast }: { isSiteAdmin: boolean; showToast: (message: string, kind: "success" | "warning") => void }) {
    const queryClient = useQueryClient();
    const [selectedID, setSelectedID] = useState<number | null>(null);
    const jobsQuery = useQuery({ queryKey: ["jobs"], queryFn: () => apiFetch<Job[]>("/api/v1/jobs"), refetchInterval: 2000 });
    const logsQuery = useQuery({ queryKey: ["jobs", selectedID, "logs"], queryFn: () => apiFetch<JobLog[]>(`/api/v1/jobs/${selectedID}/logs`), enabled: selectedID !== null, refetchInterval: 2000 });
    const jobs = jobsQuery.data || [];
    const selected = jobs.find((job) => job.id === selectedID) || null;
    const demoMutation = useMutation({
        mutationFn: () => apiFetch<Job>("/api/v1/jobs/demo", { method: "POST", headers: { "Idempotency-Key": crypto.randomUUID() } }),
        onSuccess: (job) => { setSelectedID(job.id); queryClient.invalidateQueries({ queryKey: ["jobs"] }); showToast("Demonstration job queued", "success"); },
        onError: (error) => showToast(error instanceof Error ? error.message : "Failed to queue job", "warning")
    });
    const cancelMutation = useMutation({
        mutationFn: (id: number) => apiFetch<Job>(`/api/v1/jobs/${id}/cancel`, { method: "POST", body: JSON.stringify({ confirm: true }) }),
        onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["jobs"] }); showToast("Cancellation requested", "success"); },
        onError: (error) => showToast(error instanceof Error ? error.message : "Cancellation failed", "warning")
    });

    return <section className="dashboard-view is-active operations-view">
        <article className="dashboard-panel">
            <PanelHeading label="Durable history" title="Operations" action={isSiteAdmin ? <TextButton onClick={() => demoMutation.mutate()}>{demoMutation.isPending ? "Queuing…" : "Run test job"}</TextButton> : undefined} />
            {jobs.length === 0 ? <EmptyState>No visible operations yet.</EmptyState> : <div className="data-table jobs-table">
                <div className="data-table-head"><span>Operation</span><span>Status</span><span>Progress</span><span>Updated</span></div>
                {jobs.map((job) => <button type="button" className={`data-table-row ${selectedID === job.id ? "is-selected" : ""}`} key={job.id} onClick={() => setSelectedID(job.id)}>
                    <span><strong>{job.operation || "operation"}</strong><span>Job #{job.id} · requester #{job.requested_by_user_id || "system"}{job.resource_id ? ` · resource #${job.resource_id}` : job.project_id ? ` · project #${job.project_id}` : ""}{job.node ? ` · ${job.node}` : ""}</span></span>
                    <span><strong>{statusNames[job.status] || "unknown"}</strong><span>Attempt {job.attempt_count}/{job.max_attempts}</span></span>
                    <span><strong>{job.progress}%</strong><span>{job.retry_class || "not classified"}</span></span>
                    <span><strong>{timestamp(job.updated_at)}</strong><span>{job.error_code || "No error"}</span></span>
                </button>)}
            </div>}
        </article>
        {selected && <article className="dashboard-panel">
            <PanelHeading label={`Job #${selected.id}`} title={selected.operation || "Operation detail"} action={(selected.status === 0 || selected.status === 1) ? <TextButton onClick={() => window.confirm("Request cancellation at the next safe checkpoint?") && cancelMutation.mutate(selected.id)}>Cancel</TextButton> : undefined} />
            <dl className="detail-list"><div><dt>Created</dt><dd>{timestamp(selected.created_at)}</dd></div><div><dt>Started</dt><dd>{timestamp(selected.started_at)}</dd></div><div><dt>Finished</dt><dd>{timestamp(selected.finished_at)}</dd></div></dl>
            {selected.error_summary && <p className="inventory-warning">{selected.error_summary}</p>}
            <div className="job-log" aria-label="Operation log">{(logsQuery.data || []).length === 0 ? <EmptyState>No log entries yet.</EmptyState> : (logsQuery.data || []).map((entry) => <div key={entry.id}><time>{timestamp(entry.created_at)}</time><code>{entry.message}</code></div>)}</div>
        </article>}
    </section>;
}

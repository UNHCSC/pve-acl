import { FormEvent, ReactNode, useState } from "react";
import type { ToastKind } from "../types";
import { classNames } from "../ui-helpers";

export function PanelHeading({ label, title, action }: { label: string; title: ReactNode; action?: ReactNode }) {
    return (
        <div className="panel-heading compact-heading">
            <div>
                <span className="panel-label">{label}</span>
                <h2>{title}</h2>
            </div>
            {action}
        </div>
    );
}

export function TextButton({ children, onClick }: { children: ReactNode; onClick: () => void }) {
    return (
        <button type="button" className="button-secondary compact-button" onClick={onClick}>
            {children}
        </button>
    );
}

export function EmptyState({ children }: { children: ReactNode }) {
    return <p className="empty-state">{children}</p>;
}

export function EmptyDetail({ title, text }: { title: string; text: string }) {
    return (
        <article className="dashboard-panel empty-detail">
            <PanelHeading label="Directory" title={title} />
            <p>{text}</p>
        </article>
    );
}

export function Detail({ label, children }: { label: string; children: ReactNode }) {
    return (
        <div>
            <dt>{label}</dt>
            <dd>{children}</dd>
        </div>
    );
}

export function CompactList<T extends { id: number }>({ items, render }: { items: T[]; render: (item: T) => ReactNode }) {
    if (items.length === 0) {
        return <EmptyState>No records found.</EmptyState>;
    }

    return (
        <div className="compact-list">
            {items.map((item) => (
                <div className="compact-list-row" key={item.id}>
                    {render(item)}
                </div>
            ))}
        </div>
    );
}

export function ToastStack({ toasts }: { toasts: Array<{ id: number; text: string; kind: ToastKind }> }) {
    return (
        <div className="toast-stack">
            {toasts.map((toast) => (
                <div key={toast.id} className={classNames("toast-message", toast.kind === "warning" && "is-warning", toast.kind === "success" && "is-success")}>
                    {toast.text}
                </div>
            ))}
        </div>
    );
}

export function ModalFrame({ title, label, children, onClose }: { title: string; label: string; children: ReactNode; onClose: () => void }) {
    return (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
            <section className="modal-panel" role="dialog" aria-modal="true" aria-label={title}>
                <div className="panel-heading">
                    <div>
                        <span className="panel-label">{label}</span>
                        <h2>{title}</h2>
                    </div>
                    <button type="button" className="icon-button" aria-label="Close modal" onClick={onClose}>
                        x
                    </button>
                </div>
                {children}
            </section>
        </div>
    );
}

export function SimpleFormModal({
    title,
    label,
    children,
    onSubmit,
    onClose
}: {
    title: string;
    label: string;
    children: ReactNode;
    onSubmit: (data: FormData) => Promise<void>;
    onClose: () => void;
}) {
    const [submitting, setSubmitting] = useState(false);
    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        setSubmitting(true);
        await onSubmit(new FormData(event.currentTarget));
    };

    return (
        <ModalFrame title={title} label={label} onClose={onClose}>
            <form className="modal-form" onSubmit={submit}>
                {children}
                <ModalActions disabled={submitting} onClose={onClose} />
            </form>
        </ModalFrame>
    );
}

export function ModalActions({ disabled, onClose }: { disabled: boolean; onClose: () => void }) {
    return (
        <div className="modal-actions">
            <button type="button" className="button-secondary" onClick={onClose} disabled={disabled}>
                Cancel
            </button>
            <button type="submit" className="button-primary" disabled={disabled}>
                Save
            </button>
        </div>
    );
}

export function Field({ name, label, type = "text", required = false }: { name: string; label: string; type?: string; required?: boolean }) {
    return (
        <label className="field-group">
            <span className="field-label">{label}</span>
            <input className="field-input" name={name} type={type} required={required} />
        </label>
    );
}

export function Textarea({ name, label }: { name: string; label: string }) {
    return (
        <label className="field-group">
            <span className="field-label">{label}</span>
            <textarea className="field-input" name={name} rows={3} />
        </label>
    );
}

export function Select({
    name,
    label,
    children,
    required = false,
    defaultValue
}: {
    name: string;
    label: string;
    children: ReactNode;
    required?: boolean;
    defaultValue?: string | number;
}) {
    return (
        <label className="field-group">
            <span className="field-label">{label}</span>
            <select className="field-input" name={name} required={required} defaultValue={defaultValue}>
                {children}
            </select>
        </label>
    );
}

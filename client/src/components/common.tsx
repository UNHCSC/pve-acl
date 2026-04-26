import { Children, FormEvent, ReactNode, useEffect, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import type { ToastItem } from "../hooks/useToasts";
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

export function RowActionMenu({
    ariaLabel = "More actions",
    buttonClassName,
    children,
    className,
    closeOnSelect = true,
    menuClassName,
    menuWidth = 176,
    open,
    setOpen
}: {
    ariaLabel?: string;
    buttonClassName?: string;
    children: ReactNode;
    className?: string;
    closeOnSelect?: boolean;
    menuClassName?: string;
    menuWidth?: number;
    open?: boolean;
    setOpen?: (open: boolean) => void;
}) {
    const [internalOpen, setInternalOpen] = useState(false);
    const [position, setPosition] = useState<CSSProperties>({});
    const actualOpen = open ?? internalOpen;
    const updateOpen = setOpen ?? setInternalOpen;

    useEffect(() => {
        if (!actualOpen) {
            return;
        }
        const close = () => updateOpen(false);
        const closeOnEscape = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                updateOpen(false);
            }
        };
        window.addEventListener("click", close);
        window.addEventListener("keydown", closeOnEscape);
        return () => {
            window.removeEventListener("click", close);
            window.removeEventListener("keydown", closeOnEscape);
        };
    }, [actualOpen, updateOpen]);

    return (
        <div className={classNames("row-actions", className)}>
            <button
                type="button"
                className={classNames("row-menu-button", buttonClassName)}
                aria-label={ariaLabel}
                aria-haspopup="menu"
                aria-expanded={actualOpen}
                onClick={(event) => {
                    event.stopPropagation();
                    const itemCount = Children.toArray(children).length;
                    const height = Math.min(280, Math.max(44, itemCount * 41 + 2));
                    const rect = event.currentTarget.getBoundingClientRect();
                    const top = rect.bottom + 6 + height <= window.innerHeight - 8 ? rect.bottom + 6 : Math.max(8, rect.top - height - 6);
                    setPosition({
                        left: Math.max(8, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - 8)),
                        maxHeight: height,
                        top
                    });
                    updateOpen(!actualOpen);
                }}
            >
                <span className="row-menu-icon" aria-hidden="true" />
            </button>
            {actualOpen && createPortal(
                <div
                    role="menu"
                    className={classNames("row-menu", menuClassName)}
                    style={position}
                    onClick={(event) => {
                        event.stopPropagation();
                        if (closeOnSelect) {
                            updateOpen(false);
                        }
                    }}
                >
                    {children}
                </div>,
                document.body
            )}
        </div>
    );
}

export function ToastStack({ toasts }: { toasts: ToastItem[] }) {
    return (
        <div className="toast-stack">
            {toasts.map((toast) => (
                <div
                    key={toast.id}
                    className={classNames("toast-message", toast.leaving && "is-leaving", toast.kind === "warning" && "is-warning", toast.kind === "success" && "is-success")}
                    style={{ "--toast-duration": `${toast.duration}ms` } as CSSProperties & Record<string, string>}
                >
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
    const [error, setError] = useState("");
    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        setSubmitting(true);
        setError("");
        try {
            await onSubmit(new FormData(event.currentTarget));
        } catch (submitError) {
            setError(submitError instanceof Error ? submitError.message : "Action failed");
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <ModalFrame title={title} label={label} onClose={onClose}>
            <form className="modal-form" onSubmit={submit}>
                {children}
                {error && <p className="form-message is-warning">{error}</p>}
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

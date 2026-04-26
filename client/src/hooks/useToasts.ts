import { useState } from "react";
import type { ToastKind } from "../types";

export const TOAST_DURATION_MS = 4200;
const TOAST_EXIT_MS = 240;

export type ToastItem = {
    id: number;
    text: string;
    kind: ToastKind;
    leaving: boolean;
    duration: number;
};

export function useToasts() {
    const [toasts, setToasts] = useState<ToastItem[]>([]);

    const showToast = (text: string, kind: ToastKind = "info") => {
        const id = Date.now() + Math.round(Math.random() * 1000);
        setToasts((items) => [...items, { id, text, kind, leaving: false, duration: TOAST_DURATION_MS }]);
        window.setTimeout(() => {
            setToasts((items) => items.map((item) => (item.id === id ? { ...item, leaving: true } : item)));
            window.setTimeout(() => {
                setToasts((items) => items.filter((item) => item.id !== id));
            }, TOAST_EXIT_MS);
        }, TOAST_DURATION_MS);
    };

    return { toasts, showToast };
}

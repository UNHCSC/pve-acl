import { useState } from "react";
import type { ToastKind } from "../types";

export function useToasts() {
    const [toasts, setToasts] = useState<Array<{ id: number; text: string; kind: ToastKind }>>([]);

    const showToast = (text: string, kind: ToastKind = "info") => {
        const id = Date.now() + Math.round(Math.random() * 1000);
        setToasts((items) => [...items, { id, text, kind }]);
        window.setTimeout(() => {
            setToasts((items) => items.filter((item) => item.id !== id));
        }, 4200);
    };

    return { toasts, showToast };
}

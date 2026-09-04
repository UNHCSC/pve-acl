import { useEffect, useState } from "react";

export type EditorIndent = 2 | 4 | 8;

const storageKey = "organesson-editor-indent";
const changeEvent = "organesson-editor-settings-change";

export function readEditorIndent(): EditorIndent {
    if (typeof window === "undefined") return 4;
    const value = Number(window.localStorage.getItem(storageKey));
    return value === 2 || value === 8 ? value : 4;
}

export function storeEditorIndent(value: EditorIndent) {
    window.localStorage.setItem(storageKey, String(value));
    window.dispatchEvent(new CustomEvent(changeEvent, { detail: value }));
}

export function useEditorIndent() {
    const [indent, setIndent] = useState<EditorIndent>(readEditorIndent);

    useEffect(() => {
        const update = () => setIndent(readEditorIndent());
        window.addEventListener("storage", update);
        window.addEventListener(changeEvent, update);
        return () => {
            window.removeEventListener("storage", update);
            window.removeEventListener(changeEvent, update);
        };
    }, []);

    return [indent, (value: EditorIndent) => { storeEditorIndent(value); setIndent(value); }] as const;
}

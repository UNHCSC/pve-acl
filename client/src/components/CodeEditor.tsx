import CodeMirror from "@uiw/react-codemirror";
import { json } from "@codemirror/lang-json";
import { yaml } from "@codemirror/lang-yaml";
import { indentUnit, syntaxHighlighting, HighlightStyle } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { indentWithTab } from "@codemirror/commands";
import { tags } from "@lezer/highlight";
import { hcl } from "codemirror-lang-hcl";
import { useMemo } from "react";
import { useEditorIndent } from "../editor-settings";

export type CodeLanguage = "json" | "opentofu" | "ansible";

const highlightStyle = HighlightStyle.define([
    { tag: tags.keyword, color: "var(--color-code-keyword)" },
    { tag: [tags.string, tags.special(tags.string)], color: "var(--color-code-string)" },
    { tag: [tags.number, tags.bool, tags.null], color: "var(--color-code-value)" },
    { tag: [tags.propertyName, tags.attributeName], color: "var(--color-code-property)" },
    { tag: [tags.comment, tags.lineComment, tags.blockComment], color: "var(--color-code-comment)", fontStyle: "italic" },
    { tag: [tags.bracket, tags.punctuation], color: "var(--color-ink-muted)" },
    { tag: [tags.typeName, tags.className], color: "var(--color-code-type)" }
]);

const editorTheme = EditorView.theme({
    "&": { backgroundColor: "var(--color-paper)", color: "var(--color-ink)", fontSize: "13px" },
    ".cm-content": { caretColor: "var(--color-accent)", fontFamily: "IBM Plex Mono, monospace", lineHeight: "1.55", padding: "12px 0" },
    ".cm-gutters": { backgroundColor: "var(--color-surface)", borderRight: "1px solid var(--color-line)", color: "var(--color-ink-muted)" },
    ".cm-activeLine, .cm-activeLineGutter": { backgroundColor: "color-mix(in srgb, var(--color-accent) 7%, transparent)" },
    ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--color-accent)" },
    ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": { backgroundColor: "var(--color-selected-bg)" },
    "&.cm-focused": { outline: "none" }
});

function languageExtension(language: CodeLanguage) {
    if (language === "opentofu") return hcl();
    if (language === "ansible") return yaml();
    return json();
}

export function CodeEditor({ ariaLabel, language, value, onChange, readOnly = false }: { ariaLabel: string; language: CodeLanguage; value: string; onChange?: (value: string) => void; readOnly?: boolean }) {
    const [indent] = useEditorIndent();
    const extensions = useMemo(() => [
        languageExtension(language),
        indentUnit.of(" ".repeat(indent)),
        EditorState.tabSize.of(indent),
        keymap.of([indentWithTab]),
        EditorView.lineWrapping,
        syntaxHighlighting(highlightStyle)
    ], [indent, language]);

    return <CodeMirror aria-label={ariaLabel} value={value} extensions={extensions} theme={editorTheme} editable={!readOnly} basicSetup={{ bracketMatching: true, closeBrackets: true, foldGutter: true, highlightActiveLine: !readOnly, lineNumbers: true }} onChange={onChange} />;
}

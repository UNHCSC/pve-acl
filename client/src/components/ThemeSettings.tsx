import type { ThemeKey } from "../types";
import { themeOptions } from "../theme";
import { classNames } from "../ui-helpers";
import { type EditorIndent, useEditorIndent } from "../editor-settings";

export function ThemeSettings({
    open,
    setOpen,
    theme,
    setTheme
}: {
    open: boolean;
    setOpen: (open: boolean) => void;
    theme: ThemeKey;
    setTheme: (theme: ThemeKey) => void;
}) {
    const [editorIndent, setEditorIndent] = useEditorIndent();

    return (
        <div className="settings-menu" data-topbar-menu>
            <button className="settings-button" type="button" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)}>
                Settings
            </button>
            <div className="settings-dropdown" hidden={!open}>
                <span className="settings-label">Theme</span>
                <div className="theme-options" role="radiogroup" aria-label="Theme">
                    {themeOptions.map((option) => (
                        <button
                            key={option.key}
                            type="button"
                            className={classNames("theme-option", theme === option.key && "is-active")}
                            role="radio"
                            aria-checked={theme === option.key}
                            onClick={() => setTheme(option.key)}
                        >
                            <span className="theme-swatch" data-theme-swatch={option.key} aria-hidden="true" />
                            <span>{option.label}</span>
                        </button>
                    ))}
                </div>
                <span className="settings-label settings-section-label">Editor indentation</span>
                <div className="theme-options" role="radiogroup" aria-label="Editor indentation">
                    {([2, 4, 8] as EditorIndent[]).map((indent) => (
                        <button key={indent} type="button" className={classNames("theme-option", editorIndent === indent && "is-active")} role="radio" aria-checked={editorIndent === indent} onClick={() => setEditorIndent(indent)}>
                            <span className="indent-glyph" aria-hidden="true">→</span>
                            <span>{indent} spaces</span>
                        </button>
                    ))}
                </div>
            </div>
        </div>
    );
}

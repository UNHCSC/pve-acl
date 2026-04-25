import type { ThemeKey } from "../types";
import { themeOptions } from "../theme";
import { classNames } from "../ui-helpers";

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
            </div>
        </div>
    );
}

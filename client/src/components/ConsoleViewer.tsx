import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

export function ConsoleViewer({ path, password, targetWindow, onClose }: { path: string; password: string; targetWindow: Window; onClose: () => void }) {
    const screen = useRef<HTMLDivElement>(null);
    const [status, setStatus] = useState("Connecting…");
    useEffect(() => {
        targetWindow.document.title = "Organesson VM Console";
        targetWindow.document.body.className = "console-popup-body";
        if (!targetWindow.document.querySelector('link[data-organesson-console="styles"]')) {
            const stylesheet = targetWindow.document.createElement("link");
            stylesheet.rel = "stylesheet";
            stylesheet.href = "/static/build/site.css";
            stylesheet.dataset.organessonConsole = "styles";
            targetWindow.document.head.appendChild(stylesheet);
        }
        targetWindow.addEventListener("beforeunload", onClose);
        return () => targetWindow.removeEventListener("beforeunload", onClose);
    }, [onClose, targetWindow]);
    useEffect(() => {
        if (!screen.current) {
            return;
        }
        let disposed = false;
        let disconnect = () => {};
        void import("@novnc/novnc").then(({ default: RFB }) => {
            if (disposed || !screen.current) {
                return;
            }
            const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
            const rfb = new RFB(screen.current, `${scheme}//${window.location.host}${path}`, { credentials: { password } });
            rfb.scaleViewport = true;
            rfb.resizeSession = false;
            rfb.addEventListener("connect", () => setStatus("Connected"));
            rfb.addEventListener("disconnect", (event) => setStatus(event.detail.clean ? "Disconnected" : "Console connection failed"));
            rfb.addEventListener("securityfailure", (event) => setStatus(event.detail.reason || "Console authentication failed"));
            disconnect = () => rfb.disconnect();
        }).catch(() => setStatus("Failed to load the console client"));
        return () => {
            disposed = true;
            disconnect();
        };
    }, [password, path]);
    return createPortal(
        <div className="console-popup-root">
            <section className="console-panel" aria-label="Virtual machine console">
                <header>
                    <strong>Virtual machine console</strong>
                    <span>{status}</span>
                    <button className="button-secondary compact-button" type="button" onClick={onClose}>Close</button>
                </header>
                <div className="console-screen" ref={screen} />
            </section>
        </div>,
        targetWindow.document.body
    );
}

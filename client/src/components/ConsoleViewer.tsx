import { useEffect, useRef, useState } from "react";

export function ConsoleViewer({ path, password, onClose }: { path: string; password: string; onClose: () => void }) {
    const screen = useRef<HTMLDivElement>(null);
    const [status, setStatus] = useState("Connecting…");
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
    return (
        <div className="modal-backdrop">
            <section className="console-panel" role="dialog" aria-modal="true" aria-label="Virtual machine console">
                <header>
                    <strong>Virtual machine console</strong>
                    <span>{status}</span>
                    <button className="button-secondary compact-button" type="button" onClick={onClose}>Close</button>
                </header>
                <div className="console-screen" ref={screen} />
            </section>
        </div>
    );
}

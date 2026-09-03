import { useEffect, useRef } from "react";

export function ConsoleViewer({ path, onClose }: { path: string; onClose: () => void }) {
    const screen = useRef<HTMLDivElement>(null);
    useEffect(() => {
        if (!screen.current) return;
        let disposed = false;
        let disconnect = () => {};
        void import("@novnc/novnc").then(({ default: RFB }) => {
            if (disposed || !screen.current) return;
            const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
            const rfb = new RFB(screen.current, `${scheme}//${window.location.host}${path}`);
            rfb.scaleViewport = true;
            rfb.resizeSession = true;
            disconnect = () => rfb.disconnect();
        });
        return () => { disposed = true; disconnect(); };
    }, [path]);
    return <div className="modal-backdrop"><section className="console-panel" role="dialog" aria-modal="true" aria-label="Virtual machine console"><header><strong>Virtual machine console</strong><button className="button-secondary compact-button" type="button" onClick={onClose}>Close</button></header><div className="console-screen" ref={screen} /></section></div>;
}

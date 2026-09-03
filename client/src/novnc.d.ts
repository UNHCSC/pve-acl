declare module "@novnc/novnc" {
    export default class RFB {
        constructor(target: HTMLElement, url: string, options?: { credentials?: { password?: string } });
        scaleViewport: boolean;
        resizeSession: boolean;
        disconnect(): void;
        addEventListener(type: "connect", listener: () => void): void;
        addEventListener(type: "disconnect", listener: (event: CustomEvent<{ clean: boolean }>) => void): void;
        addEventListener(type: "securityfailure", listener: (event: CustomEvent<{ reason?: string }>) => void): void;
    }
}

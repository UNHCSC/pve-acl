type ApiErrorBody = { error?: string };

// requestKey creates an idempotency key without relying on secure-context-only randomUUID.
export function requestKey(): string {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
        return crypto.randomUUID();
    }
    if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
        const bytes = crypto.getRandomValues(new Uint8Array(16));
        return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
    }
    return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    if (init.body && !headers.has("Content-Type")) {
        headers.set("Content-Type", "application/json");
    }

    const response = await fetch(path, {
        credentials: "same-origin",
        ...init,
        headers
    });

    if (response.status === 401) {
        const redirect = encodeURIComponent(`${window.location.pathname}${window.location.search}`);
        window.location.href = `/login?redirect=${redirect}`;
        throw new Error("authentication required");
    }

    if (!response.ok) {
        let message = response.statusText || "Request failed";
        try {
            const body = (await response.json()) as ApiErrorBody;
            if (body.error) {
                message = body.error;
            }
        } catch {
            // Keep status text when the response is empty.
        }
        throw new Error(message);
    }

    if (response.status === 204) {
        return undefined as T;
    }

    const text = await response.text();
    if (!text) {
        return undefined as T;
    }

    const contentType = response.headers.get("Content-Type") || "";
    if (contentType.includes("application/json")) {
        return JSON.parse(text) as T;
    }

    return text as T;
}

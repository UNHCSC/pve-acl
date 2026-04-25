type ApiErrorBody = { error?: string };

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
    return (await response.json()) as T;
}

import type { components } from './wyo-api';

/**
 * The one place the wyo interface calls the wyo server. `fetch` is used in this folder only (an ESLint rule fails it anywhere
 * else), so every request the interface makes is here: it can be stubbed, counted and checked in one place, and no page can
 * call an address that is not the wyo server's own API (the browser holds no secret and calls nothing else).
 *
 * A call never throws. It answers with the data, or with the refusal the wyo server sent, or with a refusal of this file's
 * own when the call did not arrive (`network`) or the answer could not be read (`bad_answer`). The wyo server never writes a
 * sentence: a refusal is a code with parameters and the interface words it from `messages/` (`refusalKey`, `forms/refusal.ts`).
 */

/** The wyo server's refusal: `{ error: { code, params?, issues?: [{ path, code }] } }` (`api/wyo.yaml`, `Refusal`). */
export type TRefusal = components['schemas']['Refusal']['error'];

/** The program's name, version and mode (`GET /api/v1/meta`). */
export type TMeta = components['schemas']['Meta'];

export type TApiResult<T> = { ok: true; data: T } | { ok: false; status: number; refusal: TRefusal };

type TMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

function isRefusal(value: unknown): value is { error: TRefusal } {
    if (typeof value !== 'object' || value === null || !('error' in value)) return false;
    const error = (value as { error: unknown }).error;
    return typeof error === 'object' && error !== null && typeof (error as { code?: unknown }).code === 'string';
}

/** Sends one call to the wyo server and returns what came back; see the note above for what "came back" can be. */
export async function apiRequest<T>(method: TMethod, path: string, body?: unknown): Promise<TApiResult<T>> {
    const headers: Record<string, string> = { Accept: 'application/json' };
    const init: RequestInit = { method, headers };
    if (body !== undefined) {
        headers['Content-Type'] = 'application/json';
        init.body = JSON.stringify(body);
    }

    let response: Response;
    try {
        response = await fetch(path, init);
    } catch {
        return { ok: false, status: 0, refusal: { code: 'network' } };
    }

    const text = await response.text();
    let parsed: unknown;
    try {
        parsed = text === '' ? undefined : JSON.parse(text);
    } catch {
        return { ok: false, status: response.status, refusal: { code: 'bad_answer' } };
    }

    if (response.ok) return { ok: true, data: parsed as T };
    if (isRefusal(parsed)) return { ok: false, status: response.status, refusal: parsed.error };
    return { ok: false, status: response.status, refusal: { code: 'bad_answer' } };
}

/** The wyo server's calls. A page uses these, never `fetch`. */
export const api = {
    getMeta: () => apiRequest<TMeta>('GET', '/api/v1/meta'),
    get: <T>(path: string) => apiRequest<T>('GET', path),
    post: <T>(path: string, body: unknown) => apiRequest<T>('POST', path, body),
    put: <T>(path: string, body: unknown) => apiRequest<T>('PUT', path, body),
    patch: <T>(path: string, body: unknown) => apiRequest<T>('PATCH', path, body),
    delete: <T>(path: string) => apiRequest<T>('DELETE', path)
};

/** The values of a refusal's parameters that a message can be filled with (texts and numbers); the rest is dropped. */
export function refusalParams(refusal: TRefusal): Record<string, string | number> | undefined {
    if (!refusal.params) return undefined;
    const params: Record<string, string | number> = {};
    for (const [name, value] of Object.entries(refusal.params)) {
        if (typeof value === 'string' || typeof value === 'number') params[name] = value;
    }
    return params;
}

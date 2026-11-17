import type { TApiResult } from '$lib/api/api';

/** The one name the stub refuses, so a person can see a refusal of the "server" laid onto a box. */
export const REFUSED_NAME = 'отказ';

/**
 * The stand-in for the wyo server on the showcase page: a form is sent here instead of through `src/lib/api/`, so the page works
 * in a built interface with nothing behind it. It answers like the wyo server does: a success, or the refusal
 * `{ code, issues: [{ path, code }] }` naming the box.
 */
export async function stubSend(values: { name: string }): Promise<TApiResult<unknown>> {
    if (values.name.trim().toLowerCase() === REFUSED_NAME) {
        return {
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'name', code: 'invalid' }] }
        };
    }
    return { ok: true, data: {} };
}

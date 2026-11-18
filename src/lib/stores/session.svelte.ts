import { api } from '$lib/api/api';
import type { components } from '$lib/api/wyo-api';

/** The signed-in person as the wyo server answers (`GET /api/v1/session`). */
export type TSession = components['schemas']['Session'];

/**
 * Whether anybody is signed in: `unread` before the first answer, `in` with the person's login, `out` when the wyo server said
 * `signed_out`, `failed` when the call did not go through (the wyo server did not answer, or answered something else).
 */
export type TSessionStatus = 'unread' | 'in' | 'out' | 'failed';

/**
 * The one place the interface keeps who is signed in, so every page of a signed-in owner reads the same answer and the sign-in page
 * and the sign-out buttons tell it what they did. It holds the login and the idle minutes, never the session's secret, which the
 * browser keeps in a cookie no script can read.
 */
class SessionState {
    status = $state<TSessionStatus>('unread');
    current = $state<TSession | null>(null);
    /** The code of the refusal when `status` is `failed`. */
    refusal = $state<string | null>(null);

    async load(): Promise<void> {
        const result = await api.get<TSession>('/api/v1/session');
        if (result.ok) {
            this.signedIn(result.data);
        } else if (result.refusal.code === 'signed_out') {
            this.signedOut();
        } else {
            this.status = 'failed';
            this.current = null;
            this.refusal = result.refusal.code;
        }
    }

    signedIn(person: TSession): void {
        this.status = 'in';
        this.current = person;
        this.refusal = null;
    }

    signedOut(): void {
        this.status = 'out';
        this.current = null;
        this.refusal = null;
    }

    /** Back to before the first answer: for tests, and for a sign-out that wants the next page to ask again. */
    reset(): void {
        this.status = 'unread';
        this.current = null;
        this.refusal = null;
    }
}

export const session = new SessionState();

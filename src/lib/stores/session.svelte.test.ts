import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn() }
}));

import { api } from '$lib/api/api';
import { session } from './session.svelte';

const get = vi.mocked(api.get);

beforeEach(() => session.reset());
afterEach(() => get.mockReset());

describe('session', () => {
    it('starts unread, and reads the signed-in person once from the wyo server', async () => {
        expect(session.status).toBe('unread');
        get.mockResolvedValue({ ok: true, data: { login: 'owner', mustChangePassword: false, idleMinutes: 60 } });

        await session.load();

        expect(get).toHaveBeenCalledExactlyOnceWith('/api/v1/session');
        expect(session.status).toBe('in');
        expect(session.current).toEqual({ login: 'owner', mustChangePassword: false, idleMinutes: 60 });
    });

    it('is signed out when the wyo server says so, and failed — with the refusal — when the call did not go through', async () => {
        get.mockResolvedValue({ ok: false, status: 401, refusal: { code: 'signed_out' } });
        await session.load();
        expect([session.status, session.current, session.refusal]).toEqual(['out', null, null]);

        get.mockResolvedValue({ ok: false, status: 0, refusal: { code: 'network' } });
        await session.load();
        expect([session.status, session.current, session.refusal]).toEqual(['failed', null, 'network']);
    });

    it('takes the person from a sign-in and forgets them on a sign-out', () => {
        session.signedIn({ login: 'owner', mustChangePassword: true, idleMinutes: 30 });
        expect(session.status).toBe('in');
        expect(session.current?.mustChangePassword).toBe(true);

        session.signedOut();
        expect([session.status, session.current]).toEqual(['out', null]);
    });
});

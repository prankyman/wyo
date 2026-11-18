import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);

import { goto } from '$app/navigation';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import { session } from '$lib/stores/session.svelte';
import { sendToSignIn } from './sign-in-redirect';

afterEach(() => {
    resetPage();
    session.reset();
    vi.mocked(goto).mockClear();
});

describe('sendToSignIn', () => {
    it('forgets the person and opens the sign-in page', () => {
        session.signedIn({ login: 'owner', mustChangePassword: false, idleMinutes: 60 });
        setPageUrl('/settings');

        sendToSignIn();

        expect(session.status).toBe('out');
        expect(goto).toHaveBeenCalledExactlyOnceWith('/auth/login');
    });

    it('does not open the sign-in page again while the person is already on it', () => {
        setPageUrl('/auth/login');

        sendToSignIn();

        expect(goto).not.toHaveBeenCalled();
    });
});

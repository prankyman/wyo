import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn(), getMeta: vi.fn() }
}));

import { api } from '$lib/api/api';
import { session } from '$lib/stores/session.svelte';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import Layout from './+layout.svelte';

const get = vi.mocked(api.get);
const getMeta = vi.mocked(api.getMeta);
const children = createRawSnippet(() => ({ render: () => '<p data-test="inner">страница</p>' }));
const PERSON = { login: 'owner', mustChangePassword: false, idleMinutes: 60 };
const NOTICES = {
    notices: [{ id: 'backups:backup_none', source: 'backups', code: 'backup_none', tone: 'warn' }]
};

/** Answers each path of the wyo server's API with what the case gives it. */
function answer(person: Awaited<ReturnType<typeof api.get>>) {
    get.mockImplementation(async (path: string) => (path === '/api/v1/session' ? person : { ok: true, data: NOTICES }));
    getMeta.mockResolvedValue({ ok: true, data: { name: 'wyo', version: '1.2.3', mode: 'release' } });
}

beforeEach(() => {
    session.reset();
    setPageUrl('/settings');
});
afterEach(() => {
    resetPage();
    get.mockReset();
    getMeta.mockReset();
});

describe('the layout of the signed-in owner’s pages', () => {
    it('says it is opening the page while it reads who is signed in, and draws no menu', () => {
        get.mockReturnValue(new Promise(() => undefined));

        render(Layout, { props: { children } });

        expect(screen.getByTestId('app-loading')).toHaveTextContent('Открываем wyo…');
        expect(screen.queryByTestId('app-header')).toBeNull();
        expect(screen.queryByTestId('inner')).toBeNull();
    });

    it('draws the shell with the owner’s login, the version, the page inside and the notices behind the bell', async () => {
        answer({ ok: true, data: PERSON });

        render(Layout, { props: { children } });

        expect(await screen.findByTestId('inner')).toBeInTheDocument();
        expect(screen.getByTestId('app-user-login')).toHaveAccessibleName('owner');
        expect(screen.getByTestId('nav-settings')).toHaveAttribute('aria-current', 'page');
        await waitFor(() => expect(screen.getByTestId('nav-version')).toHaveTextContent('Версия wyo: 1.2.3'));
        await waitFor(() => expect(screen.getByTestId('notification-bell-number')).toHaveTextContent('1'));
        expect(get).toHaveBeenCalledWith('/api/v1/notices', { background: true });
    });

    it('draws no page for a person who is signed out', async () => {
        answer({ ok: false, status: 401, refusal: { code: 'signed_out' } });

        render(Layout, { props: { children } });

        await waitFor(() => expect(session.status).toBe('out'));
        expect(screen.queryByTestId('inner')).toBeNull();
        expect(screen.queryByTestId('app-header')).toBeNull();
    });

    it('says the wyo server did not answer, and reads again on a press of «Повторить»', async () => {
        answer({ ok: false, status: 0, refusal: { code: 'network' } });
        render(Layout, { props: { children } });

        expect(await screen.findByTestId('app-failed')).toHaveTextContent('Нет связи с программой wyo.');
        answer({ ok: true, data: PERSON });
        await fireEvent.click(screen.getByTestId('app-retry'));

        expect(await screen.findByTestId('inner')).toBeInTheDocument();
    });
});

import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { sendForm } from '$test/support/superform-answer';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);
vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { put: vi.fn(), delete: vi.fn() }
}));

import { goto } from '$app/navigation';
import { api } from '$lib/api/api';
import { session } from '$lib/stores/session.svelte';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import Page from './+page.svelte';

const put = vi.mocked(api.put);
const del = vi.mocked(api.delete);

async function open(id: string): Promise<void> {
    const section = screen.getByTestId(id) as HTMLDetailsElement;
    section.open = true;
    await fireEvent(section, new Event('toggle'));
}

/** Types the three boxes of the password form. */
async function type(current: string, next: string, confirm: string): Promise<void> {
    await fireEvent.input(screen.getByTestId('account-current'), { target: { value: current } });
    await fireEvent.input(screen.getByTestId('account-new'), { target: { value: next } });
    await fireEvent.input(screen.getByTestId('account-confirm'), { target: { value: confirm } });
}

beforeEach(() => {
    session.signedIn({ login: 'owner', mustChangePassword: false, idleMinutes: 60 });
    setPageUrl('/account');
});
afterEach(() => {
    resetPage();
    session.reset();
    put.mockReset();
    del.mockReset();
    vi.mocked(goto).mockClear();
});

describe('«Мой профиль»', () => {
    it('draws its heading with one «?», says who is signed in, and folds the password and the sign-out into two sections', () => {
        render(Page);

        expect(screen.getByTestId('account-heading')).toHaveTextContent('Мой профиль');
        expect(screen.getAllByTestId('account-help-button')).toHaveLength(1);
        expect(screen.getByTestId('account-login')).toHaveTextContent('Вы вошли как owner.');
        expect((screen.getByTestId('account-password') as HTMLDetailsElement).open).toBe(false);
        expect((screen.getByTestId('account-signout') as HTMLDetailsElement).open).toBe(false);
        expect(screen.queryByTestId('account-must-change')).toBeNull();
    });

    it('asks for a new password in a note when the password was set for the owner and not chosen by them', () => {
        session.signedIn({ login: 'owner', mustChangePassword: true, idleMinutes: 60 });

        render(Page);

        expect(screen.getByTestId('account-must-change')).toHaveTextContent('Смените его сейчас');
    });

    it('sends the current and the new password, says «Сохранено.» and empties the boxes', async () => {
        put.mockResolvedValue({ ok: true, data: undefined });
        render(Page);
        await open('account-password');
        await type('old-password-1', 'new-password-2', 'new-password-2');

        await sendForm(screen.getByTestId('account-password-form'));

        await waitFor(() => expect(screen.getByTestId('account-password-saved')).toHaveTextContent('Сохранено.'));
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/account/password', {
            currentPassword: 'old-password-1',
            newPassword: 'new-password-2'
        });
        expect(screen.getByTestId('account-current')).toHaveValue('');
        expect(screen.getByTestId('account-new')).toHaveValue('');
        expect(screen.getByTestId('account-confirm')).toHaveValue('');
    });

    it('stops a second typing that differs before anything is sent, and lays the wyo server’s refusal on the current-password box', async () => {
        put.mockResolvedValue({
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'currentPassword', code: 'wrong' }] }
        });
        render(Page);
        await open('account-password');
        await type('old-password-1', 'new-password-2', 'new-password-3');

        await sendForm(screen.getByTestId('account-password-form'));
        await waitFor(() =>
            expect(screen.getByTestId('account-confirm-error')).toHaveTextContent('Пароли не совпадают.')
        );
        expect(put).not.toHaveBeenCalled();

        await fireEvent.input(screen.getByTestId('account-confirm'), { target: { value: 'new-password-2' } });
        await sendForm(screen.getByTestId('account-password-form'));
        await waitFor(() => expect(screen.getByTestId('account-current-error')).toHaveTextContent('Неверно.'));
        expect(screen.getByTestId('account-new')).toHaveValue('new-password-2');
    });

    it('signs out of this browser: asks the wyo server, forgets the person and opens the sign-in page', async () => {
        del.mockResolvedValue({ ok: true, data: undefined });
        render(Page);
        await open('account-signout');

        await fireEvent.click(screen.getByTestId('account-sign-out'));

        await waitFor(() => expect(goto).toHaveBeenCalledExactlyOnceWith('/auth/login'));
        expect(del).toHaveBeenCalledExactlyOnceWith('/api/v1/session');
        expect(session.status).toBe('out');
    });

    it('asks, in a window that says what it does, before signing out everywhere, and words a refusal inside the window', async () => {
        del.mockResolvedValueOnce({ ok: false, status: 500, refusal: { code: 'internal' } });
        render(Page);
        await open('account-signout');

        await fireEvent.click(screen.getByTestId('account-sign-out-everywhere'));
        expect(screen.getByTestId('account-everywhere-window')).toHaveTextContent('завершит все сеансы');
        expect(del).not.toHaveBeenCalled();

        await fireEvent.click(screen.getByTestId('account-everywhere-window-confirm'));
        await waitFor(() => expect(screen.getByTestId('account-everywhere-error')).toBeInTheDocument());
        expect(screen.getByTestId('account-everywhere-window')).toBeInTheDocument();

        del.mockResolvedValueOnce({ ok: true, data: undefined });
        await fireEvent.click(screen.getByTestId('account-everywhere-window-confirm'));
        await waitFor(() => expect(goto).toHaveBeenCalledExactlyOnceWith('/auth/login'));
        expect(del).toHaveBeenLastCalledWith('/api/v1/sessions');
    });
});

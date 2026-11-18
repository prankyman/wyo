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
    api: { post: vi.fn() }
}));

import { goto } from '$app/navigation';
import { api } from '$lib/api/api';
import { session } from '$lib/stores/session.svelte';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import Page from './+page.svelte';

const post = vi.mocked(api.post);

async function type(login: string, password: string): Promise<void> {
    await fireEvent.input(screen.getByTestId('login-login'), { target: { value: login } });
    await fireEvent.input(screen.getByTestId('login-password'), { target: { value: password } });
}

beforeEach(() => setPageUrl('/auth/login'));
afterEach(() => {
    resetPage();
    session.reset();
    post.mockReset();
    vi.mocked(goto).mockClear();
});

describe('the sign-in page', () => {
    it('draws its heading with one «?» and a login and a password box, each with a «?», and never offers an account', () => {
        render(Page);

        expect(screen.getByTestId('login-heading')).toHaveTextContent('Вход в wyo');
        expect(screen.getAllByTestId('login-help-button')).toHaveLength(1);
        expect(screen.getByTestId('login-login')).toHaveAttribute('autocomplete', 'username');
        expect(screen.getByTestId('login-password')).toHaveAttribute('type', 'password');
        expect(screen.getByTestId('login-password')).toHaveAttribute('autocomplete', 'current-password');
        expect(screen.getByTestId('login-login-help-button')).toBeInTheDocument();
        expect(screen.getByTestId('login-password-help-button')).toBeInTheDocument();
        expect(screen.getByTestId('login-send')).toHaveTextContent('Войти');
        expect(screen.queryByText(/регистрац|создать учётную/i)).toBeNull();
    });

    it('sends the typed login and password, remembers who signed in and opens «Обзор»', async () => {
        post.mockResolvedValue({ ok: true, data: { login: 'owner', mustChangePassword: false, idleMinutes: 60 } });
        render(Page);
        await type('owner', 'secret-pass-1');

        await sendForm(screen.getByTestId('login-form'));

        await waitFor(() => expect(goto).toHaveBeenCalledExactlyOnceWith('/'));
        expect(post).toHaveBeenCalledExactlyOnceWith('/api/v1/session', { login: 'owner', password: 'secret-pass-1' });
        expect(session.current?.login).toBe('owner');
    });

    it('opens «Мой профиль» for an owner whose password has to be changed', async () => {
        post.mockResolvedValue({ ok: true, data: { login: 'owner', mustChangePassword: true, idleMinutes: 60 } });
        render(Page);
        await type('owner', 'secret-pass-1');

        await sendForm(screen.getByTestId('login-form'));

        await waitFor(() => expect(goto).toHaveBeenCalledExactlyOnceWith('/account'));
    });

    it('asks for both boxes in the browser and sends nothing while one is empty', async () => {
        render(Page);
        await type('owner', '');

        await sendForm(screen.getByTestId('login-form'));

        await waitFor(() =>
            expect(screen.getByTestId('login-password-error')).toHaveTextContent('Это поле нужно заполнить.')
        );
        expect(post).not.toHaveBeenCalled();
    });

    it.each([
        [
            'a wrong login or password, in one sentence for both',
            401,
            { code: 'sign_in_refused' },
            'Логин или пароль не подходят.'
        ],
        [
            'too many attempts, with the minutes left',
            429,
            { code: 'too_many_attempts', params: { minutes: 7 } },
            'Слишком много попыток. Подождите 7 мин. и повторите.'
        ]
    ])('words %s under the button, keeps the login and empties the password', async (_case, status, refusal, text) => {
        post.mockResolvedValue({ ok: false, status, refusal });
        render(Page);
        await type('owner', 'wrong-password');

        await sendForm(screen.getByTestId('login-form'));

        await waitFor(() => expect(screen.getByTestId('login-answer-error')).toHaveTextContent(text));
        expect(screen.getByTestId('login-login')).toHaveValue('owner');
        expect(screen.getByTestId('login-password')).toHaveValue('');
        expect(goto).not.toHaveBeenCalled();
    });

    it('tells a person who forgot the password that it is set again on the server, because wyo sends no e-mail', async () => {
        render(Page);
        expect(screen.queryByTestId('login-forgot-text')).toBeNull();

        await fireEvent.click(screen.getByTestId('login-forgot'));

        expect(screen.getByTestId('login-forgot-text')).toHaveTextContent('sudo wyo owner reset-password');
        expect(screen.getByTestId('login-forgot-text')).toHaveTextContent('не отправляет писем');
    });
});

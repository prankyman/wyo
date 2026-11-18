import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountOf, MARKETPLACE } from '$test/support/marketplaces';
import { sendForm } from '$test/support/superform-answer';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { post: vi.fn(), put: vi.fn() }
}));

import { api } from '$lib/api/api';
import KeyForm from './KeyForm.svelte';

const post = vi.mocked(api.post);
const put = vi.mocked(api.put);
const TOKEN = 'eyJhbGciOiJFUzI1NiJ9.secret-part-of-the-token.zz99';

afterEach(() => {
    post.mockReset();
    put.mockReset();
});

/** The key form of an adapter: its boxes are drawn from the adapter's `keyFields`, a secret one as a secret box. */
describe('KeyForm', () => {
    it('adds an account: the name and the adapter’s boxes, the secret one hidden as it is typed, sent with the adapter’s code', async () => {
        const added = accountOf({ keyLast4: 'zz99' });
        post.mockResolvedValue({ ok: true, data: added });
        const onSaved = vi.fn();
        render(KeyForm, { props: { lang: 'ru', marketplace: MARKETPLACE, id: 'account-add-form', onSaved } });

        expect(screen.getByTestId('account-add-form-steps')).toBeInTheDocument();
        const secret = screen.getByTestId('account-add-form-key-token');
        expect(secret).toHaveAttribute('type', 'password');
        expect(secret).toHaveAttribute('autocomplete', 'off');
        expect(screen.getByTestId('account-add-form-key-token-field')).toHaveTextContent('Токен');
        await fireEvent.input(screen.getByTestId('account-add-form-name'), { target: { value: 'Магазин одежды' } });
        await fireEvent.input(secret, { target: { value: TOKEN } });
        await sendForm(screen.getByTestId('account-add-form'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(added));
        expect(post).toHaveBeenCalledExactlyOnceWith('/api/v1/accounts', {
            marketplace: MARKETPLACE.code,
            name: 'Магазин одежды',
            key: { token: TOKEN }
        });
        expect(screen.getByTestId('account-add-form-answer-saved')).toHaveTextContent('Аккаунт добавлен');
        expect(screen.getByTestId('account-add-form-name')).toHaveValue('');
        expect(secret).toHaveValue('');
    });

    it('keeps everything typed when the wyo server refuses the key, and says why under the secret box in the accounts’ words', async () => {
        post.mockResolvedValue({
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'key.token', code: 'key_type_not_accepted' }] }
        });
        render(KeyForm, { props: { lang: 'ru', marketplace: MARKETPLACE, id: 'account-add-form', onSaved: vi.fn() } });
        await fireEvent.input(screen.getByTestId('account-add-form-name'), { target: { value: 'Магазин' } });
        await fireEvent.input(screen.getByTestId('account-add-form-key-token'), { target: { value: TOKEN } });

        await sendForm(screen.getByTestId('account-add-form'));

        await waitFor(() =>
            expect(screen.getByTestId('account-add-form-key-token-field')).toHaveTextContent('нужен Персональный ключ')
        );
        expect(screen.getByTestId('account-add-form-name')).toHaveValue('Магазин');
        expect(screen.getByTestId('account-add-form-key-token')).toHaveValue(TOKEN);
    });

    it('stops an empty form in the browser before anything is sent', async () => {
        render(KeyForm, { props: { lang: 'ru', marketplace: MARKETPLACE, id: 'account-add-form', onSaved: vi.fn() } });

        await sendForm(screen.getByTestId('account-add-form'));

        await waitFor(() =>
            expect(screen.getByTestId('account-add-form-key-token-field')).toHaveTextContent(
                'Это поле нужно заполнить.'
            )
        );
        expect(post).not.toHaveBeenCalled();
    });

    it('replaces the key of an account: names only the last four characters of the stored one, sends the key and empties its box', async () => {
        const account = accountOf();
        put.mockResolvedValue({ ok: true, data: { ...account, keyLast4: 'zz99' } });
        const onSaved = vi.fn();
        render(KeyForm, { props: { lang: 'ru', marketplace: MARKETPLACE, account, id: 'account-7-replace', onSaved } });

        expect(screen.getByTestId('account-7-replace-current')).toHaveTextContent('заканчивается на a1b2');
        expect(screen.queryByTestId('account-7-replace-name')).toBeNull();
        expect((screen.getByTestId('account-7-replace-steps') as HTMLDetailsElement).open).toBe(false);
        await fireEvent.input(screen.getByTestId('account-7-replace-key-token'), { target: { value: TOKEN } });
        await sendForm(screen.getByTestId('account-7-replace'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/accounts/7/key', { key: { token: TOKEN } });
        expect(screen.getByTestId('account-7-replace-key-token')).toHaveValue('');
    });
});

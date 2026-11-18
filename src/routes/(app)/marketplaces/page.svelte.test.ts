import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountOf, MARKETPLACE } from '$test/support/marketplaces';
import { sendForm } from '$test/support/superform-answer';
import type { TAccount, TMarketplace } from '$lib/components/marketplaces/types';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn(), post: vi.fn(), delete: vi.fn() }
}));

import { api } from '$lib/api/api';
import Page from './+page.svelte';

const get = vi.mocked(api.get);
const post = vi.mocked(api.post);
const del = vi.mocked(api.delete);

/** The wyo server's answers: the adapters and the accounts the page reads. */
function answers(accounts: TAccount[], marketplaces: TMarketplace[] = [MARKETPLACE]): void {
    get.mockImplementation(
        async (path: string) =>
            (path === '/api/v1/marketplaces'
                ? { ok: true, data: { marketplaces } }
                : { ok: true, data: { accounts } }) as never
    );
}

afterEach(() => {
    get.mockReset();
    post.mockReset();
    del.mockReset();
});

describe('«Маркетплейсы»', () => {
    it('draws its heading with one «?», reads the adapters and the accounts, a card for each, and keeps «Добавить аккаунт» folded', async () => {
        answers([accountOf(), accountOf({ id: 8, name: 'Второй магазин' })]);

        render(Page);

        expect(screen.getByTestId('marketplaces-heading')).toHaveTextContent('Маркетплейсы');
        expect(screen.getAllByTestId('marketplaces-help-button')).toHaveLength(1);
        expect(screen.getByTestId('marketplaces-loading')).toBeInTheDocument();
        await screen.findByTestId('marketplaces-loaded');
        expect(screen.getAllByTestId('account-card').map((card) => card.getAttribute('data-account-id'))).toEqual([
            '7',
            '8'
        ]);
        expect(screen.getByTestId('account-add-open')).toHaveTextContent('Добавить аккаунт');
        expect(screen.queryByTestId('marketplaces-empty')).toBeNull();
        expect(get).toHaveBeenCalledTimes(2);
    });

    it('teaches what to do when no account exists yet, with the adding form open on arrival', async () => {
        answers([]);

        render(Page);

        expect(await screen.findByTestId('marketplaces-empty')).toHaveTextContent('Нажмите «Добавить аккаунт»');
        expect(screen.getByTestId('account-add-form')).toBeInTheDocument();
        expect(screen.queryByTestId('marketplaces-choose')).toBeNull();
    });

    it('puts the card of an added account into the list', async () => {
        answers([]);
        post.mockResolvedValue({ ok: true, data: accountOf({ id: 9, name: 'Новый магазин' }) });
        render(Page);
        await screen.findByTestId('account-add-form');

        await fireEvent.input(screen.getByTestId('account-add-form-name'), { target: { value: 'Новый магазин' } });
        await fireEvent.input(screen.getByTestId('account-add-form-key-token'), { target: { value: 'token-text' } });
        await sendForm(screen.getByTestId('account-add-form'));

        await waitFor(() => expect(screen.getAllByTestId('account-card')).toHaveLength(1));
        expect(within(screen.getByTestId('account-card')).getByTestId('account-name')).toHaveTextContent(
            'Новый магазин'
        );
        expect(screen.queryByTestId('marketplaces-empty')).toBeNull();
    });

    it('takes the card of a deleted account out of the list', async () => {
        answers([accountOf({ hasOrders: false })]);
        del.mockResolvedValue({ ok: true, data: undefined });
        render(Page);
        await screen.findByTestId('account-card');

        await fireEvent.click(screen.getByTestId('account-delete'));
        await fireEvent.click(screen.getByTestId('account-delete-window-confirm'));

        await waitFor(() => expect(screen.queryByTestId('account-card')).toBeNull());
        expect(screen.getByTestId('marketplaces-empty')).toBeInTheDocument();
    });

    it('asks which marketplace the account is on when the wyo server holds more than one adapter', async () => {
        const second: TMarketplace = { ...MARKETPLACE, code: 'later-one', mark: 'unknown' };
        answers([], [MARKETPLACE, second]);
        render(Page);

        const choose = await screen.findByTestId('marketplaces-choose');

        expect(
            within(choose)
                .getAllByRole('option')
                .map((option) => option.getAttribute('value'))
        ).toEqual([MARKETPLACE.code, 'later-one']);
    });

    it('words a refusal and reads everything again on «Повторить»', async () => {
        get.mockResolvedValue({ ok: false, status: 500, refusal: { code: 'internal' } });
        render(Page);

        expect(await screen.findByTestId('marketplaces-refusal')).toHaveTextContent(
            'В программе wyo произошла ошибка.'
        );
        answers([]);
        await fireEvent.click(screen.getByTestId('marketplaces-retry'));

        expect(await screen.findByTestId('marketplaces-loaded')).toBeInTheDocument();
    });
});

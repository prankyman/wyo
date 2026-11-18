import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountOf, MARKETPLACE, MARKETPLACE_NAME } from '$test/support/marketplaces';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() }
}));

import { api } from '$lib/api/api';
import AccountCard from './AccountCard.svelte';

const post = vi.mocked(api.post);
const del = vi.mocked(api.delete);

afterEach(() => {
    post.mockReset();
    del.mockReset();
});

const props = (change: Partial<Parameters<typeof accountOf>[0]> = {}) => ({
    lang: 'ru' as const,
    account: accountOf(change),
    marketplace: MARKETPLACE,
    onChanged: vi.fn(),
    onDeleted: vi.fn()
});

/** One marketplace account on «Маркетплейсы»: what its key and its readings say, and what the owner can do with it. */
describe('AccountCard', () => {
    it('draws the marketplace, the name, the state and the facts, the key only by its last four characters', () => {
        render(AccountCard, {
            props: props({ lastRefusal: { code: 'key_refused', at: '2026-11-18T11:00:00', status: 401 } })
        });

        const card = screen.getByTestId('account-card');
        expect(card).toHaveAttribute('data-account-id', '7');
        expect(within(card).getByTestId('account-mark')).toHaveTextContent(MARKETPLACE_NAME);
        expect(within(card).getByTestId('account-name')).toHaveTextContent('Магазин одежды');
        expect(within(card).getByTestId('account-state')).toHaveTextContent('Работает');
        expect(within(card).getByTestId('account-facts-seller')).toHaveTextContent('ИП Иванов');
        expect(within(card).getByTestId('account-facts-key')).toHaveTextContent('заканчивается на a1b2');
        expect(within(card).getByTestId('account-facts-access')).toHaveTextContent('чтение и запись');
        expect(within(card).getByTestId('account-facts-expires')).toHaveTextContent('16 мая 2027');
        expect(within(card).getByTestId('account-facts-lastSuccess')).toHaveTextContent('18 нояб. 2026');
        expect(within(card).getByTestId('account-facts-lastRefusal')).toHaveTextContent(
            'Маркетплейс не принял этот ключ'
        );
        expect(within(card).getByTestId('account-facts-deadlineHours')).toHaveTextContent('48 ч после создания заказа');
        expect(within(card).getByTestId('account-replace-open')).toHaveTextContent('Заменить ключ');
        expect(within(card).getByTestId('account-change-open')).toHaveTextContent('Изменить');
        expect(within(card).queryByTestId('account-note-test')).toBeNull();
        expect(within(card).queryByTestId('account-delete')).toBeNull();
    });

    it('words the last refusal with the status the marketplace answered', () => {
        render(AccountCard, {
            props: props({ lastRefusal: { code: 'request_refused', at: '2026-11-18T11:00:00', status: 400 } })
        });

        const refusal = screen.getByTestId('account-facts-lastRefusal');
        expect(refusal).toHaveTextContent('Маркетплейс не принял один запрос wyo и ответил кодом 400.');
        expect(refusal).not.toHaveTextContent('{status}');
    });

    it('says that a test key reads only the sandbox, and what a missing category hides', () => {
        render(AccountCard, { props: props({ test: true, missingCategories: ['statistics', 'content'] }) });

        expect(screen.getByTestId('account-note-test')).toHaveTextContent('только к песочнице маркетплейса');
        expect(screen.getByTestId('account-note-missing-statistics')).toHaveTextContent('нет категории «Статистика»');
        expect(screen.getByTestId('account-note-missing-content')).toHaveTextContent('нет категории «content»');
    });

    it('says «ещё не было» before the first reading, and names no seller, end day or refusal it does not know', () => {
        render(AccountCard, {
            props: props({
                lastSuccessAt: undefined,
                sellerName: undefined,
                keyExpiresAt: undefined,
                lastRefusal: undefined
            })
        });

        expect(screen.getByTestId('account-facts-lastSuccess')).toHaveTextContent('ещё не было');
        expect(screen.queryByTestId('account-facts-seller')).toBeNull();
        expect(screen.queryByTestId('account-facts-expires')).toBeNull();
        expect(screen.queryByTestId('account-facts-lastRefusal')).toBeNull();
    });

    it('checks the key at once and hands the page the account as it now stands', async () => {
        const checked = accountOf({ state: 'key_refused' });
        post.mockResolvedValue({ ok: true, data: checked });
        const shown = props();
        render(AccountCard, { props: shown });

        await fireEvent.click(screen.getByTestId('account-check'));

        await waitFor(() => expect(shown.onChanged).toHaveBeenCalledExactlyOnceWith(checked));
        expect(post).toHaveBeenCalledExactlyOnceWith('/api/v1/accounts/7/check', {});
        expect(screen.getByTestId('account-answer-saved')).toHaveTextContent('Ключ проверен');
    });

    it.each([
        ['switches a working account off', 'working', 'account-disable', '/api/v1/accounts/7/disable'],
        ['switches an account off by the owner on again', 'disabled', 'account-enable', '/api/v1/accounts/7/enable']
    ] as const)('%s', async (_case, state, button, path) => {
        post.mockResolvedValue({ ok: true, data: accountOf() });
        const shown = props({ state });
        render(AccountCard, { props: shown });

        await fireEvent.click(screen.getByTestId(button));

        await waitFor(() => expect(shown.onChanged).toHaveBeenCalledOnce());
        expect(post).toHaveBeenCalledExactlyOnceWith(path, {});
    });

    it('words a refusal of a press under the buttons', async () => {
        post.mockResolvedValue({ ok: false, status: 502, refusal: { code: 'no_connection' } });
        render(AccountCard, { props: props() });

        await fireEvent.click(screen.getByTestId('account-check'));

        await waitFor(() =>
            expect(screen.getByTestId('account-answer-error')).toHaveTextContent('не смогла связаться с маркетплейсом')
        );
    });

    it('deletes an account no order came from, after the owner confirms it in a window', async () => {
        del.mockResolvedValue({ ok: true, data: undefined });
        const shown = props({ hasOrders: false });
        render(AccountCard, { props: shown });

        await fireEvent.click(screen.getByTestId('account-delete'));
        expect(screen.getByTestId('account-delete-window')).toHaveTextContent('Удалить аккаунт «Магазин одежды»?');
        expect(del).not.toHaveBeenCalled();
        await fireEvent.click(screen.getByTestId('account-delete-window-confirm'));

        await waitFor(() => expect(shown.onDeleted).toHaveBeenCalledExactlyOnceWith(7));
        expect(del).toHaveBeenCalledExactlyOnceWith('/api/v1/accounts/7');
    });
});

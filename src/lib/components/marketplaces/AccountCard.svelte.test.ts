import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
    accountOf,
    MARKETPLACE,
    MARKETPLACE_NAME,
    PACKING_MARKETPLACE,
    SECOND_MARKETPLACE,
    SECOND_MARKETPLACE_CODE,
    SECOND_MARKETPLACE_NAME,
    THIRD_MARKETPLACE,
    THIRD_MARKETPLACE_CODE,
    THIRD_MARKETPLACE_NAME
} from '$test/support/marketplaces';

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

/** An account of the second adapter: its key has a field that is not secret, and its orders come with their own deadline. */
const secondProps = (change: Partial<Parameters<typeof accountOf>[0]> = {}) => ({
    ...props({
        marketplace: SECOND_MARKETPLACE_CODE,
        keyFields: [{ name: 'client_id', value: '1234567' }],
        ...change
    }),
    marketplace: SECOND_MARKETPLACE
});

/** An account of the third adapter: a cabinet of several shops whose marketplace sends notifications. */
const thirdProps = (change: Partial<Parameters<typeof accountOf>[0]> = {}) => ({
    ...props({
        marketplace: THIRD_MARKETPLACE_CODE,
        keyLast4: 'q7r8',
        keyType: undefined,
        keyExpiresAt: undefined,
        ...change
    }),
    marketplace: THIRD_MARKETPLACE
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

    it('shows the fields of the key that are not secret in full with their names, beside the secret one’s last four characters', () => {
        render(AccountCard, { props: secondProps({ keyLast4: 'zz99' }) });

        expect(screen.getByTestId('account-mark')).toHaveTextContent(SECOND_MARKETPLACE_NAME);
        expect(screen.getByTestId('account-facts-key_client_id')).toHaveTextContent('1234567');
        expect(screen.getByTestId('account-facts-key_client_id-help-button')).toBeInTheDocument();
        expect(screen.getByText('Client ID')).toBeInTheDocument();
        expect(screen.getByTestId('account-facts-key')).toHaveTextContent('заканчивается на zz99');
    });

    it('claims no right of the key for a marketplace that is only read, whose key does not say whether it may write', () => {
        render(AccountCard, { props: secondProps() });

        expect(screen.queryByTestId('account-facts-access')).toBeNull();
    });

    it('says that the key of a marketplace that takes part in the exchange only reads, and names the level to add in the marketplace’s own words', () => {
        render(AccountCard, {
            props: {
                ...secondProps({ readOnly: true, missingCategories: ['posting_fbs_write'] }),
                marketplace: PACKING_MARKETPLACE
            }
        });

        expect(screen.getByTestId('account-facts-access')).toHaveTextContent('только чтение');
        expect(screen.getByTestId('account-note-missing-posting_fbs_write')).toHaveTextContent(
            'нет уровня доступа «Posting FBS»'
        );
        expect(screen.getByTestId('account-note-missing-posting_fbs_write')).toHaveTextContent('Заменить ключ');
    });

    it('draws no fact of a key field for an account whose key has none that is not secret', () => {
        render(AccountCard, { props: props() });

        expect(screen.queryByTestId('account-facts-key_client_id')).toBeNull();
    });

    it('says in the marketplace’s own words what a key without a level of access hides, and has no note of a test key', () => {
        render(AccountCard, {
            props: secondProps({ missingCategories: ['posting_fbo', 'product', 'seller_info'] })
        });

        expect(screen.getByTestId('account-note-missing-posting_fbo')).toHaveTextContent(
            'нет уровня доступа «Posting FBO»'
        );
        expect(screen.getByTestId('account-note-missing-product')).toHaveTextContent(
            'нет уровня доступа «Product read-only»'
        );
        expect(screen.getByTestId('account-note-missing-seller_info')).toHaveTextContent('сведениям о вашей компании');
        expect(screen.queryByTestId('account-note-test')).toBeNull();
    });

    it.each([
        [1, 'У аккаунта 1 склад, с которого'],
        [2, 'У аккаунта 2 склада, с которых'],
        [5, 'У аккаунта 5 складов, с которых']
    ] as const)(
        'says how many warehouses the seller delivers from itself, whose orders are not read: %i',
        (count, sentence) => {
            render(AccountCard, { props: secondProps({ sellerDeliversWarehouses: count }) });

            expect(screen.getByTestId('account-note-seller-delivers')).toHaveTextContent(sentence);
            expect(screen.getByTestId('account-note-seller-delivers')).toHaveTextContent(
                'адреса покупателей никогда не хранит'
            );
        }
    );

    it('says nothing of such warehouses when there are none', () => {
        render(AccountCard, { props: secondProps({ sellerDeliversWarehouses: undefined }) });

        expect(screen.queryByTestId('account-note-seller-delivers')).toBeNull();
    });

    it('shows the hours to hand an order over only for a marketplace that does not give each order its own deadline', () => {
        const { unmount } = render(AccountCard, { props: props() });
        expect(screen.getByTestId('account-facts-deadlineHours')).toHaveTextContent('48 ч после создания заказа');
        unmount();

        render(AccountCard, { props: secondProps() });
        expect(screen.queryByTestId('account-facts-deadlineHours')).toBeNull();
    });

    it('words the last refusal in the marketplace’s own words, and in the general ones for a code nobody described', () => {
        const { unmount } = render(AccountCard, {
            props: secondProps({ lastRefusal: { code: 'key_deactivated', at: '2026-11-18T11:00:00' } })
        });
        expect(screen.getByTestId('account-facts-lastRefusal')).toHaveTextContent('Api-key is deactivated');
        unmount();

        render(AccountCard, {
            props: secondProps({ lastRefusal: { code: 'never_heard_of_it', at: '2026-11-18T11:00:00' } })
        });
        expect(screen.getByTestId('account-facts-lastRefusal')).toHaveTextContent('у wyo нет описания этого отказа');
    });

    it('draws the list of shops under the facts when the account has shops, and none when it has none', () => {
        const shops = [
            { number: '21001', name: 'Одежда на Ленина', scheme: 'FBS', apiOff: false },
            { number: '21002', name: 'Склад курьеров', scheme: 'FBY', apiOff: true }
        ];
        const { unmount } = render(AccountCard, { props: thirdProps({ shops }) });

        expect(within(screen.getByTestId('account-card')).getByTestId('account-shops')).toBeInTheDocument();
        expect(screen.getByTestId('account-shops-api-off-21002')).toBeInTheDocument();
        unmount();

        render(AccountCard, { props: thirdProps() });
        expect(screen.queryByTestId('account-shops')).toBeNull();
    });

    it('draws the notification address under the facts when the account has one, and none when it has none', () => {
        const { unmount } = render(AccountCard, {
            props: thirdProps({ notifications: { path: '/hooks/marketplace/k3x9Q2mZp7', state: 'waiting' } })
        });

        expect(screen.getByTestId('account-notifications-address')).toHaveTextContent(
            `${window.location.origin}/hooks/marketplace/k3x9Q2mZp7`
        );
        expect(screen.getByTestId('account-notifications-state')).toHaveTextContent('Уведомления не подключены');
        unmount();

        render(AccountCard, { props: thirdProps() });
        expect(screen.queryByTestId('account-notifications')).toBeNull();
    });

    it.each([
        [3, '3'],
        [1, '1']
    ] as const)(
        'counts the warehouses a courier takes orders from within hours, and says nothing when there are none: %i',
        (count, value) => {
            const { unmount } = render(AccountCard, { props: thirdProps({ expressWarehouses: count }) });

            expect(screen.getByTestId('account-facts-expressWarehouses')).toHaveTextContent(value);
            expect(screen.getByText('Складов Экспресс')).toBeInTheDocument();
            expect(screen.getByTestId('account-facts-expressWarehouses-help-button')).toBeInTheDocument();
            unmount();

            render(AccountCard, { props: thirdProps({ expressWarehouses: undefined }) });
            expect(screen.queryByTestId('account-facts-expressWarehouses')).toBeNull();
        }
    );

    it('says in the third marketplace’s own words what a key without the products level or the changing level hides, and why the key was refused', () => {
        render(AccountCard, {
            props: thirdProps({
                name: 'Кабинет',
                missingCategories: ['products', 'orders_write'],
                readOnly: true,
                lastRefusal: { code: 'key_revoked', at: '2026-11-18T11:00:00', status: 401 }
            })
        });

        expect(screen.getByTestId('account-mark')).toHaveTextContent(THIRD_MARKETPLACE_NAME);
        expect(screen.getByTestId('account-note-missing-products')).toHaveTextContent('Просмотр товаров и карточек');
        expect(screen.getByTestId('account-note-missing-products')).toHaveTextContent('штрихкоды');
        expect(screen.getByTestId('account-note-missing-orders_write')).toHaveTextContent(
            'Обработка заказов и учёт товаров'
        );
        expect(screen.getByTestId('account-facts-lastRefusal')).toHaveTextContent('Api-Key token is revoked');
        expect(screen.queryByTestId('account-facts-access')).toBeNull();
        expect(screen.queryByTestId('account-facts-deadlineHours')).toBeNull();
    });
});

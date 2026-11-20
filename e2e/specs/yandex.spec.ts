import type { Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import { orderRowOf } from '../helpers/order-rows.js';
import { openScreen } from '../helpers/open-screen.js';
import { sweepScreen } from '../helpers/sweep.js';
import {
    cancelFakeYandexOrder,
    FAKE_YANDEX_CABINET,
    FAKE_YANDEX_SHOP_EXPRESS,
    FAKE_YANDEX_SHOP_FBS,
    FAKE_YANDEX_SHOP_FBY,
    makeFakeYandexOrder,
    makeYandexKey,
    setFakeYandexNotifications,
    setUpFakeYandex,
    type IFakeYandexSetup
} from '../support/fake-yandex.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * A Яндекс Маркет account on «Маркетплейсы» and its orders on «Заказы», against the fake Яндекс Маркет (Plan 7, section G): the
 * account added with a made-up key, its card with the key's last four characters, its shops and its notification address; the
 * fake told that address, so that its orders arrive by notification without «Обновить» — an FBS order of three products, an
 * Express one, an FBY one and a test order, marked and left out of the home page's numbers — and a cancellation with whom; each of
 * Яндекс Маркет's refusals of a key in its sentence and the note of a key without the products' level. What every state of a card
 * and an order says is proved by the components' unit tests. Each test runs on a wyo server of its own.
 */
test.use({ serverKind: 'own' });

const SHOP_FBS = FAKE_YANDEX_SHOP_FBS;
const SHOP_EXPRESS = FAKE_YANDEX_SHOP_EXPRESS;
const SHOP_FBY = FAKE_YANDEX_SHOP_FBY;

/** Sets a fresh key up at the fake: the shared made-up cabinet, with the changes a test makes. */
async function freshKey(change: IFakeYandexSetup = {}): Promise<string> {
    const key = makeYandexKey();
    await setUpFakeYandex(key, { ...FAKE_YANDEX_CABINET, ...change });
    return key;
}

/** Fills «Добавить аккаунт» for Яндекс Маркет and sends it. */
async function addThroughTheForm(page: Page, key: string): Promise<void> {
    await openScreen(page, '/marketplaces');
    await page.locator('#marketplaces-choose-box').selectOption('yandex');
    await page.getByTestId('account-add-form-name').fill('Кабинет на Маркете');
    await page.getByTestId('account-add-form-key-api_key').fill(key);
    await page.getByTestId('account-add-form-send').click();
}

test('a Яндекс Маркет account shows its key by its last four characters, its shops and its notification address, and its orders arrive by notification', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const key = await freshKey();

    await addThroughTheForm(page, key);
    await expect(page.getByTestId('account-add-form-answer-saved')).toHaveText(ruText('accounts.form.added'));

    const card = page.getByTestId('account-card');
    await expect(card.getByTestId('account-facts-key')).toHaveText(
        ruText('accounts.fact.key.value', { last4: key.slice(-4) })
    );
    await expect(card.getByTestId('account-facts-seller')).toHaveText('Тестовый кабинет');
    await expect(card.getByTestId(`account-shops-scheme-${SHOP_FBY}`)).toBeVisible();
    await expect(card.getByTestId('account-note-seller-delivers')).toBeVisible();
    await expect(card.getByTestId('account-notifications-state')).toHaveText(ruText('accounts.notifications.waiting'));
    await expect(card.getByTestId('account-notifications-copy')).toBeVisible();

    const address = (await card.getByTestId('account-notifications-address').textContent())?.trim() ?? '';
    expect(address.startsWith(`${server.origin}/hooks/marketplace/`), `the address ${address}`).toBe(true);
    expect(await page.content(), 'the page holds the key').not.toContain(key.slice(0, -4));

    // The fake sends its check to the address and then a notification for every order: they arrive without «Обновить».
    await setFakeYandexNotifications(key, address);
    await makeFakeYandexOrder(key, {
        order_id: 1000007001,
        campaign_id: SHOP_FBS,
        shipment_date: '2030-01-15',
        shipment_time: '12:00',
        items: [
            { offer_id: 'SHIRT', offer_name: 'Рубашка', count: 2 },
            { offer_id: 'SOCKS', offer_name: 'Носки', count: 1 },
            { offer_id: 'CAP', offer_name: 'Кепка', count: 3 }
        ]
    });
    await makeFakeYandexOrder(key, {
        order_id: 1000007002,
        campaign_id: SHOP_EXPRESS,
        program_type: 'EXPRESS',
        items: [{ offer_id: 'SHIRT', offer_name: 'Рубашка', count: 1 }]
    });
    await makeFakeYandexOrder(key, {
        order_id: 1000007003,
        campaign_id: SHOP_FBY,
        program_type: 'FBY',
        status: 'DELIVERY',
        substatus: 'DELIVERY_SERVICE_RECEIVED',
        items: [{ offer_id: 'SOCKS', offer_name: 'Носки', count: 1 }]
    });
    await makeFakeYandexOrder(key, {
        order_id: 1000007004,
        campaign_id: SHOP_FBS,
        fake: true,
        items: [{ offer_id: 'SHIRT', offer_name: 'Рубашка', count: 1 }]
    });

    await openScreen(page, '/marketplaces');
    await expect(card.getByTestId('account-notifications-state')).toHaveText(ruText('accounts.notifications.working'));

    await expect(async () => {
        await openScreen(page, '/orders');
        await expect(orderRowOf(page, '1000007004')).toHaveCount(1, { timeout: 1_000 });
    }, 'the notified orders arrive without «Обновить»').toPass({ timeout: 20_000 });

    await expect(orderRowOf(page, '1000007002').getByTestId('orders-list-scheme')).toContainText(
        ruText('orders.scheme.express')
    );
    await expect(orderRowOf(page, '1000007003').getByTestId('orders-list-scheme')).toContainText(
        ruText('orders.scheme.fby')
    );
    await expect(orderRowOf(page, '1000007004').getByTestId('orders-list-test')).toContainText(
        ruText('orders.test.label')
    );
    await expect(orderRowOf(page, '1000007001').getByTestId('orders-list-test')).toHaveCount(0);

    await orderRowOf(page, '1000007001').getByTestId('orders-list-link').click();
    const panel = page.getByTestId('order-details-panel');
    await expect(panel.getByTestId('order-lines-1-article')).toHaveText('SHIRT');
    await expect(panel.getByTestId('order-lines-1-barcodes')).toHaveText('4600000007001');
    await expect(panel.getByTestId('order-lines-3-article')).toHaveText('CAP');
    await page.getByTestId('order-details-close').click();

    await orderRowOf(page, '1000007004').getByTestId('orders-list-link').click();
    await expect(panel.getByTestId('order-test')).toContainText(ruText('orders.test.label'));
    await page.getByTestId('order-details-close').click();

    // The buyer cancels the FBS order; the notification brings the cancellation with whom.
    await cancelFakeYandexOrder(key, 1000007001, 'USER_CHANGED_MIND');
    await expect(async () => {
        await openScreen(page, '/orders');
        await expect(orderRowOf(page, '1000007001').getByTestId('orders-list-state')).toHaveText(
            ruText('orders.state.cancelled.buyer'),
            { timeout: 1_000 }
        );
    }).toPass({ timeout: 20_000 });

    // The home page counts three real orders and says one test order is left out of its numbers.
    await openScreen(page, '/');
    await expect(page.getByTestId('overview-card-orders-sentence')).toContainText(
        ruText('overview.orders.testSentence', { testOrders: 1 })
    );

    expect(requests.outside([baseURL!])).toEqual([]);
    await sweepScreen(page, screenOf('/orders'));
});

test('«Маркетплейсы» with a Яндекс Маркет card fits the phone and stands level at every width', async ({
    ownerPage: page
}) => {
    await addThroughTheForm(page, await freshKey());
    await expect(page.getByTestId('account-card')).toHaveCount(1);

    await sweepScreen(page, screenOf('/marketplaces'));
});

const REFUSALS: { name: string; setup: IFakeYandexSetup; text: string; params?: Record<string, string> }[] = [
    { name: 'an invalid key', setup: { key_state: 'invalid' }, text: 'marketplace.yandex.refusal.key_refused' },
    { name: 'a revoked key', setup: { key_state: 'revoked' }, text: 'marketplace.yandex.refusal.key_revoked' },
    {
        name: 'a key without the level of the orders',
        setup: { scopes: ['PRICING'] },
        text: 'marketplace.yandex.refusal.access_level_missing',
        params: { level: 'Просмотр информации о заказах' }
    }
];

for (const refusal of REFUSALS) {
    test(`${refusal.name} is refused with Яндекс Маркет's sentence on the key's box, and no account is added`, async ({
        ownerPage: page,
        expectedRefusals
    }) => {
        expectedRefusals.push({ status: 422, pathEnd: '/api/v1/accounts' });

        await addThroughTheForm(page, await freshKey(refusal.setup));

        await expect(page.getByTestId('account-add-form-key-api_key-error')).toHaveText(
            ruText(refusal.text, refusal.params)
        );
        await expect(page.getByTestId('account-card')).toHaveCount(0);
    });
}

test('a key without the level of the products is accepted, and its card says the barcodes are not shown', async ({
    ownerPage: page
}) => {
    await addThroughTheForm(page, await freshKey({ scopes: ['INVENTORY_AND_ORDER_PROCESSING'] }));

    await expect(page.getByTestId('account-note-missing-products')).toHaveText(
        ruText('marketplace.yandex.note.missing.products')
    );
});

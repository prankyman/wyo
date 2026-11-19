import type { Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import { addOzonAccountThroughApi, readOrdersNow } from '../helpers/accounts.js';
import { OZON_READING_METHODS } from '../helpers/external-api.js';
import { orderRowOf } from '../helpers/order-rows.js';
import { openScreen } from '../helpers/open-screen.js';
import { sweepScreen } from '../helpers/sweep.js';
import {
    hoursFromNow,
    makeFakeOzonPosting,
    makeOzonKey,
    setFakeOzonPostingStatus,
    setUpFakeOzon,
    splitFakeOzonPosting,
    type IFakeOzonKey,
    type IFakeOzonRole,
    type IFakeOzonSetup
} from '../support/fake-ozon.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * An Ozon account on «Маркетплейсы» and its orders on «Заказы», against the fake Ozon (plan 5, section G): the account added with
 * a made-up Client ID and API key, its card showing the Client ID in full and the key only by its last four characters (rule
 * R4), each of Ozon's refusals in its own sentence, the note of a key without the FBO level, an order of three products, a split
 * made in the cabinet and a cancellation with whom. What every state of a card, a form and an order's details says is proved by
 * the unit tests of the components; here a person uses the pages against the real wyo server. Each test runs on a wyo server of
 * its own.
 */
test.use({ serverKind: 'own' });

const ADMIN: IFakeOzonRole[] = [{ name: 'Admin', methods: [] }];

/** Sets a fresh key up at the fake Ozon: the seller, one FBS and one rFBS warehouse, three products, living 60 days. */
async function freshKey(change: IFakeOzonSetup = {}): Promise<IFakeOzonKey> {
    const key = makeOzonKey();
    await setUpFakeOzon(key.apiKey, {
        client_id: key.clientId,
        seller_name: 'ООО «Тестовый продавец»',
        expires_at: hoursFromNow(60 * 24),
        roles: ADMIN,
        warehouses: [
            { id: 1001, name: 'Склад FBS' },
            { id: 1002, name: 'Склад rFBS', is_rfbs: true }
        ],
        products: [
            { sku: 2001, offer_id: 'SHIRT-RED', name: 'Рубашка', barcodes: ['4600000002001'] },
            { sku: 2002, offer_id: 'SOCKS-BLUE', name: 'Носки', barcodes: ['4600000002002'] },
            { sku: 2003, offer_id: 'CAP', name: 'Кепка' }
        ],
        ...change
    });
    return key;
}

/** Fills «Добавить аккаунт» for Ozon and sends it. */
async function addThroughTheForm(page: Page, key: IFakeOzonKey): Promise<void> {
    await openScreen(page, '/marketplaces');
    await page.locator('#marketplaces-choose-box').selectOption('ozon');
    await page.getByTestId('account-add-form-name').fill('Магазин на Ozon');
    await page.getByTestId('account-add-form-key-client_id').fill(key.clientId);
    await page.getByTestId('account-add-form-key-api_key').fill(key.apiKey);
    await page.getByTestId('account-add-form-send').click();
}

test('an Ozon account added with a made-up Client ID and key shows the Client ID in full and the key only by its last four characters', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const key = await freshKey();

    await addThroughTheForm(page, key);

    await expect(page.getByTestId('account-add-form-answer-saved')).toHaveText(ruText('accounts.form.added'));
    await expect(page.getByTestId('account-add-form-key-api_key'), 'the secret box empties after the save').toHaveValue(
        ''
    );
    const card = page.getByTestId('account-card');
    await expect(card).toHaveCount(1);
    await expect(card.getByTestId('account-facts-key_client_id')).toHaveText(key.clientId);
    await expect(card.getByTestId('account-facts-key')).toHaveText(
        ruText('accounts.fact.key.value', { last4: key.apiKey.slice(-4) })
    );
    await expect(card.getByTestId('account-facts-seller')).toHaveText('ООО «Тестовый продавец»');
    await expect(card.getByTestId('account-note-seller-delivers')).toBeVisible();

    // Rule R4: neither the page nor the wyo server's answers hold more of the key than its last four characters.
    expect(await page.content(), 'the page holds the key').not.toContain(key.apiKey.slice(0, -4));
    const stored = await page.context().request.get(`${server.origin}/api/v1/accounts`);
    expect(await stored.text(), 'the list of accounts holds the key').not.toContain(key.apiKey.slice(0, -4));
    // Rule R3: only the wyo server talks to the marketplace.
    expect(requests.outside([baseURL!])).toEqual([]);
});

test('«Маркетплейсы» with an Ozon card fits the phone and stands level at every width', async ({
    ownerPage: page,
    server
}) => {
    await addOzonAccountThroughApi(page.context(), server, 'Магазин для замера', await freshKey());

    await sweepScreen(page, screenOf('/marketplaces'));
});

/** Each of Ozon's refusals of a key, the setup of the fake that provokes it, and its sentence. */
const REFUSALS: { name: string; setup: IFakeOzonSetup; text: string; params?: Record<string, string> }[] = [
    {
        name: 'a deactivated key',
        setup: { key_state: 'deactivated' },
        text: 'marketplace.ozon.refusal.key_deactivated'
    },
    {
        name: 'a key of other networks',
        setup: { key_state: 'ip_restricted' },
        text: 'marketplace.ozon.refusal.ip_restricted'
    },
    {
        name: 'a key before the offer',
        setup: { key_state: 'offer_not_signed' },
        text: 'marketplace.ozon.refusal.offer_not_signed'
    },
    {
        name: 'a key without the warehouses',
        setup: { roles: [{ name: 'Custom', methods: ['/v4/posting', '/v3/posting'] }] },
        text: 'marketplace.ozon.refusal.access_level_missing',
        params: { level: 'Warehouse' }
    }
];

for (const refusal of REFUSALS) {
    test(`${refusal.name} is refused with Ozon's sentence on the API key's box, and no account is added`, async ({
        ownerPage: page,
        expectedRefusals
    }) => {
        expectedRefusals.push({ status: 422, pathEnd: '/api/v1/accounts' });
        const key = await freshKey(refusal.setup);

        await addThroughTheForm(page, key);

        await expect(page.getByTestId('account-add-form-key-api_key-error')).toHaveText(
            ruText(refusal.text, refusal.params)
        );
        await expect(
            page.getByTestId('account-add-form-key-api_key'),
            'a refused save keeps what was typed'
        ).toHaveValue(key.apiKey);
        await expect(page.getByTestId('account-card')).toHaveCount(0);
    });
}

test('a key without the FBO level is accepted, and its card says what will not be shown', async ({
    ownerPage: page,
    server
}) => {
    const key = await freshKey({
        roles: [
            {
                name: 'Custom',
                methods: ['/v4/posting', '/v3/posting/fbs', '/v2/warehouse', '/v3/product', '/v1/seller']
            }
        ]
    });
    await addOzonAccountThroughApi(page.context(), server, 'Без FBO', key);

    await openScreen(page, '/marketplaces');

    await expect(page.getByTestId('account-note-missing-posting_fbo')).toHaveText(
        ruText('marketplace.ozon.note.missing.posting_fbo')
    );
});

test('a key that may only read is accepted, and its card says so and names the level to add', async ({
    ownerPage: page,
    server
}) => {
    const key = await freshKey({ roles: [{ name: 'Custom', methods: OZON_READING_METHODS }] });
    await addOzonAccountThroughApi(page.context(), server, 'Только чтение', key);

    await openScreen(page, '/marketplaces');

    await expect(page.getByTestId('account-facts-access')).toHaveText(ruText('accounts.fact.access.readOnly'));
    await expect(page.getByTestId('account-note-missing-posting_fbs_write')).toHaveText(
        ruText('marketplace.ozon.note.missing.posting_fbs_write')
    );
    await sweepScreen(page, screenOf('/marketplaces'));
});

test('an Ozon key that writes says so on its card and has no note of a level to add', async ({
    ownerPage: page,
    server
}) => {
    await addOzonAccountThroughApi(page.context(), server, 'С записью', await freshKey());

    await openScreen(page, '/marketplaces');

    await expect(page.getByTestId('account-facts-access')).toHaveText(ruText('accounts.fact.access.readWrite'));
    await expect(page.getByTestId('account-note-missing-posting_fbs_write')).toHaveCount(0);
});

test('an Ozon order of three products, its split made in the cabinet and a cancellation show on «Заказы»', async ({
    ownerPage: page,
    server
}) => {
    const key = await freshKey();
    await addOzonAccountThroughApi(page.context(), server, 'Магазин на Ozon', key);
    await makeFakeOzonPosting(key.apiKey, {
        posting_number: '31400001-0001-1',
        order_number: '31400001',
        warehouse_id: 1001,
        shipment_date: hoursFromNow(5),
        products: [
            { sku: 2001, offer_id: 'SHIRT-RED', name: 'Рубашка', quantity: 2 },
            { sku: 2002, offer_id: 'SOCKS-BLUE', name: 'Носки', quantity: 1 },
            { sku: 2003, offer_id: 'CAP', name: 'Кепка', quantity: 1 }
        ]
    });
    await makeFakeOzonPosting(key.apiKey, {
        posting_number: '31400002-0001-1',
        order_number: '31400002',
        warehouse_id: 1001,
        shipment_date: hoursFromNow(8),
        products: [{ sku: 2003, offer_id: 'CAP', name: 'Кепка', quantity: 1 }]
    });

    await openScreen(page, '/orders');
    await page.getByTestId('orders-refresh').click();
    const row = orderRowOf(page, '31400001-0001-1');
    await expect(row).toHaveCount(1, { timeout: 15_000 });
    // Ozon takes part in the exchange: no «пока не поддерживается»; with «Внешний API» empty the order is simply not passed on.
    await expect(row.getByTestId('orders-list-exchange-unsupported')).toHaveCount(0);
    await expect(row.getByTestId('orders-list-exchange-state')).toHaveText(ruText('exchange.state.not_sent'));

    await row.getByTestId('orders-list-link').click();
    const panel = page.getByTestId('order-details-panel');
    await expect(panel.getByTestId('order-lines-1-article')).toHaveText('SHIRT-RED');
    await expect(panel.getByTestId('order-lines-1-barcodes')).toHaveText('4600000002001');
    await expect(panel.getByTestId('order-lines-2-article')).toHaveText('SOCKS-BLUE');
    await expect(panel.getByTestId('order-lines-3-article')).toHaveText('CAP');
    await page.getByTestId('order-details-close').click();

    // The split made in the seller's cabinet and the buyer's cancellation of the other posting, read again at once.
    await splitFakeOzonPosting(key.apiKey, {
        posting_number: '31400001-0001-1',
        parts: [
            { posting_number: '31400001-0001-2', products: [{ sku: 2001, quantity: 2 }] },
            {
                posting_number: '31400001-0001-3',
                products: [
                    { sku: 2002, quantity: 1 },
                    { sku: 2003, quantity: 1 }
                ]
            }
        ]
    });
    await setFakeOzonPostingStatus(key.apiKey, {
        posting_number: '31400002-0001-1',
        status: 'cancelled',
        cancellation_type: 'customer'
    });
    await readOrdersNow(page.context(), server);
    await readOrdersNow(page.context(), server);
    await openScreen(page, '/orders');

    const cancelled = orderRowOf(page, '31400002-0001-1');
    await expect(cancelled.getByTestId('orders-list-state')).toHaveText(ruText('orders.state.cancelled.buyer'));

    const part = orderRowOf(page, '31400001-0001-3');
    await part.getByTestId('orders-list-link').click();
    await expect(panel.getByTestId('order-facts-parent')).toHaveText('31400001-0001-1');
    await page.getByTestId('order-details-close').click();

    // The original stays «Отменён», and its row names the parts it was split into.
    const original = orderRowOf(page, '31400001-0001-1');
    await expect(original.getByTestId('orders-list-state')).toHaveText(ruText('orders.state.cancelled'));
    await expect(original.getByTestId('orders-list-state-parts')).toHaveText(
        `${ruText('orders.field.parts')}: 31400001-0001-2, 31400001-0001-3`
    );
    await original.getByTestId('orders-list-link').click();
    await expect(panel.getByTestId('order-state')).toHaveText(ruText('orders.state.cancelled'));
    await expect(panel.getByTestId('order-facts-parts')).toHaveText('31400001-0001-2, 31400001-0001-3');
});

test('«Заказы» with an Ozon order of several products fits the phone and stands level at every width', async ({
    ownerPage: page,
    server
}) => {
    const key = await freshKey();
    await addOzonAccountThroughApi(page.context(), server, 'Магазин для замера', key);
    await makeFakeOzonPosting(key.apiKey, {
        posting_number: '31500001-0001-1',
        order_number: '31500001',
        warehouse_id: 1001,
        shipment_date: hoursFromNow(5),
        products: [
            { sku: 2001, offer_id: 'SHIRT-RED', name: 'Рубашка', quantity: 2 },
            { sku: 2002, offer_id: 'SOCKS-BLUE', name: 'Носки', quantity: 1 }
        ]
    });
    await readOrdersNow(page.context(), server);

    await sweepScreen(page, screenOf('/orders'));
});

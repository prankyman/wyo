import type { Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import { addAccountThroughApi, addOzonAccountThroughApi } from '../helpers/accounts.js';
import {
    connectExternalApiThroughApi,
    matchExternalApiThroughApi,
    OZON_READING_METHODS,
    ozonSellerSetUp,
    WILDBERRIES_HANDOVER
} from '../helpers/external-api.js';
import { openScreen } from '../helpers/open-screen.js';
import { openAllSections, openSection } from '../helpers/sections.js';
import { sweepScreen } from '../helpers/sweep.js';
import {
    FAKE_FC,
    FAKE_FULFILMENT_URL,
    flushFc,
    keyTail,
    makeFcKey,
    makeFcSecret,
    pingFc,
    setFcWebhook,
    setUpFc
} from '../support/fake-fulfilment.js';
import { makeOzonKey, setUpFakeOzon, type IFakeOzonKey, type IFakeOzonWarehouse } from '../support/fake-ozon.js';
import { makeToken } from '../support/fake-wildberries.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';
import type { IWyoServer } from '../support/wyo-server.js';

/**
 * «Внешний API» (`/external-api`): the address and the key of the fake fulfilment centre filled, checked and emptied, a key that
 * may only read refused with its sentence, the warehouses and the marketplace's code matched, the hand-over settings saved, and
 * notifications switched on and proved by a signed test notification. What every state and refusal says is proved by the unit
 * tests of the components; here a person uses the page against the real wyo server and the fake fulfilment centre. Every test
 * changes the setting, so each runs on a wyo server of its own, and on a key of its own at the fake.
 */
test.use({ serverKind: 'own' });

test('«Внешний API» not filled fits the phone and stands level at every width, with its section open too', async ({
    ownerPage: page
}) => {
    await sweepScreen(page, screenOf('/external-api'));
    await sweepScreen(page, screenOf('/external-api'), { before: openAllSections });
});

test('«Внешний API» filled, with its five sections open, fits the phone and stands level at every width', async ({
    ownerPage: page,
    server
}) => {
    const key = makeFcKey();
    const account = await addAccountThroughApi(page.context(), server, 'Магазин для замера', makeToken());
    await connectExternalApiThroughApi(page.context(), server, key);
    await matchExternalApiThroughApi(page.context(), server, account.id, { handoverSettings: true });

    await sweepScreen(page, screenOf('/external-api'));
    await sweepScreen(page, screenOf('/external-api'), { before: openAllSections });
});

test('the setting is filled and checked, names the key only by its last four characters, and is emptied behind a window', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const key = makeFcKey();
    await setUpFc(key, { seller_name: 'Склад Север' });
    await openScreen(page, '/external-api');
    await expect(page.getByTestId('external-api-empty-next')).toBeVisible();
    await expect(page.getByTestId('external-api-warehouses')).toHaveCount(0);

    await openSection(page, 'external-api-connection');
    await page.getByTestId('connection-address-box').fill(FAKE_FULFILMENT_URL);
    await page.getByTestId('connection-key-box').fill(key);
    await page.getByTestId('external-api-connection-save').click();

    await expect(page.getByTestId('external-api-connection-saved')).toHaveText(ruText('common.disclosure.saved'));
    await expect(page.getByTestId('external-api-empty-next')).toHaveCount(0);
    for (const id of ['warehouses', 'marketplaces', 'events', 'handover']) {
        await expect(page.getByTestId(`external-api-${id}`)).toBeVisible();
    }
    await openSection(page, 'external-api-connection');
    await expect(page.getByTestId('connection-key-box'), 'the key box empties after the save').toHaveValue('');
    await expect(page.getByTestId('connection-result-state')).toHaveText(ruText('externalApi.state.working'));
    await expect(page.getByTestId('connection-result-facts-seller')).toHaveText('Склад Север');
    await expect(page.getByTestId('connection-result-facts-key')).toHaveText(
        ruText('externalApi.result.key.value', { last4: keyTail(key) })
    );
    await expect(page.getByTestId('connection-result-facts-address')).toHaveText(FAKE_FULFILMENT_URL);
    await expect(page.getByTestId('connection-current')).toHaveText(
        ruText('externalApi.connection.current', { last4: keyTail(key) })
    );
    // Rule R4: neither the page nor the wyo server's answer holds more of the key than its last four characters.
    expect(await page.content(), 'the page holds the key').not.toContain(key.slice(0, -4));
    const stored = await page.context().request.get(`${server.origin}/api/v1/external-api`);
    expect(await stored.text(), "the setting's answer holds the key").not.toContain(key.slice(0, -4));

    await page.getByTestId('connection-check').click();

    await expect(page.getByTestId('connection-check-answer-saved')).toHaveText(
        ruText('externalApi.connection.checked')
    );

    await page.getByTestId('connection-disconnect').click();
    await expect(page.getByTestId('connection-disconnect-window')).toBeVisible();
    await page.getByTestId('connection-disconnect-window-confirm').click();

    await expect(page.getByTestId('external-api-empty-next')).toBeVisible();
    await expect(page.getByTestId('external-api-warehouses')).toHaveCount(0);
    const emptied = await page.context().request.get(`${server.origin}/api/v1/external-api`);
    expect(await emptied.json()).toMatchObject({ state: 'empty' });
    // Rule R3: the page asked only the wyo server; the fake was asked by the wyo server alone.
    expect(requests.outside([baseURL!])).toEqual([]);
});

test('a key that may only read is refused with its sentence, keeps what was typed and fills nothing', async ({
    ownerPage: page,
    server,
    expectedRefusals
}) => {
    expectedRefusals.push({ status: 422, pathEnd: '/api/v1/external-api' });
    const key = makeFcKey();
    await setUpFc(key, { access: 'read_only' });
    await openScreen(page, '/external-api');

    await openSection(page, 'external-api-connection');
    await page.getByTestId('connection-address-box').fill(FAKE_FULFILMENT_URL);
    await page.getByTestId('connection-key-box').fill(key);
    await page.getByTestId('external-api-connection-save').click();

    await expect(page.getByTestId('external-api-connection-error')).toHaveText(ruText('errors.fc_read_only_key'));
    await expect(page.getByTestId('connection-key-box'), 'a refused save keeps what was typed').toHaveValue(key);
    await expect(page.getByTestId('external-api-empty-next')).toBeVisible();
    const stored = await page.context().request.get(`${server.origin}/api/v1/external-api`);
    expect(await stored.json()).toMatchObject({ state: 'empty' });
});

test("the seller's warehouse is matched to the fulfilment centre's, and only then is it no longer unmatched", async ({
    ownerPage: page,
    server
}) => {
    await addAccountThroughApi(page.context(), server, 'Магазин одежды', makeToken());
    await connectExternalApiThroughApi(page.context(), server, makeFcKey());
    await openScreen(page, '/external-api');
    await openSection(page, 'external-api-warehouses');
    await expect(page.getByTestId('warehouses-0-unmatched')).toHaveText(ruText('externalApi.warehouses.unmatched'));

    await page.getByTestId('warehouses-0-box').selectOption(FAKE_FC.warehouse);

    await expect(page.getByTestId('warehouses-0-unmatched')).toHaveCount(0);
    await page.getByTestId('external-api-warehouses-save').click();
    await expect(page.getByTestId('external-api-warehouses-saved')).toHaveText(ruText('common.disclosure.saved'));
    const saved = await page.context().request.get(`${server.origin}/api/v1/external-api/warehouses`);
    expect(((await saved.json()) as { rows: { fcWarehouse?: string }[] }).rows[0].fcWarehouse).toBe(FAKE_FC.warehouse);
});

test("the marketplace's code is proposed when it equals the adapter's, and one press saves it", async ({
    ownerPage: page,
    server
}) => {
    await addAccountThroughApi(page.context(), server, 'Магазин обуви', makeToken());
    await connectExternalApiThroughApi(page.context(), server, makeFcKey());
    await openScreen(page, '/external-api');
    await openSection(page, 'external-api-marketplaces');
    await expect(page.getByTestId('codes-0-proposed')).toHaveText(
        ruText('externalApi.marketplaces.proposed', { name: 'Wildberries' })
    );
    await expect(page.getByTestId('codes-0-box')).toHaveValue(FAKE_FC.marketplace);

    await page.getByTestId('external-api-marketplaces-save').click();

    await expect(page.getByTestId('external-api-marketplaces-saved')).toHaveText(ruText('common.disclosure.saved'));
    const saved = await page.context().request.get(`${server.origin}/api/v1/external-api/marketplaces`);
    expect(((await saved.json()) as { rows: { fcMarketplace?: string }[] }).rows[0].fcMarketplace).toBe(
        FAKE_FC.marketplace
    );
});

test("the marketplace's hand-over settings are drawn from what the adapter declares and saved", async ({
    ownerPage: page,
    server
}) => {
    await addAccountThroughApi(page.context(), server, 'Магазин посуды', makeToken());
    await connectExternalApiThroughApi(page.context(), server, makeFcKey());
    await openScreen(page, '/external-api');
    await openSection(page, 'external-api-handover');
    await expect(page.getByTestId('handover-0-shipping_type-field')).toContainText(
        ruText('marketplace.wildberries.handover.shipping_type.label')
    );

    await page.getByTestId('handover-0-shipping_type-box').selectOption(WILDBERRIES_HANDOVER.shipping_type);
    await page.getByTestId('handover-0-city-box').fill(WILDBERRIES_HANDOVER.city);
    await page.getByTestId('handover-0-office_type-box').selectOption(WILDBERRIES_HANDOVER.office_type);
    await page.getByTestId('external-api-handover-save').click();

    await expect(page.getByTestId('external-api-handover-saved')).toHaveText(ruText('common.disclosure.saved'));
    const saved = await page.context().request.get(`${server.origin}/api/v1/external-api/marketplaces`);
    expect(((await saved.json()) as { rows: { handoverSettings: object }[] }).rows[0].handoverSettings).toEqual(
        WILDBERRIES_HANDOVER
    );
});

/** The warehouses of a made-up Ozon seller: one the seller assembles from, one it delivers from itself (rFBS), one assembled by Ozon. */
const OZON_WAREHOUSES: IFakeOzonWarehouse[] = [
    { id: 1001, name: 'Склад FBS' },
    { id: 1002, name: 'Склад rFBS', is_rfbs: true },
    { id: 1003, name: 'Склад с автосборкой', auto_assembly: true }
];

/** A made-up Ozon seller whose key only reads, with the three warehouses above, and its account added. */
async function addReadOnlyOzonAccount(page: Page, server: IWyoServer, name: string): Promise<IFakeOzonKey> {
    const key = makeOzonKey();
    await setUpFakeOzon(
        key.apiKey,
        ozonSellerSetUp(key, {
            warehouses: OZON_WAREHOUSES,
            roles: [{ name: 'Custom', methods: OZON_READING_METHODS }]
        })
    );
    await addOzonAccountThroughApi(page.context(), server, name, key);
    return key;
}

test('an Ozon warehouse is matched like any other, one the seller delivers from is never offered, and one assembled by Ozon warns in its row', async ({
    ownerPage: page,
    server
}) => {
    const key = makeOzonKey();
    await setUpFakeOzon(key.apiKey, ozonSellerSetUp(key, { warehouses: OZON_WAREHOUSES }));
    await addOzonAccountThroughApi(page.context(), server, 'Магазин на Ozon', key);
    await connectExternalApiThroughApi(page.context(), server, makeFcKey());
    await openScreen(page, '/external-api');
    await openSection(page, 'external-api-warehouses');

    const rows = page.getByTestId('warehouses-row');
    await expect(rows, 'the rFBS warehouse is not offered').toHaveCount(2);
    await expect(rows.filter({ hasText: 'Склад rFBS' })).toHaveCount(0);
    const plain = rows.filter({ hasText: 'Склад FBS' });
    const assembled = rows.filter({ hasText: 'Склад с автосборкой' });
    await expect(plain.locator('[data-test$="-unmatched"]')).toHaveText(ruText('externalApi.warehouses.unmatched'));
    await expect(plain.locator('[data-test$="-auto-assembly"]')).toHaveCount(0);
    await expect(assembled.locator('[data-test$="-auto-assembly"]')).toHaveText(
        ruText('externalApi.warehouses.autoAssembly')
    );

    await plain.locator('[data-test$="-box"]').selectOption(FAKE_FC.warehouse);

    await expect(plain.locator('[data-test$="-unmatched"]')).toHaveCount(0);
    await page.getByTestId('external-api-warehouses-save').click();
    await expect(page.getByTestId('external-api-warehouses-saved')).toHaveText(ruText('common.disclosure.saved'));
    const saved = await page.context().request.get(`${server.origin}/api/v1/external-api/warehouses`);
    const matches = ((await saved.json()) as { rows: { warehouseNumber: string; fcWarehouse?: string }[] }).rows;
    expect(matches.map((row) => [row.warehouseNumber, row.fcWarehouse])).toEqual([
        ['1001', FAKE_FC.warehouse],
        ['1003', undefined]
    ]);
});

test('the key of an Ozon account that may only read is warned about with the level to add, a key that writes is not, and Ozon declares no hand-over setting', async ({
    ownerPage: page,
    server
}) => {
    const writing = makeOzonKey();
    await setUpFakeOzon(writing.apiKey, ozonSellerSetUp(writing));
    await addOzonAccountThroughApi(page.context(), server, 'Ozon с записью', writing);
    await addReadOnlyOzonAccount(page, server, 'Ozon только чтение');
    await connectExternalApiThroughApi(page.context(), server, makeFcKey());
    await openScreen(page, '/external-api');
    await openSection(page, 'external-api-marketplaces');

    const rows = page.getByTestId('codes-row');
    await expect(rows).toHaveCount(2);
    await expect(
        rows.filter({ hasText: 'Ozon только чтение' }).locator('[data-test$="-read-only"]'),
        'the key that may only read names the level to add'
    ).toHaveText(ruText('marketplace.ozon.note.readOnly', { account: 'Ozon только чтение' }));
    await expect(rows.filter({ hasText: 'Ozon с записью' }).locator('[data-test$="-read-only"]')).toHaveCount(0);
    await expect(
        rows.locator('[data-test$="-unsupported"]'),
        'Ozon takes part in the exchange, so no row says «пока не поддерживается»'
    ).toHaveCount(0);

    await openSection(page, 'external-api-handover');
    await expect(page.getByTestId('handover-none'), 'Ozon declares no hand-over setting').toHaveText(
        ruText('externalApi.handover.none')
    );
});

test('«Внешний API» with Ozon accounts, a warehouse assembled by Ozon and a key that only reads fits the phone and stands level at every width', async ({
    ownerPage: page,
    server
}) => {
    await addReadOnlyOzonAccount(page, server, 'Ozon только чтение');
    await connectExternalApiThroughApi(page.context(), server, makeFcKey());

    await sweepScreen(page, screenOf('/external-api'), { before: openAllSections });
});

test('notifications switched on show the address to copy, and a signed test notification shows its time', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const key = makeFcKey();
    const secret = makeFcSecret();
    await connectExternalApiThroughApi(page.context(), server, key);
    await openScreen(page, '/external-api');
    await openSection(page, 'external-api-events');
    await expect(page.getByTestId('events-mode-box')).toHaveValue('poll');

    await page.getByTestId('events-mode-box').selectOption('notify');
    await expect(page.getByTestId('events-hook-pending')).toHaveText(ruText('externalApi.events.hookUrl.pending'));
    await page.getByTestId('events-secret-box').fill(secret);
    await page.getByTestId('external-api-events-save').click();

    await expect(page.getByTestId('external-api-events-saved')).toHaveText(ruText('common.disclosure.saved'));
    await openSection(page, 'external-api-events');
    await expect(page.getByTestId('events-secret-box'), 'the secret box empties after the save').toHaveValue('');
    await expect(page.getByTestId('events-secret-saved')).toHaveText(ruText('externalApi.events.secretSaved'));
    const hook = (await page.getByTestId('events-hook-url').textContent()) ?? '';
    expect(new URL(hook).pathname, 'the address to copy').toContain('/hooks/fulfilment/');
    await expect(page.getByTestId('events-facts-lastPing')).toHaveText(ruText('externalApi.events.lastPing.never'));
    expect(await page.content(), 'the page holds the secret').not.toContain(secret);

    // The fulfilment centre's system is given the address and the secret, and sends a signed test notification.
    await setFcWebhook(key, hook, secret);
    await pingFc(key);
    await flushFc(key);

    await expect
        .poll(
            async () => {
                const events = await page.context().request.get(`${server.origin}/api/v1/external-api/events`);
                return ((await events.json()) as { lastPingAt?: string }).lastPingAt;
            },
            { message: 'the wyo server never received the signed test notification' }
        )
        .toBeDefined();
    await openScreen(page, '/external-api');
    await openSection(page, 'external-api-events');
    await expect(page.getByTestId('events-facts-lastPing')).not.toHaveText(ruText('externalApi.events.lastPing.never'));
    expect(await page.content(), 'the page holds the secret').not.toContain(secret);
    // Rule R3: the page asked only the wyo server; the notification came from the fake to the wyo server.
    expect(requests.outside([baseURL!])).toEqual([]);
});

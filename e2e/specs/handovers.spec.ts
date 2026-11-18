import type { Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import { runExchangeUntil, runExchangeUntilHandover, setUpExchange } from '../helpers/external-api.js';
import { openScreen } from '../helpers/open-screen.js';
import { sweepScreen } from '../helpers/sweep.js';
import { fcHandover, fcHandoverStep, fcOrders, fcOrderStep } from '../support/fake-fulfilment.js';
import { makeFakeOrder } from '../support/fake-wildberries.js';
import { screenOf } from '../support/screens.js';
import type { IWyoServer } from '../support/wyo-server.js';
import { ruText } from '../support/texts.js';

/**
 * «Передачи» (`/handovers`): the fake fulfilment centre asks for the documents of a hand-over, the wyo server walks its steps with
 * the fake Wildberries, waits for the owner at the shipping point, and sends the documents. What each state, step and the picker
 * say is proved by the unit tests of `HandoverStateBadge`, `HandoverSteps`, `HandoverDetails` and `ShippingPointPicker`; here the
 * hand-over comes from the real wyo server and the fakes. Every test walks an order, so each runs on a wyo server of its own.
 */
test.use({ serverKind: 'own' });

/** The point the fake Wildberries offers for the hand-over settings of `WILDBERRIES_HANDOVER`. */
const POINT = '100';

/**
 * Walks one order to «packed» and has the fake fulfilment centre ask for its hand-over's documents; the wyo server then waits for
 * the shipping point. Returns what the tests read afterwards.
 */
async function walkToAWaitingHandover(page: Page, server: IWyoServer) {
    const { token, key } = await setUpExchange(page.context(), server, { handoverSettings: true });
    const number = String(await makeFakeOrder(token, { article: 'ART-HANDOVER', skus: ['4600000000901'] }));
    await runExchangeUntil(page.context(), server, number, 'sent');
    const [fcOrder] = await fcOrders(key);
    await fcOrderStep(key, fcOrder.id, 'reserve');
    await runExchangeUntil(page.context(), server, number, 'label_sent');
    await fcOrderStep(key, fcOrder.id, 'pack');
    await runExchangeUntil(page.context(), server, number, 'packed');
    const handoverAtFc = (await fcOrders(key))[0].handover_id;
    expect(handoverAtFc, 'the packed order stands in a hand-over at the fulfilment centre').not.toBeNull();

    await fcHandoverStep(key, handoverAtFc!, 'request-documents');
    const waiting = await runExchangeUntilHandover(page.context(), server, 'waiting');
    return { key, number, handoverAtFc: handoverAtFc!, waiting };
}

test('«Передачи» with no hand-over says so, fits the phone and stands level at every width', async ({
    ownerPage: page
}) => {
    await openScreen(page, '/handovers');
    await expect(page.getByTestId('handovers-empty')).toHaveText(ruText('handovers.empty'));

    await sweepScreen(page, screenOf('/handovers'));
});

test('«Передачи» with a hand-over that waits for the owner fits the phone and stands level at every width', async ({
    ownerPage: page,
    server
}) => {
    await walkToAWaitingHandover(page, server);

    await sweepScreen(page, screenOf('/handovers'));
});

test('a hand-over the fulfilment centre asked for waits for the shipping point, which the owner picks, and then the documents are sent', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const { key, number, handoverAtFc, waiting } = await walkToAWaitingHandover(page, server);

    await openScreen(page, '/handovers');
    const row = page.getByTestId('handovers-list-row');
    await expect(row).toHaveCount(1);
    await expect(row).toHaveAttribute('data-state', 'waiting');
    await expect(row.getByTestId('handovers-list-state')).toHaveText(ruText('handovers.state.waiting'));

    await row.getByTestId('handovers-list-link').click();

    const details = page.getByTestId('handovers-details-panel');
    await expect(details.getByTestId('handover-state')).toHaveText(ruText('handovers.state.waiting'));
    await expect(details.getByTestId('handover-steps-shipping')).toContainText(ruText('handovers.step.shipping'));
    await expect(details.getByTestId('handover-steps-now')).toHaveText(
        ruText('handovers.now.waiting', {
            number: 4,
            total: 6,
            step: ruText('handovers.step.shipping').toLowerCase()
        })
    );
    await expect(details.getByTestId('handover-facts-orders')).toHaveText(number);
    // The wyo server never guesses the point: the owner chooses it.
    const picker = details.getByTestId('shipping-point-box');
    await expect(picker).toHaveValue('');
    await picker.selectOption(POINT);
    await details.getByTestId('shipping-point-send').click();
    // The point is remembered: the picker has done its work and leaves the details.
    await expect(details.getByTestId('shipping-point-box')).toHaveCount(0);

    const sent = await runExchangeUntilHandover(page.context(), server, 'documents_sent');

    expect(sent.id).toBe(waiting.id);
    await openScreen(page, '/handovers');
    await expect(page.getByTestId('handovers-list-row')).toHaveAttribute('data-state', 'documents_sent');
    await page.getByTestId('handovers-list-link').click();
    const opened = page.getByTestId('handovers-details-panel');
    await expect(opened.getByTestId('handover-state')).toHaveText(ruText('handovers.state.documents_sent'));
    await expect(opened.getByTestId('handover-steps-now')).toHaveText(ruText('handovers.now.done'));
    await expect(opened.getByTestId('handover-closed')).toHaveText(ruText('handovers.closed'));
    await expect(opened.getByTestId('shipping-point-box')).toHaveCount(0);
    // The fulfilment centre's hand-over now holds the documents.
    const atFc = await fcHandover(key, handoverAtFc);
    expect(atFc.state).toBe('documents_ready');
    expect(atFc.documents).toHaveLength(1);
    expect(atFc.documents[0].kind).toBe('handover_code');
    // Rule R3: the page asked only the wyo server.
    expect(requests.outside([baseURL!])).toEqual([]);
});

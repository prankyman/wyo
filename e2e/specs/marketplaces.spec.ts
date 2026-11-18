import { expect, test } from '../fixtures/test.js';
import { addAccountThroughApi } from '../helpers/accounts.js';
import { openScreen } from '../helpers/open-screen.js';
import { sweepScreen } from '../helpers/sweep.js';
import {
    CATEGORY,
    daysFromNow,
    FAKE_SELLER,
    makeToken,
    setUpFake,
    TOKEN_TYPE,
    type ITokenClaims
} from '../support/fake-wildberries.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * «Маркетплейсы» (`/marketplaces`): an account added with a made-up token against the fake Wildberries, its card naming the key
 * only by its last four characters (rule R4), each refusal's sentence on the key's box, and a key that ends in two days behind the
 * bell. What every state and refusal of a card and a form says is proved by the unit tests of `AccountCard` and `KeyForm`; here a
 * person uses the page against the real wyo server. Every test adds or refuses accounts, so each runs on a wyo server of its own.
 */
test.use({ serverKind: 'own' });

test('«Маркетплейсы» with no account, its form open, fits the phone and stands level at every width', async ({
    ownerPage: page
}) => {
    await sweepScreen(page, screenOf('/marketplaces'));
});

test('«Маркетплейсы» with an account card fits the phone and stands level at every width', async ({
    ownerPage: page,
    server
}) => {
    await addAccountThroughApi(page.context(), server, 'Магазин для замера', makeToken());

    await sweepScreen(page, screenOf('/marketplaces'));
});

test('an account added with a made-up token shows its card, which names the key only by its last four characters', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const token = makeToken();
    const last4 = token.slice(-4);
    await openScreen(page, '/marketplaces');
    await expect(page.getByTestId('marketplaces-empty')).toHaveText(ruText('accounts.empty'));

    // The wyo server holds more than one marketplace, so the form asks first which one the account is on.
    await page.locator('#marketplaces-choose-box').selectOption('wildberries');
    await page.getByTestId('account-add-form-name').fill('Магазин одежды');
    await page.getByTestId('account-add-form-key-token').fill(token);
    await page.getByTestId('account-add-form-send').click();

    await expect(page.getByTestId('account-add-form-answer-saved')).toHaveText(ruText('accounts.form.added'));
    await expect(page.getByTestId('account-add-form-key-token'), 'the key box empties after the save').toHaveValue('');
    const card = page.getByTestId('account-card');
    await expect(card).toHaveCount(1);
    await expect(card).toHaveAttribute('data-state', 'working');
    await expect(card.getByTestId('account-name')).toHaveText('Магазин одежды');
    await expect(card.getByTestId('account-state')).toHaveText(ruText('accounts.state.working'));
    await expect(card.getByTestId('account-facts-seller')).toHaveText(FAKE_SELLER.name);
    await expect(card.getByTestId('account-facts-key')).toHaveText(ruText('accounts.fact.key.value', { last4 }));
    await expect(page.getByTestId('marketplaces-empty')).toHaveCount(0);

    // Rule R4: neither the page nor the wyo server's answers hold more of the key than its last four characters.
    expect(await page.content(), 'the page holds the key').not.toContain(token.slice(0, -4));
    const stored = await page.context().request.get(`${server.origin}/api/v1/accounts`);
    expect(await stored.text(), 'the list of accounts holds the key').not.toContain(token.slice(0, -4));
    // Rule R3: the page asked nothing of the fake or of anybody else; only the wyo server talks to the marketplace.
    expect(requests.outside([baseURL!])).toEqual([]);
});

/**
 * Each refusal of a key, the token that provokes it at the fake Wildberries, and the text that words it: Wildberries' own
 * sentence (`marketplace.wildberries.refusal.<code>`) where it has one, the shared one otherwise.
 */
const REFUSALS: { name: string; text: string; claims: ITokenClaims; refusedByFake?: boolean }[] = [
    { name: 'an expired token', text: 'accounts.refusal.key_expired', claims: { expiresAt: daysFromNow(-1) } },
    {
        name: 'a «Базовый» token',
        text: 'marketplace.wildberries.refusal.key_type_not_accepted',
        claims: { type: TOKEN_TYPE.base }
    },
    {
        name: 'a token without the category «Маркетплейс»',
        text: 'marketplace.wildberries.refusal.category_missing',
        claims: { categories: CATEGORY.statistics }
    },
    {
        name: 'a token Wildberries answers with 401',
        text: 'marketplace.wildberries.refusal.key_refused',
        claims: {},
        refusedByFake: true
    }
];

for (const refusal of REFUSALS) {
    test(`${refusal.name} is refused with its sentence on the key's box, and no account is added`, async ({
        ownerPage: page,
        expectedRefusals
    }) => {
        expectedRefusals.push({ status: 422, pathEnd: '/api/v1/accounts' });
        const token = makeToken(refusal.claims);
        if (refusal.refusedByFake) await setUpFake(token, { token_refused: 'invalid' });
        await openScreen(page, '/marketplaces');

        await page.locator('#marketplaces-choose-box').selectOption('wildberries');
        await page.getByTestId('account-add-form-name').fill('Магазин');
        await page.getByTestId('account-add-form-key-token').fill(token);
        await page.getByTestId('account-add-form-send').click();

        await expect(page.getByTestId('account-add-form-key-token-error')).toHaveText(ruText(refusal.text));
        await expect(page.getByTestId('account-add-form-key-token'), 'a refused save keeps what was typed').toHaveValue(
            token
        );
        await expect(page.getByTestId('account-card')).toHaveCount(0);
    });
}

test('a key that ends in two days shows behind the bell and leads to «Маркетплейсы»', async ({
    ownerPage: page,
    server
}) => {
    await addAccountThroughApi(page.context(), server, 'Магазин обуви', makeToken({ expiresAt: daysFromNow(2) }));
    await openScreen(page, '/');

    await page.getByTestId('notification-bell').click();
    const line = page.locator('[data-test="notification-line"][data-kind="account_key_expiring"]');
    await expect(line).toHaveCount(1);
    await expect(line.getByTestId('notification-line-text')).toHaveText(
        ruText('notices.account_key_expiring.few', { account: 'Магазин обуви', count: 2 })
    );
    await line.getByTestId('notification-line-link').click();

    await expect(page.getByTestId('marketplaces-loaded')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/marketplaces');
    await expect(page.getByTestId('account-card')).toHaveAttribute('data-state', 'key_expiring');
});

import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { expect, test } from '../fixtures/test.js';
import { openScreen } from '../helpers/open-screen.js';
import { openAllSections, openSection } from '../helpers/sections.js';
import { sweepScreen } from '../helpers/sweep.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * «Настройки» (`/settings`): the sweep, saving a setting, a value out of range, the copy of the database, and a request to the
 * updater. The registry's rules (kinds, bounds, defaults, what is stored) are proved by the Go tests; here a person uses the page.
 * The tests that change something run on a wyo server of their own.
 */
test('«Настройки» fits the phone and stands level at every width', async ({ ownerPage: page }) => {
    await sweepScreen(page, screenOf('/settings'));
});

test('«Настройки» with its sections open fits the phone and stands level at every width', async ({
    ownerPage: page
}) => {
    await sweepScreen(page, screenOf('/settings'), { before: openAllSections });
});

test.describe('on a wyo server of its own', () => {
    test.use({ serverKind: 'own' });

    test('the idle minutes are saved, kept by the wyo server and read again after a reload', async ({
        ownerPage: page
    }) => {
        await openScreen(page, '/settings');
        await openSection(page, 'settings-security');
        const minutes = page.getByTestId('setting-idleMinutes');
        await expect(minutes).toHaveValue('60');

        await minutes.fill('30');
        const [saved] = await Promise.all([
            page.waitForResponse(
                (response) =>
                    response.url().endsWith('/api/v1/settings/security') && response.request().method() === 'PUT'
            ),
            page.getByTestId('settings-security-save').click()
        ]);

        expect(saved.status()).toBe(200);
        await expect(page.getByTestId('settings-security-saved')).toHaveText(ruText('common.disclosure.saved'));
        const stored = (await (await page.context().request.get('/api/v1/settings')).json()) as {
            groups: { id: string; settings: { key: string; value: number }[] }[];
        };
        expect(stored.groups.find((group) => group.id === 'security')?.settings).toEqual([
            expect.objectContaining({ key: 'security.idleMinutes', value: 30 })
        ]);
        await page.reload();
        await expect(page.getByTestId('settings-sections')).toBeVisible();
        await openSection(page, 'settings-security');
        await expect(page.getByTestId('setting-idleMinutes')).toHaveValue('30');
    });

    test.describe('a value out of range', () => {
        for (const outside of ['4', '721']) {
            test(`${outside} minutes is refused on the field and nothing is sent`, async ({ ownerPage: page }) => {
                const sent: string[] = [];
                page.on('request', (candidate) => {
                    if (candidate.method() === 'PUT') sent.push(candidate.url());
                });
                await openScreen(page, '/settings');
                await openSection(page, 'settings-security');

                await page.getByTestId('setting-idleMinutes').fill(outside);
                await page.getByTestId('settings-security-save').click();

                await expect(page.getByTestId('setting-idleMinutes-error')).toHaveText(ruText('errors.out_of_range'));
                expect(sent).toEqual([]);
                const stored = (await (await page.context().request.get('/api/v1/settings')).json()) as {
                    groups: { id: string; settings: { value: number }[] }[];
                };
                expect(stored.groups.find((group) => group.id === 'security')?.settings[0]?.value).toBe(60);
            });
        }
    });

    test('«Сделать копию сейчас» adds a copy to the list and writes the file', async ({ ownerPage: page, server }) => {
        await openScreen(page, '/settings');
        await openSection(page, 'settings-backups');
        await expect(page.getByTestId('backups-empty')).toHaveText(ruText('settings.backups.empty'));

        await page.getByTestId('backups-create').click();

        await expect(page.getByTestId('backups-create-answer-saved')).toHaveText(ruText('settings.backups.created'));
        await expect(page.getByTestId('backup-row')).toHaveCount(1);
        await expect(page.getByTestId('backups-empty')).toHaveCount(0);
        const name = (await page.getByTestId('backup-row').textContent())?.split(' — ')[0]?.trim() ?? '';
        expect(name).toMatch(/^wyo-\d{4}-\d{2}-\d{2}\.db$/);
        expect(existsSync(path.join(server.dataDir, 'backups', name))).toBe(true);
    });

    test('«Проверить сейчас» asks the updater, which the wyo server writes down as a file', async ({
        ownerPage: page,
        server
    }) => {
        await openScreen(page, '/settings');
        await openSection(page, 'settings-updates');
        await expect(page.getByTestId('updates-requested')).toHaveCount(0);

        await page.getByTestId('updates-check').click();

        await expect(page.getByTestId('updates-requested')).toHaveText(ruText('settings.updates.requested'));
        await expect(page.getByTestId('updates-check')).toBeDisabled();
        await expect(page.getByTestId('updates-install')).toBeDisabled();
        expect(readFileSync(path.join(server.dataDir, 'update-request'), 'utf8').trim()).toBe('check');
    });
});

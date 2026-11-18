import { expect, test } from '../fixtures/test.js';
import { openScreen } from '../helpers/open-screen.js';
import { openSection } from '../helpers/sections.js';
import { writeUpdateState } from '../helpers/update-state.js';
import { PHONE } from '../helpers/phone.js';
import { ruText } from '../support/texts.js';

/**
 * The bell in the top bar. What each notice says and how the bell looks for each mix of lines is proved by the unit tests
 * (`notice-lines`, `panel-rules`, `NotificationBell`); what each source of notices reports is proved by the Go tests. Here a notice
 * goes the whole way: the updater writes its file (`update-state.json`, written only by the updater), the wyo server reads it, and
 * the bell shows it — in the popover at a computer's width and in the full-screen sheet at the phone's.
 */
test.use({ serverKind: 'own' });

test('a version the updater found appears behind the bell, at a computer and at a phone, and leads to «Настройки»', async ({
    ownerPage: page,
    server
}) => {
    // A fresh installation has no copy of the database yet, which is a notice of its own («backup_none»); one copy makes the bell empty.
    const copy = await page.context().request.post(`${server.origin}/api/v1/backups`, { data: {} });
    expect(copy.status()).toBe(201);
    await openScreen(page, '/');
    const bell = page.getByTestId('notification-bell');
    await expect(bell).toHaveAttribute('aria-label', ruText('shell.notifications.bellLabelEmpty'));
    await expect(page.getByTestId('notification-bell-number')).toHaveCount(0);
    await bell.click();
    await expect(page.getByTestId('notification-panel-empty')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('notification-panel-empty')).toHaveCount(0);

    writeUpdateState(server, { waitingVersion: '9.9.9' });
    await page.reload();
    await expect(page.getByTestId('overview-cards')).toBeVisible();

    await expect(bell).toHaveAttribute('aria-label', ruText('shell.notifications.bellLabel', { count: 1, urgent: 0 }));
    await expect(page.getByTestId('notification-bell-number')).toHaveText('1');
    await bell.click();
    const line = page.getByTestId('notification-line');
    await expect(line).toHaveCount(1);
    await expect(line).toHaveAttribute('data-kind', 'update_waiting');
    await expect(line.getByTestId('notification-line-text')).toContainText('9.9.9');
    await page.keyboard.press('Escape');
    await expect(line).toHaveCount(0);

    // The same line in the phone's sheet: whole width, at least 44 px tall.
    await page.setViewportSize(PHONE);
    await bell.click();
    await expect(line).toHaveCount(1);
    const box = await page.getByTestId('notification-line-link').boundingBox();
    expect(box?.height, 'the line is a tap target of at least 44 px').toBeGreaterThanOrEqual(44);
    expect(box && box.x >= 0 && box.x + box.width <= PHONE.width, "the line is inside the phone's screen").toBe(true);

    await page.getByTestId('notification-line-link').click();

    await expect(page.getByTestId('settings-sections')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/settings');
    await expect(page.getByTestId('notification-line')).toHaveCount(0);
    await openSection(page, 'settings-updates');
    await expect(page.getByTestId('updates-waiting')).toHaveText(
        ruText('settings.updates.waiting', { version: '9.9.9' })
    );
});

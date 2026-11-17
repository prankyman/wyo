import { expect, type Page } from '@playwright/test';
import { assertNoStaleExceptions } from '../support/phone-exceptions.js';
import type { IScreen } from '../support/screens.js';
import { LEVEL_WIDTHS, measureLevelAtWidths } from './box-level.js';
import { collectPhoneFailures, formatPhoneFailures, PHONE, sidewaysOverflow, type IMeasureStats } from './phone.js';
import { stackGapProblemsAtWidths } from './stack-gaps.js';

/**
 * The screen sweep of one page: the machinery behind the constitution's «Every screen is mobile-first». A spec calls
 * `sweepScreen(page, screenOf('<route>'))` for the page it owns (the row says which spec that is, `e2e/support/screens.ts`).
 *
 * The page is opened ONCE, at the phone's size (390 x 844) — the size is set before the page is opened, as Playwright advises
 * (https://playwright.dev/docs/api/class-page#page-set-viewport-size) — and waited for until its `ready` element is drawn. Then:
 *
 * - at 390 px, the phone's rules (`e2e/helpers/phone.ts`): no sideways scroll, every tap target at least 44 x 44 px and on the
 *   screen, nothing covers a tap target, no two tap targets overlap, no two fixed boxes overlap, a select shows its text before
 *   its arrow; and the one 16 px gap between the blocks of every column (`e2e/helpers/stack-gaps.ts`);
 * - at 640, 768, 1024 and 1280 px, the SAME page widened: no sideways scroll, the level rules of the boxes of its forms
 *   (`e2e/helpers/box-level.ts`), and the 16 px gap again.
 *
 * Every finding of the page is listed at once, one per line, in one soft assertion, so one run shows everything that is wrong:
 * `phone | route | target | rule | WxH at x,y | detail`, `level | width | rule | detail`, `gap | width | column | block → next
 * block | distance`, `sideways | width | N px wider than the screen`. A finding is never silenced here: the only list of exceptions
 * is `e2e/support/phone-exceptions.ts`, and it needs a reason for every entry.
 */

/** The widths the page is widened to after the phone's size. */
export const WIDER_WIDTHS: readonly number[] = LEVEL_WIDTHS;

/** One line per width at which the page is wider than its screen. */
async function sidewaysFindings(page: Page, widths: readonly number[]): Promise<string[]> {
    const before = page.viewportSize() ?? PHONE;
    const findings: string[] = [];
    try {
        for (const width of widths) {
            await page.setViewportSize({ width, height: before.height });
            const overflow = await sidewaysOverflow(page);
            if (overflow > 0)
                findings.push(`sideways | ${width} px | the page is ${overflow} px wider than the screen`);
        }
    } finally {
        await page.setViewportSize(before);
    }
    return findings;
}

/** Opens `row` and measures it as described above. The test fails (softly, with every finding) when anything is wrong. */
export async function sweepScreen(page: Page, row: IScreen): Promise<void> {
    const { route } = row;
    await page.setViewportSize(PHONE);
    const response = await page.goto(route);
    expect(response?.status(), `${route} was not opened`).toBeLessThan(400);
    await expect(page.getByTestId(row.ready), `${route} never drew its ready element ${row.ready}`).toBeVisible();

    const stats = {} as IMeasureStats;
    const findings: string[] = [];
    const failures = await collectPhoneFailures(page, route, { stats });
    findings.push(...formatPhoneFailures(failures).map((line) => `phone | ${line}`));
    findings.push(...(await stackGapProblemsAtWidths(page, [PHONE.width])).map((line) => `gap | ${line}`));

    findings.push(...(await sidewaysFindings(page, WIDER_WIDTHS)));
    const level = await measureLevelAtWidths(page, WIDER_WIDTHS);
    findings.push(...level.problems.map((line) => `level | ${line}`));
    findings.push(...(await stackGapProblemsAtWidths(page, WIDER_WIDTHS)).map((line) => `gap | ${line}`));

    expect
        .soft(
            findings.length,
            findings.length === 0
                ? `${route} fits the phone and stands level`
                : `${route} has ${findings.length} layout finding${findings.length === 1 ? '' : 's'}:\n${findings.join('\n')}`
        )
        .toBe(0);
    expect.soft(stats.targets, `${route} | the measurement found no tap target on the page`).toBeGreaterThan(0);
    try {
        assertNoStaleExceptions(route);
    } catch (error) {
        expect.soft(error instanceof Error ? error.message : String(error), `${route} | stale exception`).toBe('');
    }
}

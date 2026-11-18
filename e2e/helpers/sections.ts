import { expect, type Page } from '@playwright/test';

/** Where a heading row is pressed: over its arrow, away from the «?» that sits beside the title and does not fold the section. */
const HEADING_SPOT = { x: 20, y: 20 };

/**
 * Opens every folded section of the page (`CollapsibleSection` is a native `<details>` with the mark `data-collapsible`) and waits
 * until each one is open. For a sweep or a test that must see the boxes inside; one helper, used by every spec of a page with
 * folded sections.
 */
export async function openAllSections(page: Page): Promise<void> {
    const closed = page.locator('details[data-collapsible]:not([open]) > summary');
    for (let remaining = await closed.count(); remaining > 0; remaining = await closed.count()) {
        await closed.first().click({ position: HEADING_SPOT });
        await expect(closed).toHaveCount(remaining - 1);
    }
}

/** Opens the folded section `id` (its `data-test`) and waits until it is open. */
export async function openSection(page: Page, id: string): Promise<void> {
    const section = page.getByTestId(id);
    if ((await section.getAttribute('open')) === null)
        await page.getByTestId(`${id}-heading`).click({ position: HEADING_SPOT });
    await expect(section).toHaveAttribute('open', '');
}

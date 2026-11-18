import type { Locator, Page } from '@playwright/test';

/** The row of the order whose marketplace number is `number` in the list of «Заказы». */
export function orderRowOf(page: Page, number: string): Locator {
    return page
        .getByTestId('orders-list-row')
        .filter({ has: page.getByTestId('orders-list-link').getByText(number, { exact: true }) });
}

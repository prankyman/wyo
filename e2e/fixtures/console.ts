import type { Page, TestInfo } from '@playwright/test';

export interface IConsoleError {
    type: string;
    text: string;
}

export function attachConsoleCollector(page: Page, errors: IConsoleError[]) {
    page.on('console', (msg) => {
        if (msg.type() === 'error') {
            errors.push({ type: msg.type(), text: msg.text() });
        }
    });
    page.on('pageerror', (error) => {
        errors.push({ type: 'pageerror', text: String(error) });
    });
}

export function reportConsoleErrors(errors: IConsoleError[], testInfo: TestInfo) {
    if (errors.length > 0) {
        testInfo.attach('console-errors', {
            body: Buffer.from(errors.map((e) => `[${e.type}] ${e.text}`).join('\n')),
            contentType: 'text/plain'
        });
    }
}

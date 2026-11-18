import type { Page, TestInfo } from '@playwright/test';

export interface IConsoleError {
    type: string;
    text: string;
}

/**
 * A refusal of the wyo server a test provokes on purpose (a call made without a session answers 401): the browser prints a line
 * to its console for every failed request, whatever the page does with the answer. A test names the refusal it expects — the
 * status and the end of the address — and only a line for exactly that one is left out; any other console error still fails
 * the test.
 */
export interface IExpectedRefusal {
    status: number;
    /** The end of the address the refusal comes from, e.g. `/api/v1/session`. */
    pathEnd: string;
}

/** The line Chromium prints for a request that was answered with an error status. */
const FAILED_LOAD = /^Failed to load resource: the server responded with a status of (\d{3})\b/;

/** Whether `text`, printed for the address `url`, is the browser's line for one of the refusals the test expects. */
export function isExpectedRefusal(text: string, url: string, expected: readonly IExpectedRefusal[]): boolean {
    const match = FAILED_LOAD.exec(text);
    if (!match) return false;
    const status = Number(match[1]);
    const path = new URL(url, 'http://placeholder.invalid').pathname;
    return expected.some((refusal) => refusal.status === status && path.endsWith(refusal.pathEnd));
}

export function attachConsoleCollector(
    page: Page,
    errors: IConsoleError[],
    expected: readonly IExpectedRefusal[] = []
) {
    page.on('console', (msg) => {
        if (msg.type() !== 'error') return;
        if (isExpectedRefusal(msg.text(), msg.location().url, expected)) return;
        errors.push({ type: msg.type(), text: msg.text() });
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

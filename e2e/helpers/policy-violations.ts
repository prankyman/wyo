import type { Page } from '@playwright/test';

/** One refusal by the page's Content-Security-Policy, as the browser reports it (`securitypolicyviolation`). */
export interface IPolicyViolation {
    /** The directive that refused: `script-src-elem`, `style-src-attr`, `connect-src` … */
    directive: string;
    /** What was refused: an address, or `inline` / `eval`. */
    blocked: string;
    /** The file and line it came from. */
    source: string;
}

/**
 * Starts recording every refusal by the page's security policy in every document the page opens from now on. The browser prints
 * some refusals to its console and some it does not (a caught `eval`), so the console check alone would miss them: this listens to
 * the refusal event itself (https://developer.mozilla.org/docs/Web/API/SecurityPolicyViolationEvent). Call it BEFORE the page opens.
 */
export async function watchPolicyViolations(page: Page): Promise<void> {
    await page.addInitScript(() => {
        const store = ((window as unknown as { __policyViolations?: string[][] }).__policyViolations ??= []);
        document.addEventListener('securitypolicyviolation', (event) => {
            store.push([event.effectiveDirective, event.blockedURI, `${event.sourceFile}:${event.lineNumber}`]);
        });
    });
}

/** The refusals recorded so far on the page's current document. */
export async function policyViolations(page: Page): Promise<IPolicyViolation[]> {
    const raw = await page.evaluate(
        () => (window as unknown as { __policyViolations?: string[][] }).__policyViolations ?? []
    );
    return raw.map(([directive, blocked, source]) => ({ directive, blocked, source }));
}

import type { BrowserContext, Page } from '@playwright/test';

/**
 * Records every request a page makes, so a test can say where the page went (rule R3: no request leaves for any address but the
 * wyo server's own). One recorder per browser context, started before the page is opened: it sees the page's own requests, those of
 * its frames, its workers and its web sockets.
 */

export interface IRecordedRequest {
    kind: 'request' | 'web socket';
    method: string;
    url: string;
}

export interface IRequestLog {
    /** Everything the context asked for so far, in order. */
    all(): IRecordedRequest[];
    /**
     * What went to an address that is not one of `allowedOrigins` (`http://127.0.0.1:8800`). A `data:`, `blob:` or `about:` address
     * is not a request to anywhere and is left out.
     */
    outside(allowedOrigins: readonly string[]): IRecordedRequest[];
}

/** Schemes that never leave the browser. */
const LOCAL_SCHEMES = new Set(['data:', 'blob:', 'about:']);

/** The origin a recorded address goes to; web socket addresses count as the `http` origin they upgrade from. */
function originOf(url: string): string | null {
    const address = new URL(url);
    if (LOCAL_SCHEMES.has(address.protocol)) return null;
    if (address.protocol === 'ws:') return `http://${address.host}`;
    if (address.protocol === 'wss:') return `https://${address.host}`;
    return address.origin;
}

/** Starts recording the requests of `context` (and of every page it opens). */
export function recordRequests(context: BrowserContext): IRequestLog {
    const recorded: IRecordedRequest[] = [];
    context.on('request', (request) => {
        recorded.push({ kind: 'request', method: request.method(), url: request.url() });
    });
    const watchPage = (page: Page): void => {
        page.on('websocket', (socket) => {
            recorded.push({ kind: 'web socket', method: 'GET', url: socket.url() });
        });
    };
    context.pages().forEach(watchPage);
    context.on('page', watchPage);

    return {
        all: () => [...recorded],
        outside: (allowedOrigins) =>
            recorded.filter((item) => {
                const origin = originOf(item.url);
                return origin !== null && !allowedOrigins.includes(origin);
            })
    };
}

import { describe, expect, it } from 'vitest';
import * as layout from './+layout';

describe('the root layout options', () => {
    // The wyo interface is carried by the wyo server as static files (`internal/webui/dist`) with one fallback page: nothing
    // is rendered on a server and no page is prerendered. Either option turned on would make the build fail or write pages
    // the wyo server does not serve.
    it('renders in the browser only and prerenders nothing', () => {
        expect(layout.ssr).toBe(false);
        expect(layout.prerender).toBe(false);
    });
});

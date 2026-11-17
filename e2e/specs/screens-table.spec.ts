import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { expect, test } from '@playwright/test';
import { pageRoutesUnder, routeOfPageFile, SCREENS } from '../support/screens.js';

/**
 * The screens table has a row for every page and nothing else (`e2e/support/screens.ts`). A plain Node test: it opens no browser
 * and starts no wyo server, so it imports `test` from Playwright itself and not from `e2e/fixtures/test.ts`.
 */
const root = path.resolve(import.meta.dirname, '../..');
const routesFolder = path.join(root, 'src/routes');
const tableRoutes = SCREENS.map((screen) => screen.route).sort();

test('every page under src/routes has a row in the screens table', () => {
    const missing = pageRoutesUnder(routesFolder).filter((route) => !tableRoutes.includes(route));
    expect(missing, 'pages with no row: add them to e2e/support/screens.ts').toEqual([]);
});

test('every row of the screens table has a page under src/routes, once', () => {
    const pages = pageRoutesUnder(routesFolder);
    expect(
        tableRoutes.filter((route) => !pages.includes(route)),
        'rows with no page: delete them from e2e/support/screens.ts'
    ).toEqual([]);
    expect(
        tableRoutes.filter((route, index) => tableRoutes.indexOf(route) !== index),
        'routes that have two rows'
    ).toEqual([]);
});

test('every row names a spec that measures its page with the sweep', () => {
    const unmeasured = SCREENS.filter((screen) => {
        const spec = path.join(root, 'e2e/specs', screen.sweptBy);
        if (!existsSync(spec)) return true;
        return !readFileSync(spec, 'utf8').includes(`sweepScreen(page, screenOf('${screen.route}'))`);
    }).map((screen) => `${screen.route} is not measured by ${screen.sweptBy}`);
    expect(unmeasured).toEqual([]);
});

test('a page file stands for its route: groups in brackets are dropped, parameters are kept', () => {
    expect(routeOfPageFile('+page.svelte')).toBe('/');
    expect(routeOfPageFile(path.join('showcase', '+page.svelte'))).toBe('/showcase');
    expect(routeOfPageFile(path.join('(app)', 'items', '[id]', '+page@list.svelte'))).toBe('/items/[id]');
});

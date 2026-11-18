import { readdirSync } from 'node:fs';
import path from 'node:path';

/**
 * The screens table: one row for every page of the wyo interface (every `+page.svelte` under `src/routes/`). The constitution
 * (`.specify/memory/constitution.md`, «Every screen is mobile-first») requires every page to be measured, and
 * `e2e/specs/screens-table.spec.ts` fails for a page with no row, a row with no page, and a row whose spec does not measure it,
 * so a new page cannot ship unmeasured.
 *
 * A row is `{ route, ready, sweptBy }`:
 *
 * - `route`: the folder path of the page, route groups such as `(app)` dropped: `/showcase`, `/items/[id]`. A page with
 *   parameters later gets the address to open in its row.
 * - `ready`: the `data-test` of the element that is on the page once it is drawn and has its data; the sweep and the
 *   outside-requests test wait for it, never for a fixed time.
 * - `sweptBy`: the spec file in `e2e/specs/` that measures the page with `sweepScreen(page, screenOf('<route>'))`
 *   (`e2e/helpers/sweep.ts`).
 *
 * To add a page: add its row here (copy a neighbour), measure it in the spec the row names, then run
 * `pnpm test:e2e e2e/specs/screens-table.spec.ts` and that spec.
 */
export interface IScreen {
    route: string;
    ready: string;
    sweptBy: string;
}

export const SCREENS: readonly IScreen[] = [
    { route: '/', ready: 'overview-cards', sweptBy: 'overview.spec.ts' },
    { route: '/settings', ready: 'settings-sections', sweptBy: 'settings.spec.ts' },
    { route: '/account', ready: 'account-login', sweptBy: 'account.spec.ts' },
    { route: '/auth/login', ready: 'login-send', sweptBy: 'sign-in.spec.ts' },
    { route: '/showcase', ready: 'showcase', sweptBy: 'showcase-sweep.spec.ts' }
];

/** The pages a visitor with no session can open: the sign-in page and the showcase. Every other page needs the owner signed in. */
const PUBLIC_ROUTES: readonly string[] = ['/auth/login', '/showcase'];

/** Whether the page of `route` is open only to the signed-in owner. */
export function needsSignIn(route: string): boolean {
    return !PUBLIC_ROUTES.includes(route);
}

/** The row of `route`; throws when the table has none, so a spec cannot measure a page the table does not know. */
export function screenOf(route: string): IScreen {
    const row = SCREENS.find((screen) => screen.route === route);
    if (!row) throw new Error(`e2e/support/screens.ts has no row for ${route}`);
    return row;
}

/** A page file's name: `+page.svelte`, or `+page@name.svelte` (a page with a layout of its own). */
const PAGE_FILE = /^\+page(@[^.]*)?\.svelte$/;

/** The route a page file stands for: `showcase/+page.svelte` is `/showcase`; a route group in brackets is dropped. */
export function routeOfPageFile(relativeFile: string): string {
    const folders = path
        .dirname(relativeFile)
        .split(path.sep)
        .filter((folder) => folder !== '.' && !/^\(.*\)$/.test(folder));
    return `/${folders.join('/')}`;
}

/** The routes of every page file under `routesFolder`, sorted. */
export function pageRoutesUnder(routesFolder: string): string[] {
    const routes: string[] = [];
    const walk = (folder: string): void => {
        for (const entry of readdirSync(folder, { withFileTypes: true })) {
            const full = path.join(folder, entry.name);
            if (entry.isDirectory()) walk(full);
            else if (PAGE_FILE.test(entry.name)) routes.push(routeOfPageFile(path.relative(routesFolder, full)));
        }
    };
    walk(routesFolder);
    return routes.sort();
}

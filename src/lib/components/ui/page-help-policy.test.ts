import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * One «?» per page: every working page has ONE «?» beside its heading — what the page is, why it is needed, its scenarios — and
 * NO introduction text under the heading. This check reads every page so none is left out. The heading and its «?» are
 * `PageHeading` (`src/lib/components/ui/PageHeading.svelte`), so the check is: the page renders `PageHeading` with a `help`, and
 * draws no `<h1>` followed by a `HelpTooltip` by hand. A page whose heading is drawn by a component it imports is checked
 * through that component (`headingFile`), through up to three imports (a page draws a sheet, which draws a sheet header, which
 * draws `PageHeading`).
 */

const ROOT = process.cwd();
const ROUTE_ROOTS = ['src/routes'];

/** Pages without a «?», each with its reason. The list starts empty; a plan that adds a page of another kind names it here. */
const EXEMPT: ReadonlyArray<[pattern: RegExp, reason: string]> = [];

/** The Svelte components a file imports, as paths from the repository's root (`$lib/…` and `./…`). */
function importedComponents(file: string, source: string): string[] {
    return [...source.matchAll(/import\s+\w+\s+from\s+'([^']+\.svelte)'/g)].map(([, from]) =>
        from.startsWith('$lib/')
            ? `src/lib/${from.slice(5)}`
            : path.posix.normalize(path.posix.join(path.posix.dirname(file), from))
    );
}

const drawsHeading = (source: string): boolean => source.includes('<PageHeading') || source.includes('<h1');

/**
 * The file that draws the page's heading: the page itself, or the first component it imports that has a `PageHeading` or an
 * `<h1>` (a detail screen's own heading component).
 */
function headingFile(file: string): string | null {
    const source = fs.readFileSync(path.join(ROOT, file), 'utf8');
    if (drawsHeading(source)) return file;
    const imported = headingComponent(file, source, 3);
    if (imported) return imported;
    // A page inside a screen whose layout draws the heading and its «?» (a screen with tabs): the layout is checked.
    for (
        let dir = path.posix.dirname(file);
        ROUTE_ROOTS.some((root) => dir.startsWith(`${root}/`));
        dir = path.posix.dirname(dir)
    ) {
        const layout = `${dir}/+layout.svelte`;
        const full = path.join(ROOT, layout);
        if (fs.existsSync(full) && drawsHeading(fs.readFileSync(full, 'utf8'))) return layout;
    }
    return null;
}

/** The first component the file imports, up to `depth` imports deep, that draws a heading; null when none does. */
function headingComponent(file: string, source: string, depth: number): string | null {
    if (depth === 0) return null;
    for (const component of importedComponents(file, source)) {
        const full = path.join(ROOT, component);
        if (!fs.existsSync(full)) continue;
        const componentSource = fs.readFileSync(full, 'utf8');
        if (drawsHeading(componentSource)) return component;
        const deeper = headingComponent(component, componentSource, depth - 1);
        if (deeper) return deeper;
    }
    return null;
}

/** How far below the opening `<h1` a hand-drawn «?» may stand: the heading row's wrapper and the heading's own lines. */
const HELP_WINDOW = 1600;
/** How far below the heading an introduction would stand. */
const INTRO_WINDOW = 500;

function pagesUnder(dir: string): string[] {
    const full = path.join(ROOT, dir);
    if (!fs.existsSync(full)) return [];
    return fs.readdirSync(full, { withFileTypes: true }).flatMap((entry) => {
        const relative = `${dir}/${entry.name}`;
        if (entry.isDirectory()) return pagesUnder(relative);
        return entry.name === '+page.svelte' ? [relative] : [];
    });
}

function withoutComments(source: string): string {
    return source.replace(/<!--[\s\S]*?-->/g, '');
}

/** Where the `PageHeading` tag ends: at its `/>`, or at its `</PageHeading>` when it holds snippets. */
function endOfPageHeading(source: string, open: number): number {
    const ends = [source.indexOf('/>', open), source.indexOf('</PageHeading>', open)].filter((index) => index !== -1);
    return ends.length ? Math.min(...ends) : source.length;
}

/** What is wrong with one page's heading, or nothing. */
function problemOf(file: string): string | null {
    const heading = headingFile(file);
    if (!heading) return 'no heading (PageHeading) to put the «?» beside';
    const source = withoutComments(fs.readFileSync(path.join(ROOT, heading), 'utf8'));
    const h1 = source.indexOf('<h1');
    if (h1 !== -1 && /<HelpTooltip\b/.test(source.slice(h1, h1 + HELP_WINDOW)))
        return 'the heading and its «?» are drawn by hand: render PageHeading';
    const open = source.indexOf('<PageHeading');
    if (open === -1) return 'no PageHeading: the heading is drawn without the page’s one «?»';
    const close = endOfPageHeading(source, open);
    // `help="…"`, `help={…}` or the shorthand `{help}` of a component that passes its own prop on.
    if (!/\bhelp=|\{help\}/.test(source.slice(open, close))) return 'PageHeading without its help text';
    // A paragraph right under the heading that is an introduction: marked `-intro`, or worded by an `….intro` key. A section's
    // own intro further down the page is not one.
    const after = source
        .slice(close)
        .replace(/^(\/>|<\/PageHeading>)/, '')
        .slice(0, INTRO_WINDOW);
    if (/^\s*(?:<!--[\s\S]*?-->\s*)*<p\b[^>]*(?:data-test="[^"]*-intro"|>\s*\{t\('[a-zA-Z0-9_.]+\.intro')/.test(after))
        return 'introduction text under the heading';
    return null;
}

describe('one «?» per page', () => {
    const pages = ROUTE_ROOTS.flatMap(pagesUnder).filter((file) => !EXEMPT.some(([pattern]) => pattern.test(file)));

    it('finds the pages it checks', () => {
        expect(pages.length).toBeGreaterThan(0);
    });

    it('gives every working page one «?» beside its heading and no introduction under it', () => {
        const problems = pages.flatMap((file) => {
            const problem = problemOf(file);
            return problem ? [`${file}: ${problem}`] : [];
        });
        expect(problems).toEqual([]);
    });

    it('says why every exempt page has no «?»', () => {
        for (const [pattern, reason] of EXEMPT) expect(reason, String(pattern)).not.toBe('');
    });
});

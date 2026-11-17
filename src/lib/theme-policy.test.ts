import { readFileSync, readdirSync } from 'node:fs';
import { join, relative } from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * The theme is one file. Every colour of the interface is named once, in the `@theme` block of `src/app.css`, as the scales
 * `brand`, `ink`, `ok`, `wait`, `warn` and `stop` (each 50 to 950) plus `white` and `black`. A component writes `bg-brand-600`,
 * `text-ink-800`, `bg-ok-50` - never a Tailwind palette name (`bg-indigo-600`, `text-slate-500`) and never a colour code
 * (`#4f46e5`, `rgb(...)`, `oklch(...)`), so that changing the look of the whole interface is an edit of that one block.
 *
 * Tailwind's own palette is switched off in the theme (`--color-*: initial`), so a palette class would build to nothing and
 * the element would silently lose its colour; this test is what says so out loud.
 *
 * `white` and `black` are allowed: they are named in the theme (a card's surface and a window's backdrop need exactly a pure
 * white and a pure black, which no scale expresses), so `bg-white` and `bg-black/50` are theme colours like the rest.
 *
 * The allow-list starts empty. An entry is a file and a written reason, and is added only when a colour code truly cannot be a
 * theme token (a file that draws the colours of something outside the interface).
 */
const ALLOWED: Readonly<Record<string, string>> = {};

const ROOT = join(import.meta.dirname, '../..');
const THEME_FILE = 'src/app.css';
const THIS_FILE = 'src/lib/theme-policy.test.ts';

const SCALES = ['brand', 'ink', 'ok', 'wait', 'warn', 'stop'] as const;
const SHADES = ['50', '100', '200', '300', '400', '500', '600', '700', '800', '900', '950'] as const;
const PALETTES = [
    'slate',
    'gray',
    'zinc',
    'neutral',
    'stone',
    'red',
    'orange',
    'amber',
    'yellow',
    'lime',
    'green',
    'emerald',
    'teal',
    'cyan',
    'sky',
    'blue',
    'indigo',
    'violet',
    'purple',
    'fuchsia',
    'pink',
    'rose'
] as const;

/** `bg-indigo-600`, `hover:text-slate-50/60`, `var(--color-stone-900)`: a Tailwind palette name followed by a shade. */
const PALETTE_CLASS = new RegExp(
    `(?<![\\w])(?:--color|[a-z][a-z-]*)-(?:${PALETTES.join('|')})-(?:${SHADES.join('|')})(?![\\w])`
);
/** A theme scale used with a shade the theme does not define (`bg-ink-150`): it would build to nothing. */
const SCALE_CLASS = new RegExp(`(?<![\\w-])(?:--color|[a-z][a-z-]*)-(${SCALES.join('|')})-(\\d+)(?![\\w])`, 'g');
const COLOUR_FUNCTION = /\b(?:rgb|rgba|hsl|hsla|oklch|oklab|lab|lch|hwb)\(/;
/** `#fff`, `#4f46e5`, `#4f46e580` after a quote, a space, a bracket, a colon or a comma - not a word's middle. */
const HEX_COLOUR = /(?<=['"`(\s:[,=])#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})(?![\w-])/;
/** A line that names a page anchor or a hash, where `#add` or `#face` is an address and not a colour. */
const ANCHOR_LINE = /href|\.hash|location|anchor|querySelector|getElementById|aria-controls|popovertarget/i;

/** The source without its comments, so a rule written out in prose cannot trip the scan. */
function withoutComments(source: string): string {
    return source
        .replace(/<!--[\s\S]*?-->/g, '')
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/(^|[^:'"`])\/\/.*$/gm, '$1');
}

/** What is wrong with the colours of one source, one line each. */
export function colourFindings(file: string, source: string): string[] {
    const found: string[] = [];
    withoutComments(source)
        .split('\n')
        .forEach((line, index) => {
            const at = `${file}:${index + 1}`;
            const palette = PALETTE_CLASS.exec(line);
            if (palette)
                found.push(
                    `${at} uses the Tailwind palette class "${palette[0]}" - use brand, ink, ok, wait, warn or stop`
                );
            for (const match of line.matchAll(SCALE_CLASS)) {
                if (!(SHADES as readonly string[]).includes(match[2]))
                    found.push(`${at} uses "${match[0]}", a shade the theme does not define`);
            }
            if (COLOUR_FUNCTION.test(line))
                found.push(`${at} writes a colour function - name the colour in ${THEME_FILE}`);
            if (HEX_COLOUR.test(line) && !ANCHOR_LINE.test(line))
                found.push(`${at} writes a colour code - name the colour in ${THEME_FILE}`);
        });
    return found;
}

function sourceFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = join(dir, entry.name);
        if (entry.isDirectory()) sourceFiles(full, acc);
        else if (/\.(ts|svelte|html|css|js)$/.test(entry.name)) acc.push(relative(ROOT, full).split('\\').join('/'));
    }
    return acc;
}

describe('the theme is one file', () => {
    it('defines every scale of the theme at every shade, and white and black', () => {
        const theme = readFileSync(join(ROOT, THEME_FILE), 'utf8');
        const missing = SCALES.flatMap((scale) =>
            SHADES.filter((shade) => !new RegExp(`--color-${scale}-${shade}:`).test(theme)).map(
                (shade) => `--color-${scale}-${shade}`
            )
        );
        for (const name of ['--color-white:', '--color-black:'])
            if (!theme.includes(name)) missing.push(name.slice(0, -1));

        expect(missing).toEqual([]);
        expect(theme).toContain('--color-*: initial');
    });

    it('finds a palette class, a bad shade and a colour code, and lets the theme classes and anchors through', () => {
        const bad = [
            ['<div class="bg-indigo-600"></div>', 'palette'],
            ['<div class="hover:text-slate-50/60"></div>', 'palette'],
            ['<div class="border-red-300 p-2"></div>', 'palette'],
            ["const c = 'var(--color-stone-900)';", 'palette'],
            ['<div class="bg-ink-150"></div>', 'shade'],
            ['<div style="color: #4f46e5"></div>', 'code'],
            ['<div class="bg-[#fff]"></div>', 'code'],
            ["const c = 'rgb(0, 0, 0)';", 'function'],
            ['x { color: oklch(50% 0.1 20); }', 'function']
        ] as const;
        for (const [source] of bad) expect(colourFindings('x', source), source).toHaveLength(1);

        const good = [
            '<div class="bg-brand-600 text-white hover:bg-brand-700 bg-black/50 text-ink-800 bg-ok-50 border-stop-300"></div>',
            '<a href="#add">link</a>',
            '// a note: never write bg-indigo-600 or #4f46e5 by hand',
            '/* bg-slate-100 */',
            "const url = 'https://example.com/#face';"
        ];
        for (const source of good) expect(colourFindings('x', source), source).toEqual([]);
    });

    it('has no palette class and no colour code in the interface outside the theme file', () => {
        const files = sourceFiles(join(ROOT, 'src')).filter(
            (file) => file !== THEME_FILE && file !== THIS_FILE && !(file in ALLOWED)
        );
        expect(files.length).toBeGreaterThan(20);

        const findings = files.flatMap((file) => colourFindings(file, readFileSync(join(ROOT, file), 'utf8')));

        expect(findings, findings.join('\n')).toEqual([]);
    });

    it('keeps every allow-list entry honest: a reason, and a file that still exists', () => {
        for (const [file, reason] of Object.entries(ALLOWED)) {
            expect(reason.length, file).toBeGreaterThan(10);
            expect(() => readFileSync(join(ROOT, file), 'utf8'), file).not.toThrow();
        }
    });
});

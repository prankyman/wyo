import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Enforces that the colours of a status are decided once, in `status-tones.ts`. A status names its tone
 * (`Record<status, TTone>`, kept beside the status words or in its `*Badge.svelte`), and the badge draws the tone's look.
 * Without it each screen declares its own map of colour classes — `COLORS`, `STATUS_COLORS`, `…_COLORS`,
 * `…BADGE_CLASS` — and the same state is drawn in up to four shades.
 *
 * It fails such a declaration that holds Tailwind colour classes anywhere under `src/` outside `status-tones.ts`.
 */

const ROOT = path.resolve(__dirname, '../../../..');
const HOME = 'src/lib/components/ui/status-tones.ts';
const MAP = /\bconst (COLORS|STATUS_COLORS|[A-Z_]*_COLORS|[A-Z_]*BADGE_CLASS)\b[^=]*=\s*\{([\s\S]*?)\}/g;
const COLOUR = /\b(?:bg|text|border)-[a-z]+-\d{2,3}\b/;

function walk(dir: string, out: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(full, out);
        else if (/\.(svelte|ts)$/.test(entry.name) && !/\.test\.ts$/.test(entry.name)) out.push(full);
    }
    return out;
}

/** The colour maps a source declares: their names. */
function colourMaps(source: string): string[] {
    return [...source.matchAll(MAP)].filter((match) => COLOUR.test(match[2])).map((match) => match[1]);
}

describe('the status colours', () => {
    it('are declared only in status-tones.ts: every other status map names tones', () => {
        const strays = walk(path.join(ROOT, 'src'))
            .map((file) => path.relative(ROOT, file).split(path.sep).join('/'))
            .filter((file) => file !== HOME)
            .flatMap((file) =>
                colourMaps(fs.readFileSync(path.join(ROOT, file), 'utf8')).map((name) => `${file}: ${name}`)
            );

        expect(strays).toEqual([]);
    });
});

describe('the scan of this test', () => {
    it('finds a map of colour classes and passes a map of tones', () => {
        expect(
            colourMaps(
                [
                    "const STATUS_COLORS: Record<S, string> = { open: 'bg-wait-50 text-wait-800' };",
                    "const KITTING_BADGE_CLASS = { done: 'bg-ok-100 text-ok-900' };",
                    "const ORDER_STATUS_TONE: Record<S, TTone> = { open: 'wait' };",
                    "const COLORS = { a: 'font-bold' };"
                ].join('\n')
            )
        ).toEqual(['STATUS_COLORS', 'KITTING_BADGE_CLASS']);
    });
});

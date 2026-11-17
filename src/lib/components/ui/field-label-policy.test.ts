import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Enforces "Forms", rule 4: every labelled field of the interface is drawn by
 * `src/lib/components/ui/Field.svelte`, so a field's label looks the same everywhere — small, grey and in the letters it
 * is written in, never in capitals — and that look is tested once, in `Field.svelte.test.ts`, instead of on every page.
 * In the maintainer's other repository, before this rule 229 files drew their own `<label>`, 118 of them through a
 * local `labelClass` constant, and some labels stood in capitals while others did not.
 *
 * Scanned: the markup of every `.svelte` file under `src/` except the test harnesses under `src/test/` — the `<script>`
 * and `<style>` blocks and HTML comments are left out, so prose mentioning `<label>` does not count. Read-only facts
 * (`<dt>`) are not field labels and are not touched.
 *
 * ALLOWLIST is the complete record of the exceptions, each with its reason; an entry for a file that no longer draws a
 * `<label>` fails the second test, so the list cannot outlive its reasons.
 */

const ROOT = path.resolve(__dirname, '../../../..');
const FIELD = 'src/lib/components/ui/Field.svelte';

interface IException {
    path: string;
    reason: string;
}

const ALLOWLIST: IException[] = [];

function svelteFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) svelteFiles(full, acc);
        else if (entry.name.endsWith('.svelte')) acc.push(full);
    }
    return acc;
}

/** The markup only: scripts, styles and comments removed. */
function markupOf(source: string): string {
    return source
        .replace(/<script\b[\s\S]*?<\/script>/g, '')
        .replace(/<style\b[\s\S]*?<\/style>/g, '')
        .replace(/<!--[\s\S]*?-->/g, '');
}

function relative(file: string): string {
    return path.relative(ROOT, file).split(path.sep).join('/');
}

/** Every file under `src/` (harnesses excepted) that draws a `<label>` of its own. */
function filesWithLabels(): string[] {
    return svelteFiles(path.join(ROOT, 'src'))
        .map(relative)
        .filter((file) => !file.startsWith('src/test/') && file !== FIELD)
        .filter((file) => /<label[\s>/]/.test(markupOf(fs.readFileSync(path.join(ROOT, file), 'utf8'))))
        .sort();
}

describe('field label policy (Forms, rule 4)', () => {
    it('draws every labelled field through Field.svelte', () => {
        const allowed = new Set(ALLOWLIST.map((entry) => entry.path));
        expect(filesWithLabels().filter((file) => !allowed.has(file))).toEqual([]);
    });

    it('keeps no allowlist entry for a file that no longer draws a label', () => {
        const drawing = new Set(filesWithLabels());
        expect(ALLOWLIST.filter((entry) => !drawing.has(entry.path)).map((entry) => entry.path)).toEqual([]);
        expect(ALLOWLIST.filter((entry) => entry.reason.length < 20)).toEqual([]);
    });

    it('draws the labels of Field.svelte in the letters they are written in', () => {
        const markup = markupOf(fs.readFileSync(path.join(ROOT, FIELD), 'utf8'));
        const script = fs.readFileSync(path.join(ROOT, FIELD), 'utf8').match(/<script\b[\s\S]*?<\/script>/)?.[0] ?? '';
        expect(markup).not.toMatch(/\buppercase\b/);
        expect(script).not.toMatch(/\buppercase\b/);
    });
});

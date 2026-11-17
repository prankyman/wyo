import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Enforces "Forms", rule 6: every number a person types is `NumberField`
 * (`src/lib/components/ui/NumberField.svelte`), so every number box looks and behaves the same — a text box, the phone's
 * digit keyboard by its places, no arrows, «±» beside a field that accepts a negative — and that is tested once, in
 * `NumberField.svelte.test.ts`.
 *
 * It fails an `<input>` with `type="number"`, `inputmode="decimal"` or `inputmode="numeric"` anywhere else under `src/`
 * (the test harnesses under `src/test/` excepted). Digit codes are not numbers — a leading zero matters in a PIN, an ИНН
 * or an account — so they keep a text box with the digit keyboard; DIGIT_CODES lists each file with the number of such
 * boxes in it and the reason, and a count that no longer matches fails, so the list cannot outlive its reasons. The
 * slider (`type="range"`) is not a typed box and is not counted. A box whose keyboard comes from a variable
 * is not seen by this scan; it is a digit code.
 */

const ROOT = path.resolve(__dirname, '../../../..');
const NUMBER_FIELD = 'src/lib/components/ui/NumberField.svelte';

interface IDigitCodes {
    path: string;
    /** How many digit-code boxes the file draws. */
    boxes: number;
    reason: string;
}

const DIGIT_CODES: IDigitCodes[] = [];

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

/**
 * Every `<input …>` and `<TextBox …>` tag of the markup, read to its closing `>`
 * outside `{…}` (a handler holds `=>`).
 */
function inputTags(markup: string): string[] {
    const tags: string[] = [];
    for (const match of markup.matchAll(/<(?:input|TextBox)\b/g)) {
        let depth = 0;
        let end = match.index + match[0].length;
        while (end < markup.length && !(markup[end] === '>' && depth === 0)) {
            if (markup[end] === '{') depth++;
            else if (markup[end] === '}') depth--;
            end++;
        }
        tags.push(markup.slice(match.index, end + 1));
    }
    return tags;
}

const NUMBER_BOX = /\btype="number"|\binputmode="(?:decimal|numeric)"/;

function relative(file: string): string {
    return path.relative(ROOT, file).split(path.sep).join('/');
}

/** The number of number boxes each file draws itself (harnesses and `NumberField` excepted). */
function numberBoxes(): Map<string, number> {
    const counts = new Map<string, number>();
    for (const file of svelteFiles(path.join(ROOT, 'src')).map(relative)) {
        if (file.startsWith('src/test/') || file === NUMBER_FIELD) continue;
        const markup = markupOf(fs.readFileSync(path.join(ROOT, file), 'utf8'));
        const count = inputTags(markup).filter((tag) => NUMBER_BOX.test(tag)).length;
        if (count > 0) counts.set(file, count);
    }
    return counts;
}

describe('number field policy (Forms, rule 6)', () => {
    it('draws every typed number through NumberField.svelte; only the listed digit codes keep their own box', () => {
        const codes = new Map(DIGIT_CODES.map((entry) => [entry.path, entry.boxes]));
        const strays = [...numberBoxes()]
            .filter(([file, count]) => codes.get(file) !== count)
            .map(([file, count]) => `${file}: ${count} number box(es), ${codes.get(file) ?? 0} listed as digit codes`);
        expect(strays).toEqual([]);
    });

    it('keeps no digit-code entry that matches nothing, and gives every entry its reason', () => {
        const boxes = numberBoxes();
        expect(DIGIT_CODES.filter((entry) => boxes.get(entry.path) !== entry.boxes).map((entry) => entry.path)).toEqual(
            []
        );
        expect(DIGIT_CODES.filter((entry) => entry.reason.length < 30)).toEqual([]);
    });
});

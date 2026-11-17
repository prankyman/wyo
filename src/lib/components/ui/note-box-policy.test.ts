import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Every coloured note is `NoteBox`. The tinted box — a border and a background of one colour family — drawn by hand in
 * many files gets a dozen accidental differences; the look of each tone is decided once in
 * `NoteBox.svelte`, and this check keeps it the only copy: a line of a `.svelte` or `.ts` file under `src/` (the tests and the
 * test support left out) that carries both a `bg-<tone>-50` class and a `border-<tone>-200` or `-300` class of the same family
 * fails the build — in a `.ts` module too, because the look can also be kept in class constants (`WARNING_CLASS`, `ERROR_CLASS`,
 * `SUCCESS_CLASS`) that screens splice into their own elements.
 */

const ROOT = process.cwd();
const ONE_COPY = 'src/lib/components/ui/NoteBox.svelte';

/** Files allowed to tint a box by hand, each with its reason. */
const EXCEPTIONS: ReadonlyArray<[file: string, reason: string]> = [
    ['src/lib/components/ui/StepStrip.svelte', 'a done step chip is green: a state of a chip, not a note']
];

/** The tones of `NoteBox` by their theme names (`src/app.css`): warning, error, success and information. */
const TONES = ['warn', 'stop', 'ok', 'brand'];

function sourceFilesUnder(dir: string): string[] {
    return fs.readdirSync(path.join(ROOT, dir), { withFileTypes: true }).flatMap((entry) => {
        const relative = `${dir}/${entry.name}`;
        if (entry.isDirectory())
            return entry.name === 'node_modules' || relative === 'src/test' ? [] : sourceFilesUnder(relative);
        const isSource =
            entry.name.endsWith('.svelte') || (entry.name.endsWith('.ts') && !entry.name.endsWith('.test.ts'));
        return isSource ? [relative] : [];
    });
}

/** The lines of a file that draw a tinted box by hand. */
export function handDrawnNotes(source: string): number[] {
    return source
        .split('\n')
        .flatMap((line, index) =>
            TONES.some((tone) => line.includes(`bg-${tone}-50`) && new RegExp(`border-${tone}-(200|300)\\b`).test(line))
                ? [index + 1]
                : []
        );
}

describe('every coloured note is NoteBox', () => {
    const allowed = new Set(EXCEPTIONS.map(([file]) => file));
    const files = sourceFilesUnder('src').filter((file) => file !== ONE_COPY && !allowed.has(file));

    it('finds the files it checks', () => {
        expect(files.length).toBeGreaterThan(40);
    });

    it('lets no other component tint a box by hand', () => {
        const offenders = files.flatMap((file) => {
            const lines = handDrawnNotes(fs.readFileSync(path.join(ROOT, file), 'utf8'));
            return lines.length ? [`${file}:${lines.join(',')}`] : [];
        });
        expect(offenders).toEqual([]);
    });

    it('names a reason for every exception', () => {
        for (const [file, reason] of EXCEPTIONS) {
            expect(reason, file).not.toBe('');
            expect(fs.existsSync(path.join(ROOT, file)), file).toBe(true);
        }
    });
});

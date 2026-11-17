import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { svelteFilesUnder } from '$test/support/svelte-markup';

/**
 * Every «nothing here yet» box is `EmptyState`. The dashed grey box drawn by hand drifts from file to file;
 * its look is decided once in `EmptyState.svelte`, and this check keeps it the only copy: a `.svelte` file under `src/`
 * that carries `border-dashed border-ink-300` fails the build.
 */

const ROOT = process.cwd();
const ONE_COPY = 'src/lib/components/ui/EmptyState.svelte';

/** Files allowed to draw a dashed box by hand, each with its reason. */
const EXCEPTIONS: ReadonlyArray<[file: string, reason: string]> = [];

describe('every empty-state box is EmptyState', () => {
    const allowed = new Set(EXCEPTIONS.map(([file]) => file));
    const files = svelteFilesUnder('src').filter((file) => file !== ONE_COPY && !allowed.has(file));

    it('finds the files it checks', () => {
        expect(files.length).toBeGreaterThan(40);
    });

    it('lets no other component draw the dashed box by hand', () => {
        const offenders = files.filter((file) =>
            fs.readFileSync(path.join(ROOT, file), 'utf8').includes('border-dashed border-ink-300')
        );
        expect(offenders).toEqual([]);
    });

    it('names a reason for every exception', () => {
        for (const [file, reason] of EXCEPTIONS) {
            expect(reason, file).not.toBe('');
            expect(fs.existsSync(path.join(ROOT, file)), file).toBe(true);
        }
    });
});

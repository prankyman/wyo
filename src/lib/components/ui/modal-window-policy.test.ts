import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { svelteFilesUnder } from '$test/support/svelte-markup';

/**
 * Every window is `ModalWindow`. The glue the rule demands —
 * `lockBodyScroll`, `setBackgroundInert`, `trapTabKey`, `focusableElements` from `overlay-a11y.ts`, and the `role="dialog"`
 * element itself — had once been copied into 29 files of the maintainer's other repository, each with its own small differences and only two of
 * them testing the inert page, the scroll lock or the focus return. It is written once, in `ModalWindow.svelte`, and this check keeps it so: a
 * `.svelte` file under `src/` that calls one of those functions or draws `role="dialog"` by hand fails the build.
 */

const ROOT = process.cwd();
const ONE_COPY = 'src/lib/components/ui/ModalWindow.svelte';

/** The files allowed to draw `role="dialog"` or to call the glue without `ModalWindow`, each with its reason. */
const EXCEPTIONS: ReadonlyArray<[file: string, reason: string]> = [
    [
        'src/lib/components/shell/NotificationBell.svelte',
        'the bell’s panel is a non-modal dialog anchored to the bell (no backdrop, nothing inert, the page stays usable; closed by a click outside), not a window; it moves the focus onto its first control with focusableElements when it opens'
    ]
];

const GLUE = ['lockBodyScroll(', 'setBackgroundInert(', 'trapTabKey(', 'focusableElements('];

/** The markup of a component: the file without its `<script>` blocks and HTML comments. */
function markupOf(source: string): string {
    return source.replace(/<script[\s\S]*?<\/script>/g, '').replace(/<!--[\s\S]*?-->/g, '');
}

describe('every window is ModalWindow', () => {
    const allowed = new Set(EXCEPTIONS.map(([file]) => file));
    const files = svelteFilesUnder('src').filter((file) => file !== ONE_COPY && !allowed.has(file));

    it('finds the files it checks', () => {
        expect(files.length).toBeGreaterThan(50);
    });

    it('lets no other component call the overlay glue of overlay-a11y.ts', () => {
        const offenders = files.flatMap((file) => {
            const source = fs.readFileSync(path.join(ROOT, file), 'utf8');
            const calls = GLUE.filter((call) => source.includes(call));
            return calls.length ? [`${file}: ${calls.join(', ')}`] : [];
        });
        expect(offenders).toEqual([]);
    });

    it('lets no other component draw a role="dialog" element by hand', () => {
        const offenders = files.filter((file) =>
            /\brole="dialog"/.test(markupOf(fs.readFileSync(path.join(ROOT, file), 'utf8')))
        );
        expect(offenders).toEqual([]);
    });

    it('names a reason for every exception, and every exception still exists and still draws the role', () => {
        for (const [file, reason] of EXCEPTIONS) {
            expect(reason, file).not.toBe('');
            expect(fs.existsSync(path.join(ROOT, file)), file).toBe(true);
            expect(/\brole="dialog"/.test(markupOf(fs.readFileSync(path.join(ROOT, file), 'utf8'))), file).toBe(true);
        }
    });
});

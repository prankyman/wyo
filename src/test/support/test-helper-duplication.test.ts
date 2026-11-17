import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Standing guarantee that the test set-up written once stays written once - the tests' twin of
 * `src/lib/utils/helper-duplication.test.ts`. A shared helper declared again in a test file, or in another support file, fails
 * here with the file and the home to use instead. Add a name the day a second copy of it appears, or the day a helper is written
 * that a second test is likely to need.
 */
const HOMES: Record<string, string> = {
    // A tick for a form's answer, a form sent in a test.
    settle: 'src/test/support/superform-answer.ts',
    flushTicks: 'src/test/support/superform-answer.ts',
    sendForm: 'src/test/support/superform-answer.ts',
    // A page rendered with its data.
    renderPageWith: 'src/test/support/render-page.ts',
    // The text system's mock.
    createI18nMock: 'src/test/support/i18n.ts',
    // The reactive stand-ins of `$app/state`.
    setPageUrl: 'src/test/support/page-state.svelte.ts',
    setPageStatus: 'src/test/support/page-state.svelte.ts',
    setPageData: 'src/test/support/page-state.svelte.ts',
    setPageError: 'src/test/support/page-state.svelte.ts',
    setNavigating: 'src/test/support/page-state.svelte.ts',
    resetPage: 'src/test/support/page-state.svelte.ts',
    stubHistoryBack: 'src/test/support/navigation.ts',
    // The markup readers of the policy tests.
    markupTree: 'src/test/support/svelte-markup.ts',
    rootMarkup: 'src/test/support/svelte-markup.ts',
    walkMarkup: 'src/test/support/svelte-markup.ts',
    svelteFilesUnder: 'src/test/support/svelte-markup.ts'
};

const NAMES = Object.keys(HOMES).join('|');
const DECLARATION = new RegExp(
    `^\\s*(?:export\\s+)?(?:async\\s+)?(?:function\\*?\\s+(${NAMES})\\s*[<(]|(?:const|let)\\s+(${NAMES})\\s*[:=])`,
    'gm'
);

/** Every test file under `src`, and every shared support file (a support file declaring another's helper is a copy too). */
function testFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = join(dir, entry.name);
        if (entry.isDirectory()) {
            if (entry.name !== 'node_modules') testFiles(full, acc);
        } else if (
            /\.test\.ts$/.test(entry.name) ||
            (dir === join('src', 'test', 'support') && /\.ts$/.test(entry.name))
        ) {
            acc.push(full);
        }
    }
    return acc;
}

const FILES = testFiles('src').map((file) => ({ file, source: readFileSync(file, 'utf8') }));

describe('test helper duplication', () => {
    it('keeps every home in the table real', () => {
        const missing = Object.entries(HOMES)
            .filter(
                ([name, home]) =>
                    !new RegExp(`(?:function\\*?|const|let)\\s+${name}\\b`).test(readFileSync(home, 'utf8'))
            )
            .map(([name, home]) => `${name} is not declared in ${home}`);

        expect(missing).toEqual([]);
    });

    it('declares each shared test helper only in its home', () => {
        const strays: string[] = [];
        for (const { file, source } of FILES) {
            if (file === join('src', 'test', 'support', 'test-helper-duplication.test.ts')) continue;
            for (const match of source.matchAll(DECLARATION)) {
                const name = match[1] ?? match[2];
                if (HOMES[name] !== file) strays.push(`${file} declares ${name} (home: ${HOMES[name]})`);
            }
        }
        expect(FILES.length).toBeGreaterThan(10);
        expect(strays).toEqual([]);
    });
});

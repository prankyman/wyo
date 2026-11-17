import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { marginFindings, stackFindings } from '$test/support/stack-policy';
import { markupTree, ROOT, rootMarkup, svelteFilesUnder } from '$test/support/svelte-markup';
import allowed from './stack-policy-allowed.json';

/**
 * One vertical gap everywhere. Without it the blocks of a page are spaced by their own margins: a folded section adds 16 px
 * below itself unless told otherwise, a heading's wrapper 24 px, a page's column 24 px more, and one card measures 0, 16 and
 * 24 px between its blocks. Here every page and every stack of blocks is `Stack`
 * (a flex column with `gap-4`, 16 px) and no block brings an outer margin.
 *
 * It fails: a page (`+page.svelte`) whose outermost markup is not a `Stack` — followed into the component that draws the
 * page's whole body; a `Stack` given a gap, a `space-y-*` or a margin; an element or component directly inside a `Stack` with
 * an outer margin (`mb-*`, `mt-*`, `my-*`); a block component (`CollapsibleSection`, `NoteBox`, `PageHeading`, …) given one.
 * A printed page laid out in the paper's millimetres is an exception of `stack-policy-allowed.json`, each with its reason (the list starts empty). The
 * screen sweeps measure the same rule in the browser (`e2e/helpers/stack-gaps.ts`).
 */

const ALLOWED: Readonly<Record<string, string>> = allowed;

// Every .svelte file under `src/` is parsed: under a whole run's load that takes longer than a single test's 5 seconds.
describe('stack policy (one vertical gap everywhere)', { timeout: 60_000 }, () => {
    it('stands every page in a Stack and lets no block bring an outer margin', () => {
        const findings = new Set(svelteFilesUnder('src').flatMap((file) => stackFindings(file, ALLOWED)));
        expect([...findings]).toEqual([]);
    });

    it('keeps no exception whose file no longer exists', () => {
        expect(Object.keys(ALLOWED).filter((file) => !fs.existsSync(path.join(ROOT, file)))).toEqual([]);
    });

    it('finds the margins and gaps it guards against, also inside a block and across lines', () => {
        const tree = markupTree(
            [
                '<Stack class="gap-6">',
                '    {#if shown}',
                '        <div\n            class="rounded\n mb-4">x</div>',
                '    {/if}',
                '    <NoteBox tone="info" class="sm:mt-2">y</NoteBox>',
                '    <p class="text-sm">z</p>',
                '</Stack>',
                '<CollapsibleSection {lang} id="s" title="S" class="-my-1">w</CollapsibleSection>',
                '<div class="mb-2">a margin outside a Stack is not a block of a stack</div>'
            ].join('\n')
        );
        expect(marginFindings('page', tree)).toEqual([
            'page:1 <Stack> takes gap-6: its gap is the one 16 px and it has no margin',
            'page:3 <div> directly inside a Stack carries mb-4',
            'page:7 <NoteBox> directly inside a Stack carries mt-2',
            'page:10 <CollapsibleSection> is given -my-1: a block brings no outer margin'
        ]);
    });

    // The frames that hold a page's content in a Stack of their own (`FRAMES` in `$test/support/stack-policy`) are none yet; the test
    // that what stands inside a frame is read like the blocks of a Stack comes with the first frame.

    it('reads a page from its outermost drawn markup, not its head, window or snippets', () => {
        const roots = rootMarkup(
            [
                '<svelte:head><title>T</title></svelte:head>',
                '<svelte:window onkeydown={go} />',
                '{#snippet row(item)}<li>{item}</li>{/snippet}',
                '{#if ready}<Stack as="section">a</Stack>{:else}<div>b</div>{/if}'
            ].join('\n')
        );
        expect(roots.map((node) => node.name)).toEqual(['Stack', 'div']);
    });
});

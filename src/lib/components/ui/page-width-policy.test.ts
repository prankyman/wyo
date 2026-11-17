import { describe, expect, it } from 'vitest';
import {
    classWords,
    markupTree,
    readMarkup,
    svelteFilesUnder,
    walkMarkup,
    type IMarkupNode
} from '$test/support/svelte-markup';

/**
 * Enforces "Screens at full width". A page wrapped in a narrow centred column does nothing on a phone and, on a computer, draws a
 * narrow strip between wide empty margins while the other pages use the full width of the page frame.
 *
 * It fails an element that has both `mx-auto` (centred) and a `max-w-*` (a narrower width) in a page (`src/routes/**`) or a
 * component the pages draw (`src/lib/components/app/**`). The page frame (the layout and the top bar of the shell) is the named
 * exception, added by the plan that draws it, with its reason; the list starts empty.
 */

const EXCEPTIONS: Readonly<Record<string, string>> = {
    'src/routes/+layout.svelte': 'the page frame: the one centred column every page is drawn in'
};

const isNarrowCentred = (node: IMarkupNode) => {
    const words = classWords(node);
    return words.includes('mx-auto') && words.some((word) => /(^|:)max-w-/.test(word));
};

function narrowColumns(nodes: IMarkupNode[], file: string): string[] {
    return Array.from(walkMarkup(nodes), (entry) => entry.node)
        .filter(isNarrowCentred)
        .map((node) => `${file}:${node.line}`);
}

const FILES = svelteFilesUnder('src/routes', 'src/lib/components/app');

// Every page is parsed: under a whole run's load that takes longer than a single test's 5 seconds.
describe('page width policy (floor screens at full width)', { timeout: 60_000 }, () => {
    it('centres no narrow column on a page, but for the page frame', () => {
        const offending = FILES.filter((file) => !(file in EXCEPTIONS)).flatMap((file) =>
            narrowColumns(readMarkup(file), file)
        );
        expect(offending).toEqual([]);
    });

    it('keeps no exception that matches nothing', () => {
        expect(Object.keys(EXCEPTIONS).filter((file) => narrowColumns(readMarkup(file), file).length === 0)).toEqual(
            []
        );
    });

    it('finds the defect it guards against, also when the classes are split over lines', () => {
        const tree = markupTree(
            '<section\n    class="mx-auto flex\n max-w-xl flex-col"><p class="max-w-prose">x</p></section>'
        );
        expect(narrowColumns(tree, 'page')).toEqual(['page:1']);
    });
});

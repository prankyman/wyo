import type { Page } from '@playwright/test';

/**
 * One vertical gap everywhere, measured in the browser (constitution, «One vertical gap everywhere»). Every page and every stack of blocks is `Stack` (`src/lib/components/ui/Stack.svelte`, marked `data-stack`), a
 * flex column whose gap of 16 px is the only space between two neighbouring blocks. Inside every drawn `[data-stack]`, with every
 * closed `<details>` opened (and closed again afterwards), the distance from the bottom of each block to the top of the next one
 * is `STACK_GAP` px within `STACK_GAP_TOLERANCE`. A block's own outer margin, a `space-y-*` or a wrapper with a margin shows
 * here as a larger or smaller distance.
 *
 * A block is a child of the column that is drawn in its flow: one with `display: none` (a hidden form, a closed dialog, a hidden
 * input) or taken out of the flow (`position: absolute` or `fixed`) is not a block; a child with `display: contents` stands for
 * its own children. A block with no height is reported on its own — a flex item with no height still takes a gap, so an empty
 * wrapper inside a Stack is a defect, not a pass. A block that sticks (`position: sticky`, a bar of buttons at the bottom of a
 * long form) is measured where the flow puts it: it is made `relative` for the measurement.
 *
 * Measured in the browser: jsdom lays nothing out. The source rule is `src/lib/components/ui/stack-policy.test.ts`.
 */

/** The one gap between two neighbouring blocks, px (`gap-4`). */
export const STACK_GAP = 16;

/** How far a measured gap may stray from `STACK_GAP`, px (sub-pixel rounding of the layout). */
export const STACK_GAP_TOLERANCE = 0.5;

interface IGapArg {
    gap: number;
    tolerance: number;
}

/**
 * The measurement, run inside the page by `page.evaluate`. It must stay one self-contained function: Playwright sends its source
 * text to the browser, so it cannot call anything defined outside it. One line per problem:
 * `<column> | <block> → <next block> | <distance> px` or `<column> | <block> | no height, yet it takes a gap`.
 */
async function gapsInPage({ gap, tolerance }: IGapArg): Promise<string[]> {
    const closed = Array.from(document.querySelectorAll('details:not([open])')) as HTMLDetailsElement[];
    for (const details of closed) details.open = true;
    const sticky = (Array.from(document.querySelectorAll('[data-stack] > *')) as HTMLElement[]).filter(
        (element) => getComputedStyle(element).position === 'sticky'
    );
    const stickyBefore = sticky.map((element) => element.style.position);
    for (const element of sticky) element.style.position = 'relative';
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));

    try {
        const nameOf = (element: Element): string => {
            const test = element.getAttribute('data-test');
            if (test) return test;
            const classes = typeof element.className === 'string' ? element.className.trim().split(/\s+/) : [];
            return [element.tagName.toLowerCase(), ...classes.slice(0, 4)].filter(Boolean).join('.');
        };
        const blocksOf = (parent: Element, into: Element[]): Element[] => {
            for (const child of Array.from(parent.children)) {
                const style = getComputedStyle(child);
                if (style.display === 'none') continue;
                if (style.display === 'contents') blocksOf(child, into);
                else if (style.position !== 'absolute' && style.position !== 'fixed') into.push(child);
            }
            return into;
        };

        const problems: string[] = [];
        for (const column of Array.from(document.querySelectorAll('[data-stack]'))) {
            // A column that is not drawn (inside a hidden panel, a closed tab) has nothing to measure.
            if (column.getClientRects().length === 0) continue;
            const blocks = blocksOf(column, []);
            const empty = new Set(blocks.filter((block) => block.getBoundingClientRect().height === 0));
            for (const block of empty)
                problems.push(`${nameOf(column)} | ${nameOf(block)} | no height, yet it takes a gap`);
            for (let index = 1; index < blocks.length; index++) {
                const [above, below] = [blocks[index - 1], blocks[index]];
                if (empty.has(above) || empty.has(below)) continue;
                const distance =
                    Math.round((below.getBoundingClientRect().top - above.getBoundingClientRect().bottom) * 10) / 10;
                if (Math.abs(distance - gap) > tolerance)
                    problems.push(`${nameOf(column)} | ${nameOf(above)} → ${nameOf(below)} | ${distance} px`);
            }
        }
        return problems;
    } finally {
        sticky.forEach((element, index) => (element.style.position = stickyBefore[index]));
        for (const details of closed) details.open = false;
    }
}

/** The gap problems of the page that is open, at its current size. */
export async function stackGapProblems(page: Page): Promise<string[]> {
    return page.evaluate(gapsInPage, { gap: STACK_GAP, tolerance: STACK_GAP_TOLERANCE });
}

/**
 * The gap problems of the page that is open at each of `widths` (its height kept), each line starting with the width; the page
 * gets its own size back afterwards.
 */
export async function stackGapProblemsAtWidths(page: Page, widths: readonly number[]): Promise<string[]> {
    const before = page.viewportSize();
    const height = before?.height ?? 900;
    const problems: string[] = [];
    try {
        for (const width of widths) {
            await page.setViewportSize({ width, height });
            for (const problem of await stackGapProblems(page)) problems.push(`${width} px | ${problem}`);
        }
    } finally {
        if (before) await page.setViewportSize(before);
    }
    return problems;
}

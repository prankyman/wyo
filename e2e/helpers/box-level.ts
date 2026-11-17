import type { Page } from '@playwright/test';

/**
 * Where the boxes of a form stand. A box is an input, a select or a text area that is drawn. Three
 * things are measured on the page that is open, with every closed `<details>` opened (and closed again afterwards):
 *
 * - **level**   two boxes of the SAME form (the nearest `form` ancestor; none counts as one) that stand side by side and share more
 *               than 4 px of height start within 1 px of each other. A label that runs over two lines beside a label of one line
 *               pushes its box 16 px lower, and a 44 px «?» button beside a plain label pushed its box 28 px lower; both are
 *               reported here, the first with the words "its label runs over N lines". Boxes of different forms are never
 *               compared: two panels of a page (a list of rename boxes beside a create form) are independent.
 * - **gap**     for a box whose label stands directly above it, the distance from the bottom of the label's text to the top of
 *               the box is at most `MAX_LABEL_GAP` px. The designed gap is 6 px (`gap-1.5`) plus the half-leading of the text's
 *               line; a row made taller than its text (a 44 px «?» beside the label) makes it 20 px.
 * - **overlap** no two boxes overlap.
 *
 * Measured in the browser: jsdom lays nothing out. `levelProblemsAtWidths` repeats the measuring at the widths where a form
 * changes its columns (`LEVEL_WIDTHS`); the phone's rules (390 px, one column) cannot see these defects.
 */

/** The widths a form changes its columns at: `sm` (640), `md` (768), `lg` (1024) and `xl` (1280), as Tailwind names them. */
export const LEVEL_WIDTHS = [640, 768, 1024, 1280] as const;

/** The most a label's text may stand above its box (px). */
export const MAX_LABEL_GAP = 10;

export interface ILevelResult {
    /** One line per problem. */
    problems: string[];
    /** How many boxes were measured (at the widest width measured, for `levelProblemsAtWidths`). */
    boxes: number;
}

interface ILevelArg {
    scope: string | null;
    maxGap: number;
}

/**
 * The measurement, run inside the page by `page.evaluate`. It must stay one self-contained function: Playwright sends its
 * source text to the browser, so it cannot call anything defined outside it.
 */
async function measureInPage({ scope, maxGap }: ILevelArg): Promise<ILevelResult> {
    const root = scope ? document.querySelector(scope) : document.body;
    if (!root) return { problems: [`${scope} is not on the page`], boxes: 0 };

    /** Two boxes start at the same height within this many px. */
    const LEVEL_TOLERANCE = 1;
    /** Boxes stand in one row when they share more than this many px of height. */
    const SHARED_HEIGHT = 4;
    /** Two boxes overlap when they share more than this in both directions. */
    const OVERLAP_TOLERANCE = 1;
    /** A label further above its box than this is not "directly above" it. */
    const DIRECTLY_ABOVE = 60;

    const closed = Array.from(root.querySelectorAll('details:not([open])')) as HTMLDetailsElement[];
    for (const details of closed) details.open = true;
    // The layout has settled after two frames; a menu that fills itself when it opens gets a moment to do so.
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
    if (closed.length > 0) await new Promise((resolve) => setTimeout(resolve, 30));

    try {
        const round = (value: number): number => Math.round(value * 10) / 10;

        interface IBox {
            el: Element;
            name: string;
            form: Element | null;
            left: number;
            right: number;
            top: number;
            bottom: number;
            labelText: string;
            labelLines: number;
            gap: number | null;
        }

        /**
         * The text of an element and the rectangles it is drawn in (one or more per line). The text of a control inside it (the
         * options of a select, the content of a text area) is left out: a label may wrap its box.
         */
        const textOf = (element: Element): { text: string; rects: DOMRect[] } => {
            const rects: DOMRect[] = [];
            let text = '';
            const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
            for (let node = walker.nextNode(); node; node = walker.nextNode()) {
                if (!(node.textContent ?? '').trim()) continue;
                if (node.parentElement?.closest('select, textarea, option, button, [role="tooltip"]')) continue;
                text += ` ${node.textContent ?? ''}`;
                const range = document.createRange();
                range.selectNodeContents(node);
                for (const rect of Array.from(range.getClientRects()))
                    if (rect.width > 0 && rect.height > 0) rects.push(rect);
            }
            return { text: text.replace(/\s+/g, ' ').trim(), rects };
        };

        const linesOf = (rects: DOMRect[]): number => {
            const tops = rects.map((rect) => rect.top).sort((a, b) => a - b);
            let lines = 0;
            let lineTop = -Infinity;
            for (const top of tops) {
                if (top - lineTop > 6) {
                    lines++;
                    lineTop = top;
                }
            }
            return lines;
        };

        const boxes: IBox[] = [];
        for (const el of Array.from(root.querySelectorAll('input, select, textarea'))) {
            if (
                el instanceof HTMLInputElement &&
                [
                    'hidden',
                    'checkbox',
                    'radio',
                    'file',
                    'submit',
                    'button',
                    'reset',
                    'image',
                    'range',
                    'color'
                ].includes(el.type)
            )
                continue;
            // A pop-up window is not the page; its forms are looked at on their own.
            if (el.closest('dialog, [role="dialog"]')) continue;
            if (!el.checkVisibility({ checkVisibilityCSS: true })) continue;
            const rect = el.getBoundingClientRect();
            if (rect.width < 2 || rect.height < 2) continue;

            let labelText = '';
            let labelLines = 0;
            let gap: number | null = null;
            for (const label of Array.from((el as HTMLInputElement).labels ?? [])) {
                const { text, rects } = textOf(label);
                if (rects.length === 0) continue;
                labelText = text;
                labelLines = linesOf(rects);
                // The label's block: the label, or, when other things share its row (a badge, a note), the branch of the page
                // that holds the label and not the box. The gap is measured from the lowest text of that block.
                let block: Element = label;
                if (!label.contains(el)) {
                    while (block.parentElement && !block.parentElement.contains(el)) block = block.parentElement;
                }
                const blockBox = block.getBoundingClientRect();
                const sideways = Math.min(blockBox.right, rect.right) - Math.max(blockBox.left, rect.left);
                // Directly above: the text that ends above the box (a label may also wrap the box, its text standing over it),
                // over the box, and close to it.
                const above = textOf(block).rects.filter((r) => r.bottom <= rect.top + 1);
                if (above.length > 0 && sideways > 0) {
                    const textBottom = Math.max(...above.map((r) => r.bottom));
                    if (rect.top - textBottom <= DIRECTLY_ABOVE) gap = rect.top - textBottom;
                }
                break;
            }
            boxes.push({
                el,
                name: el.getAttribute('data-test') ?? (el.id || el.getAttribute('name') || el.tagName.toLowerCase()),
                form: el.closest('form'),
                left: rect.left,
                right: rect.right,
                top: rect.top,
                bottom: rect.bottom,
                labelText,
                labelLines,
                gap
            });
        }

        const problems: string[] = [];
        const shortLabel = (box: IBox): string =>
            box.labelText.length > 40 ? `${box.labelText.slice(0, 40)}…` : box.labelText;
        const wrapNote = (a: IBox, b: IBox): string => {
            const wrapped = [a, b].filter((box) => box.labelLines > 1);
            return wrapped.length === 0
                ? ''
                : `; ${wrapped.map((box) => `the label «${shortLabel(box)}» of ${box.name} runs over ${box.labelLines} lines`).join(', ')}`;
        };

        for (let i = 0; i < boxes.length; i++) {
            for (let j = i + 1; j < boxes.length; j++) {
                const a = boxes[i];
                const b = boxes[j];
                const overlapX = Math.min(a.right, b.right) - Math.max(a.left, b.left);
                const overlapY = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
                if (overlapX > OVERLAP_TOLERANCE && overlapY > OVERLAP_TOLERANCE) {
                    problems.push(`overlap | ${a.name} and ${b.name} overlap`);
                    continue;
                }
                if (a.form !== b.form) continue;
                // Side by side: not over each other, and their heights share some of the row (a box pushed 28 px down by a
                // taller label still shares 16 of its 44 px with its neighbour).
                if (
                    overlapX <= OVERLAP_TOLERANCE &&
                    overlapY > SHARED_HEIGHT &&
                    Math.abs(a.top - b.top) > LEVEL_TOLERANCE
                ) {
                    problems.push(
                        `level | ${a.name} starts at ${round(a.top)} px but ${b.name}, beside it, at ${round(b.top)} px (${round(Math.abs(a.top - b.top))} px apart${wrapNote(a, b)})`
                    );
                }
            }
        }
        for (const box of boxes) {
            if (box.gap !== null && box.gap > maxGap) {
                problems.push(
                    `gap | ${box.name}: ${round(box.gap)} px from the text of its label «${shortLabel(box)}» to the box, the most is ${maxGap} px`
                );
            }
        }
        return { problems, boxes: boxes.length };
    } finally {
        for (const details of closed) details.open = false;
    }
}

/** Measures the page that is open, at the width it has: level, gap and overlap of its boxes. `scope` limits it to one element. */
export async function measureLevel(page: Page, scope?: string): Promise<ILevelResult> {
    return page.evaluate(measureInPage, { scope: scope ?? null, maxGap: MAX_LABEL_GAP });
}

/** The problems of the page that is open at the width it has, one line each (`rule | detail`). */
export async function levelProblems(page: Page, scope?: string): Promise<string[]> {
    return (await measureLevel(page, scope)).problems;
}

/**
 * The page that is open, measured at each of `widths` (the page's height stays), one line per problem: `640 px | rule | detail`.
 * The page gets the size it had back.
 */
export async function measureLevelAtWidths(
    page: Page,
    widths: readonly number[] = LEVEL_WIDTHS,
    scope?: string
): Promise<ILevelResult> {
    const before = page.viewportSize();
    const height = before?.height ?? 900;
    const problems: string[] = [];
    let boxes = 0;
    try {
        for (const width of widths) {
            await page.setViewportSize({ width, height });
            const result = await measureLevel(page, scope);
            boxes = Math.max(boxes, result.boxes);
            for (const problem of result.problems) problems.push(`${width} px | ${problem}`);
        }
    } finally {
        if (before) await page.setViewportSize(before);
    }
    return { problems, boxes };
}

export async function levelProblemsAtWidths(
    page: Page,
    widths: readonly number[] = LEVEL_WIDTHS,
    scope?: string
): Promise<string[]> {
    return (await measureLevelAtWidths(page, widths, scope)).problems;
}

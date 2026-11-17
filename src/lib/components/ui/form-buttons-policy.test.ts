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
 * Enforces "Forms" — the button row: a form's buttons are never a cell among its fields. In the maintainer's other repository a filter form had been a
 * four-column grid whose ninth cell was the button, so the button landed wherever the count of fields put it. The buttons are `FormButtons` (`src/lib/components/ui/FormButtons.svelte`),
 * a row of their own under all the fields that spans every column.
 *
 * It fails, in every `.svelte` file under `src/` (the test harnesses excepted): a direct cell of a grid inside a form — the
 * form itself laid out as a grid, or a grid box inside it — that holds a submit button and is not `FormButtons`. The blocks
 * (`{#if}`, `{#each}`) are see-through: a button inside an `{#if}` that sits in the grid is still a cell of the grid.
 * EXCEPTIONS lists the rest with their reasons; an entry that matches nothing fails.
 */

const EXCEPTIONS: Readonly<Record<string, string>> = {};

const isForm = (node: IMarkupNode) => (node.kind === 'element' && node.name === 'form') || node.name === 'GuardedForm';
const isGrid = (node: IMarkupNode) => classWords(node).includes('grid');

/** A submit button: `Button type="submit"`, a `<button type="submit">`, or a `<button>` with no type (the browser's default). */
function isSubmit(node: IMarkupNode): boolean {
    if (node.name === 'Button') return node.attrs.type === 'submit';
    return node.kind === 'element' && node.name === 'button' && (node.attrs.type ?? 'submit') === 'submit';
}

const holdsSubmit = (node: IMarkupNode): boolean => isSubmit(node) || node.children.some(holdsSubmit);

/** `<file>:<line>` of every grid cell of a form that holds a submit button outside `FormButtons`. */
function buttonCells(file: string): string[] {
    const found: string[] = [];
    for (const { node: form } of walkMarkup(readMarkup(file))) {
        if (!isForm(form)) continue;
        const grids = [form, ...Array.from(walkMarkup(form.children), (entry) => entry.node)].filter(isGrid);
        for (const grid of grids) {
            for (const cell of grid.children) {
                if (cell.name !== 'FormButtons' && holdsSubmit(cell)) found.push(`${file}:${cell.line}`);
            }
        }
    }
    return found;
}

const FILES = svelteFilesUnder('src').filter((file) => !file.startsWith('src/test/'));

// Every page of the interface is parsed: under a whole run's load that takes longer than a single test's 5 seconds.
describe('form buttons policy ("Forms": the buttons are a row of their own)', { timeout: 60_000 }, () => {
    it('draws no submit button as a cell of a grid form, but for the named exceptions', () => {
        const offending = FILES.filter((file) => !(file in EXCEPTIONS)).flatMap(buttonCells);
        expect(offending).toEqual([]);
    });

    it('keeps no exception that matches nothing', () => {
        expect(Object.keys(EXCEPTIONS).filter((file) => buttonCells(file).length === 0)).toEqual([]);
    });

    it('finds the defect it guards against', () => {
        const tree = `<GuardedForm class="grid sm:grid-cols-2"><Field /><div class="flex items-end">{#if a}<Button type="submit" />{/if}</div></GuardedForm>`;
        const [form] = markupTree(tree);
        const cells = form.children.filter((cell) => cell.name !== 'FormButtons' && holdsSubmit(cell));
        expect(cells.map((cell) => cell.name)).toEqual(['div']);
    });
});

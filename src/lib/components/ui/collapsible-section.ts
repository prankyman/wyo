/**
 * The browser-side helpers of `CollapsibleSection.svelte`.
 */

/**
 * Whether the section with this id is already in the page and open while its component starts: the page was drawn on the
 * server and the person opened the section before the page's script took over. The start-up then keeps it open instead of writing the arrival state over it.
 */
export function openedBeforeStart(id: string, root: Document = document): boolean {
    const element = root.getElementById(id);
    return element instanceof HTMLDetailsElement && element.open;
}

let focusQueued = false;

/**
 * Scrolls to and focuses the first refused field, once for every section that opened for a refusal in the same
 * moment: each of them asks, the first box in the page's order is the one shown. `scope` limits the search to one section
 *: a section answering its own save puts the cursor in ITS refused box, never in a refused box elsewhere in the page.
 * The control is looked up inside the refused box first — by the id its label names, then as the box's first control — so an
 * element with the same id elsewhere in the page cannot take the cursor.
 */
export function focusFirstRefusedField(root: Document = document, scope: ParentNode = root): void {
    if (focusQueued) return;
    focusQueued = true;
    queueMicrotask(() => {
        focusQueued = false;
        const field = scope.querySelector('[data-field-error]');
        if (!field) return;
        // The box the label names, else the first box of the field — never the «?» beside the label, which is a button too.
        const labelled = field.querySelector<HTMLLabelElement>('label[for]')?.htmlFor;
        const control =
            (labelled ? [...field.querySelectorAll<HTMLElement>('[id]')].find((box) => box.id === labelled) : null) ??
            (labelled ? root.getElementById(labelled) : null) ??
            field.querySelector<HTMLElement>('input:not([type="hidden"]), select, textarea');
        if (!control || field.contains(root.activeElement)) return;
        control.scrollIntoView?.({ block: 'center' });
        control.focus({ preventScroll: true });
    });
}

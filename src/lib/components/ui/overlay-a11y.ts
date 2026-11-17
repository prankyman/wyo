/**
 * The accessibility machinery every modal overlay in this interface needs.
 *
 * Extracted from `SlideOverPanel.svelte`, which had the only complete implementation —
 * the other overlays each had partial ones, and the gaps differed per
 * component (no focus trap here, an Escape handler bound to a
 * non-focusable div there). A second overlay with a different shape made that divergence worth closing rather than extending.
 *
 * Geometry stays with the component; only behaviour lives here.
 */

/**
 * A hidden input (a form's id) is never focusable, so it is not a focus-trap stop:
 * counted as the first or last one, Tab from the real last control would wrap to nothing.
 */
const FOCUSABLE_SELECTOR =
    'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

/** Every focusable descendant of `root`, in DOM order. */
export function focusableElements(root: HTMLElement | undefined | null): HTMLElement[] {
    if (!root) return [];
    return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
}

/**
 * Marks everything outside the overlay `inert`, so assistive technology and Tab cannot
 * reach the page behind it.
 *
 * `inert` rather than `aria-hidden`: the latter hides content from screen readers while
 * leaving it in the tab order, which produces the worst of both worlds — a keyboard user
 * tabbing into content their screen reader will not announce.
 */
export function setBackgroundInert(root: HTMLElement | undefined | null, isInert: boolean): void {
    if (typeof document === 'undefined') return;

    for (const child of Array.from(document.body.children)) {
        if (isInert && (child === root || (root && child.contains(root)))) continue;
        if ('inert' in child) {
            (child as HTMLElement & { inert: boolean }).inert = isInert;
        }
    }
}

/**
 * Cycles Tab within the overlay. Returns `true` when it handled the event.
 *
 * Manual rather than relying on `inert` alone, because `inert` on siblings still lets focus
 * escape to the browser chrome and back into the document at the far end.
 */
export function trapTabKey(e: KeyboardEvent, root: HTMLElement | undefined | null): boolean {
    if (e.key !== 'Tab') return false;

    const elements = focusableElements(root);
    if (elements.length === 0) return false;

    const first = elements[0];
    const last = elements[elements.length - 1];

    if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
        return true;
    }

    if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
        return true;
    }

    return false;
}

/** True when the click landed on the backdrop itself rather than on the panel inside it. */
export function isBackdropClick(e: MouseEvent): boolean {
    return e.target === e.currentTarget;
}

/**
 * Locks body scroll and returns the function that restores it.
 *
 * Returns a restorer rather than taking a boolean so the previous value is captured at lock
 * time — two overlays opening in sequence must not leave `overflow: hidden` behind.
 */
export function lockBodyScroll(): () => void {
    if (typeof document === 'undefined') return () => {};

    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    return () => {
        document.body.style.overflow = previous;
    };
}

interface IDismissableOptions {
    /** Called when the user asks for the panel to go away. */
    onDismiss: () => void;
    /**
     * The control that opens the panel. Clicks on it are ignored: it already toggles the
     * panel itself, so dismissing here as well would fight it.
     */
    trigger?: () => HTMLElement | null | undefined;
}

/**
 * Closes an open dropdown panel on Escape or on a click outside it, and returns the
 * teardown for both listeners.
 *
 * Written as a plain function driven from an `$effect` rather than a Svelte attachment:
 * this project's `svelte-eslint-parser` cannot parse `{@attach}` yet, which is why no
 * attachment appears anywhere in `src/lib`.
 *
 * Escape returns focus to the trigger, so keyboard users are not dropped at the top of the
 * document (WCAG 2.1, constitution, "Accessibility").
 */
export function dismissable(node: HTMLElement, { onDismiss, trigger }: IDismissableOptions): () => void {
    function isInside(target: EventTarget | null): boolean {
        if (!(target instanceof Node)) return false;
        return node.contains(target) || (trigger?.()?.contains(target) ?? false);
    }

    function handleKeydown(event: KeyboardEvent): void {
        if (event.key !== 'Escape') return;
        trigger?.()?.focus();
        onDismiss();
    }

    function handleClick(event: MouseEvent): void {
        if (isInside(event.target)) return;
        onDismiss();
    }

    document.addEventListener('keydown', handleKeydown);
    document.addEventListener('click', handleClick);

    return () => {
        document.removeEventListener('keydown', handleKeydown);
        document.removeEventListener('click', handleClick);
    };
}

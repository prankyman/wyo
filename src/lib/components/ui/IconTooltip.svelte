<script lang="ts">
    import type { Snippet } from 'svelte';
    import Portal from './Portal.svelte';
    import { VIEWPORT_MARGIN, horizontalPlacement, verticalPlacement, type TTooltipSide } from './tooltip-position';

    /**
     * The tooltip of an icon: the icon's name and one sentence of what it does, shown while the mouse is on
     * the icon and while the keyboard's focus is on it, never on a tap (a tap acts at once). It is placed like the «?»
     * popups (`tooltip-position.ts`), on the side the caller prefers — below for the header's icons, where a tooltip
     * above would leave the screen. The icon keeps its own accessible name (`aria-label`); the tooltip only shows it.
     */
    interface IProps {
        /** The icon's name, the same words as its `aria-label`. */
        name: string;
        /** One sentence of what the icon does. */
        description?: string;
        side?: TTooltipSide;
        testId?: string;
        /**
         * Shown only below 768 px (Tailwind's `md`), where the header draws the icon; from 768 px up the control shows its
         * words and needs no tooltip.
         */
        onlyBelowMd?: boolean;
        /** Classes of the box around the icon. */
        class?: string;
        /** The icon: a link or a button with its own `aria-label`. */
        children: Snippet;
    }

    let {
        name,
        description,
        side = 'below',
        testId = 'icon-tooltip',
        onlyBelowMd = false,
        class: className = '',
        children
    }: IProps = $props();

    /** The width from which a header shows words instead of icons: Tailwind's `md`. */
    const MD_QUERY = '(min-width: 768px)';

    /** Whether the words are drawn now instead of the icon (a browser without `matchMedia` draws icons). */
    function wordsShown(): boolean {
        return onlyBelowMd && typeof window.matchMedia === 'function' && window.matchMedia(MD_QUERY).matches;
    }

    /** How long the tooltip waits after the mouse leaves, so the mouse can move onto it (WCAG 1.4.13, «hoverable»). */
    const HIDE_DELAY_MS = 150;

    let box = $state<HTMLSpanElement | null>(null);
    let tip = $state<HTMLDivElement | null>(null);
    let shown = $state(false);
    let style = $state('');
    let hideTimer: ReturnType<typeof setTimeout> | undefined;

    function place(): void {
        if (!box) return;
        const trigger = box.getBoundingClientRect();
        const popup = tip?.getBoundingClientRect();
        style =
            `${verticalPlacement(trigger, popup?.height ?? 0, side)} ${horizontalPlacement(trigger, popup?.width ?? 0)}` +
            ` max-width: min(320px, calc(100vw - ${VIEWPORT_MARGIN * 2}px));`;
    }

    function show(): void {
        if (wordsShown()) return;
        clearTimeout(hideTimer);
        shown = true;
        place();
    }

    function hideSoon(): void {
        clearTimeout(hideTimer);
        hideTimer = setTimeout(() => (shown = false), HIDE_DELAY_MS);
    }

    /** A focus from the keyboard shows it; the focus a tap leaves behind does not. */
    function showForKeyboard(event: FocusEvent): void {
        const target = event.target;
        if (target instanceof Element && target.matches(':focus-visible')) show();
    }

    function hideOnEscape(event: KeyboardEvent): void {
        if (event.key === 'Escape') shown = false;
    }

    $effect(() => {
        if (!shown || !tip) return;
        place();
        if (typeof requestAnimationFrame !== 'function') return;
        const frame = requestAnimationFrame(place);
        return () => cancelAnimationFrame(frame);
    });

    $effect(() => () => clearTimeout(hideTimer));
</script>

<svelte:window onkeydown={shown ? hideOnEscape : undefined} />

<!-- The tooltip opens on hover and on keyboard focus only, so the box takes no press of its own: the icon inside does. -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<span
    bind:this={box}
    class="inline-flex {className}"
    onmouseenter={show}
    onmouseleave={hideSoon}
    onfocusin={showForKeyboard}
    onfocusout={() => (shown = false)}
    data-test={testId}
>
    {@render children()}
</span>

{#if shown}
    <Portal>
        <div
            bind:this={tip}
            role="tooltip"
            class="fixed z-10000 w-max rounded bg-ink-900 px-3 py-2 text-xs text-white shadow-lg"
            {style}
            onmouseenter={show}
            onmouseleave={hideSoon}
            data-test="{testId}-tooltip"
        >
            <p class="font-medium">{name}</p>
            {#if description}<p class="mt-1 text-ink-200">{description}</p>{/if}
        </div>
    </Portal>
{/if}

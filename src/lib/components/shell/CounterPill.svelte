<script lang="ts">
    import { dismissable } from '$lib/components/ui/dismissable';
    import Portal from '$lib/components/ui/Portal.svelte';
    import { FOCUS_RING, PRESS_FEEDBACK } from '$lib/components/ui/press';
    import { VIEWPORT_MARGIN, horizontalPlacement, verticalPlacement } from '$lib/components/ui/tooltip-position';

    /**
     * A menu counter: the number on an item, which explains itself. It is a `<button>` that stands BESIDE the menu
     * link, never inside it (a button inside a link is invalid HTML); its sentence is its accessible name and the text of its
     * pop-up. The pop-up opens while the mouse is on the counter and while the keyboard's focus is on it, and on a phone with a
     * tap, closed by a tap elsewhere, by a second tap or by Escape (`dismissable`). A tap on the counter opens only the
     * pop-up: it does not follow the link under it and does not open or close the group whose heading it sits in.
     */
    interface IProps {
        /** The number drawn on the counter. */
        count: number;
        /** The sentences the counter says: one for an item, one per item with a number for a closed group's heading. */
        sentences: readonly string[];
        testId: string;
        /**
         * The phone's bottom strip: a 24 × 24 px tap area under the "Equivalent" exception of the constitution's tap-target rule
         * (the same sentence is a line of the bell, a 44 px control on the same screen), marked `data-counter` for the phone sweep.
         * Otherwise the tap area is 44 × 44 px, at the end of a row 44 px tall.
         */
        compact?: boolean;
        /** Where the counter stands: the caller places it at the end of its row. */
        class?: string;
    }

    let { count, sentences, testId, compact = false, class: className = '' }: IProps = $props();

    /** How long the pop-up waits after the mouse leaves, so the mouse can move onto it (WCAG 1.4.13, «hoverable»). */
    const HIDE_DELAY_MS = 150;
    const POPUP_MAX_WIDTH = 320;

    const name = $derived(sentences.join('; '));

    let button = $state<HTMLButtonElement | null>(null);
    let popup = $state<HTMLDivElement | null>(null);
    let shown = $state(false);
    let style = $state('');
    let hideTimer: ReturnType<typeof setTimeout> | undefined;
    /** What touched the counter last (`mouse`, `touch`, `pen`): a touch and a pen toggle the pop-up, the mouse and the keyboard only open it. */
    let lastPointer = '';

    function place(): void {
        if (!button) return;
        const trigger = button.getBoundingClientRect();
        const box = popup?.getBoundingClientRect();
        style =
            `${verticalPlacement(trigger, box?.height ?? 0, 'below')} ${horizontalPlacement(trigger, box?.width ?? 0)}` +
            ` max-width: min(${POPUP_MAX_WIDTH}px, calc(100vw - ${VIEWPORT_MARGIN * 2}px));`;
    }

    function show(): void {
        clearTimeout(hideTimer);
        shown = true;
        place();
    }

    function hideSoon(): void {
        clearTimeout(hideTimer);
        hideTimer = setTimeout(() => (shown = false), HIDE_DELAY_MS);
    }

    function onPointerEnter(event: PointerEvent): void {
        if (event.pointerType === 'mouse') show();
    }

    function onPointerLeave(event: PointerEvent): void {
        if (event.pointerType === 'mouse') hideSoon();
    }

    /** A focus from the keyboard shows it; the focus a tap leaves behind does not. */
    function onFocusIn(event: FocusEvent): void {
        const target = event.target;
        try {
            if (target instanceof Element && target.matches(':focus-visible')) show();
        } catch {
            // A browser that does not know `:focus-visible` shows the pop-up on hover and tap only.
        }
    }

    function onClick(event: MouseEvent): void {
        // The tap belongs to the counter alone: not to the heading's group and not to the link under it.
        event.preventDefault();
        event.stopPropagation();
        if (lastPointer === 'touch' || lastPointer === 'pen') {
            if (shown) shown = false;
            else show();
        } else {
            show();
        }
    }

    $effect(() => {
        if (!shown || !popup) return;
        return dismissable(popup, { onDismiss: () => (shown = false), trigger: () => button });
    });

    $effect(() => {
        if (!shown || !popup) return;
        place();
        const onMove = () => place();
        window.addEventListener('scroll', onMove, true);
        window.addEventListener('resize', onMove);
        const frame = typeof requestAnimationFrame === 'function' ? requestAnimationFrame(place) : undefined;
        return () => {
            window.removeEventListener('scroll', onMove, true);
            window.removeEventListener('resize', onMove);
            if (frame !== undefined) cancelAnimationFrame(frame);
        };
    });

    $effect(() => () => clearTimeout(hideTimer));
</script>

<!--
    The button is as large as its tap area (44 × 44 px, or 24 × 24 px on the phone's strip) and draws no background of its own:
    the red number inside it is the counter. The sentence is the accessible name, so nothing else names the number.
-->
<button
    bind:this={button}
    type="button"
    data-test={testId}
    data-counter={compact ? '' : undefined}
    aria-label={name}
    class="flex cursor-pointer items-center justify-center rounded-full border-0 bg-transparent {compact
        ? 'size-6'
        : 'size-11'} {PRESS_FEEDBACK} {FOCUS_RING} {className}"
    onpointerenter={onPointerEnter}
    onpointerleave={onPointerLeave}
    onpointerdown={(event) => (lastPointer = event.pointerType)}
    onkeydown={() => (lastPointer = '')}
    onfocusin={onFocusIn}
    onfocusout={() => (shown = false)}
    onclick={onClick}
>
    <span
        aria-hidden="true"
        class="rounded-full bg-stop-600 text-center font-semibold text-white {compact
            ? 'min-w-4 px-1 text-[0.625rem] leading-4'
            : 'min-w-5 px-1.5 text-xs leading-5'}">{count}</span
    >
</button>

{#if shown}
    <Portal>
        <div
            bind:this={popup}
            role="tooltip"
            class="fixed z-10000 w-max rounded bg-ink-900 px-3 py-2 text-xs text-white shadow-lg"
            {style}
            onmouseenter={show}
            onmouseleave={hideSoon}
            data-test="{testId}-tooltip"
        >
            {#each sentences as sentence, index (index)}
                <p class={index > 0 ? 'mt-1' : ''}>{sentence}</p>
            {/each}
        </div>
    </Portal>
{/if}

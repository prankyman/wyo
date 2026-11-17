<script lang="ts">
    import Button from '$lib/components/ui/Button.svelte';
    import ModalWindow from '$lib/components/ui/ModalWindow.svelte';
    import { getLocale, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import X from 'phosphor-svelte/lib/X';
    import type { Snippet } from 'svelte';
    import { fly } from 'svelte/transition';
    import { createReducedMotion } from '$lib/components/ui/reduced-motion.svelte';
    import { sheetDragOutcome } from '$lib/components/ui/sheet-drag';

    interface IProps {
        open: boolean;
        title: string;
        lang?: TLocale;
        onClose: () => void;
        dataTest?: string;
        /**
         * `right` (default) — a right-hand panel at every viewport, unchanged behaviour.
         * `responsive` — a bottom sheet below `md`, a right-hand panel from `md` up.
         */
        placement?: 'right' | 'responsive';
        widthClass?: string;
        /**
         * Default true: the backdrop, the inert page and the scroll lock of `ModalWindow`. False:
         * from `md` up the page stays usable beside the panel and a click outside does not close it; Escape still does.
         * Below `md` the bottom sheet stays modal.
         */
        modal?: boolean;
        /** Drawn right after the title, e.g. the panel's one «?». */
        heading?: Snippet;
        children: Snippet;
    }

    let {
        open,
        title,
        lang = getLocale(),
        onClose,
        dataTest = 'slide-over-panel',
        placement = 'right',
        widthClass = 'md:w-[480px]',
        modal = true,
        heading,
        children
    }: IProps = $props();

    /** Tailwind's `md` breakpoint. The panel slides up from the bottom below it, in from the right above. */
    const MD_BREAKPOINT = 768;
    const SLIDE_DURATION_MS = 260;

    let innerWidth = $state(0);
    const reducedMotion = createReducedMotion();

    const isResponsive = $derived(placement === 'responsive');
    const isBottomSheet = $derived(isResponsive && innerWidth > 0 && innerWidth < MD_BREAKPOINT);

    /** Not modal only on a computer: a phone's bottom sheet always keeps the backdrop. */
    const isModal = $derived(modal || !isResponsive || innerWidth === 0 || isBottomSheet);

    /** Where the panel sits in the window's root (`ModalWindow`, `frame="none"`): the right edge, or the bottom edge below `md`. */
    const rootClass = $derived(isResponsive ? 'items-end md:items-stretch md:justify-end' : 'justify-end');

    const panelClass = $derived(
        isResponsive
            ? `pointer-events-auto relative w-full ${widthClass} bg-white flex flex-col shadow-2xl rounded-t-2xl md:rounded-none max-h-[88dvh] md:max-h-none md:h-[100dvh]`
            : `pointer-events-auto relative w-full ${widthClass} bg-white flex flex-col shadow-2xl`
    );

    const bodyClass = $derived(
        isResponsive
            ? 'flex-1 min-h-0 overflow-y-auto overscroll-contain pb-[env(safe-area-inset-bottom)]'
            : 'flex-1 min-h-0 overflow-y-auto'
    );

    // A `right` panel keeps its original instant appearance (duration 0 makes `fly` a no-op);
    // only the responsive placement actually slides, and reduced motion disables that too.
    const flyParams = $derived({
        x: isBottomSheet ? 0 : 560,
        y: isBottomSheet ? 420 : 0,
        duration: !isResponsive || reducedMotion.current ? 0 : SLIDE_DURATION_MS
    });

    /** How far the sheet is pulled down by the grab handle, in px; 0 when it is not being dragged. */
    let dragOffset = $state(0);
    /** Where the finger touched the handle; null while no finger is on it. */
    let dragStart = $state<{ y: number; time: number; pointerId: number } | null>(null);
    let panel = $state<HTMLDivElement | undefined>();

    // The grab handle of the bottom sheet (the handle works as a handle does): the sheet follows the finger down, and on
    // the lift it closes or springs back (`sheetDragOutcome`); a tap closes it too.
    function onHandlePointerDown(e: PointerEvent) {
        dragStart = { y: e.clientY, time: e.timeStamp, pointerId: e.pointerId };
        (e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId);
    }

    function onHandlePointerMove(e: PointerEvent) {
        if (dragStart?.pointerId !== e.pointerId) return;
        dragOffset = Math.max(0, e.clientY - dragStart.y);
    }

    function onHandlePointerUp(e: PointerEvent) {
        if (dragStart?.pointerId !== e.pointerId) return;
        const outcome = sheetDragOutcome({
            distance: Math.max(0, e.clientY - dragStart.y),
            durationMs: e.timeStamp - dragStart.time,
            sheetHeight: panel?.offsetHeight ?? 0
        });
        dragStart = null;
        if (outcome === 'close') onClose();
        else dragOffset = 0;
    }

    function onHandlePointerCancel() {
        dragStart = null;
        dragOffset = 0;
    }

    // A sheet opened again starts where it belongs, not where the last drag left it.
    $effect(() => {
        if (open) dragOffset = 0;
    });
</script>

<svelte:window bind:innerWidth />

<!--
    The side panel on `ModalWindow`: the window owns the
    backdrop, the focus trap, the inert page, the scroll lock, Escape and the backdrop click; the panel that slides and is
    dragged is this component's own, so it is the window's child with `frame="none"`.
-->
<ModalWindow
    {open}
    ariaLabel={title}
    frame="none"
    backdrop="light"
    layer="bar"
    closeOnBackdrop
    modal={isModal}
    {onClose}
    dataTest="{dataTest}-backdrop"
    class={rootClass}
>
    <div
        bind:this={panel}
        class={panelClass}
        style:height={isResponsive ? undefined : '100dvh'}
        style:transform={dragOffset > 0 ? `translateY(${dragOffset}px)` : undefined}
        style:transition={dragStart || reducedMotion.current ? undefined : 'transform 200ms ease-out'}
        transition:fly={flyParams}
        data-test="{dataTest}-panel"
    >
        {#if isResponsive}
            <!-- 48 px tall to touch (Material's minimum); hidden from screen readers, which close the sheet with ✕. -->
            <div
                class="md:hidden flex h-12 shrink-0 cursor-grab touch-none items-center justify-center select-none"
                aria-hidden="true"
                data-test="{dataTest}-grab-handle"
                onpointerdown={onHandlePointerDown}
                onpointermove={onHandlePointerMove}
                onpointerup={onHandlePointerUp}
                onpointercancel={onHandlePointerCancel}
            >
                <span class="h-1 w-10 rounded-full bg-ink-300"></span>
            </div>
        {/if}

        <div class="flex items-center justify-between px-4 py-3 border-b border-ink-200 shrink-0">
            <div class="flex items-center gap-1">
                <h2 class="font-display text-base text-ink-900">{title}</h2>
                {@render heading?.()}
            </div>
            <Button
                variant="ghost"
                onclick={onClose}
                ariaLabel={t('common.close', lang)}
                class="-mr-2 text-ink-400"
                dataTest="{dataTest}-close"
            >
                {#snippet icon()}<X class="size-5" aria-hidden="true" />{/snippet}
            </Button>
        </div>

        <div class={bodyClass} data-test="{dataTest}-body">
            {@render children()}
        </div>
    </div>
</ModalWindow>

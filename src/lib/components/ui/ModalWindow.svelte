<script lang="ts">
    import XIcon from 'phosphor-svelte/lib/XIcon';
    import type { Snippet } from 'svelte';
    import { tick } from 'svelte';
    import { cn } from '$lib/utils/cn';
    import Button from './Button.svelte';
    import { focusableElements, isBackdropClick, lockBodyScroll, setBackgroundInert, trapTabKey } from './overlay-a11y';
    import Portal from './Portal.svelte';

    /**
     * The one window shell of the wyo interface. Every
     * pop-up window — a confirmation, a dialog that asks a reason, a bottom sheet, a camera sheet, the side panel — is this component with its own content inside; the glue the rule demands is written here
     * once and nowhere else (`modal-window-policy.test.ts` keeps it so):
     *
     * - rendered inside `Portal`, so `setBackgroundInert` finds the page as siblings and marks them `inert`;
     * - the focus moves onto the first control, Tab and Shift+Tab cycle inside (`trapTabKey`), the body does not scroll;
     * - everything is restored in the `$effect`'s teardown — on close AND when the window is destroyed while open, which
     *   is what a route change does — never in an `else` branch;
     * - Escape closes it unless `closeOnEscape` is false or `canClose()` says no (a press whose answer is still pending);
     *   the backdrop closes it only when `closeOnBackdrop` is set (the side panel), under the same `canClose()`.
     *
     * The look is decided here once: a white card of one of four widths, centred (`placement="center"`) or a bottom sheet
     * on a phone that becomes the card from `sm:` up (`placement="sheet"`); `size="full"` is the one full-screen window,
     * black, for the camera picture. A window that brings its own panel — `SlideOverPanel`, which slides and is dragged —
     * passes `frame="none"`: the root with its backdrop and glue is drawn, the children are the panel, and `class`
     * places them.
     */
    interface IProps {
        /** Bindable: the window is on screen. It sets itself false when it closes (Escape, the backdrop, the ✕). */
        open?: boolean;
        /** The labelled heading of the window, already worded. */
        title?: string;
        /** Drawn right after the title in the heading row: the window's one «?». */
        heading?: Snippet;
        /** The `id` of the heading; generated when left out. */
        titleId?: string;
        /** An accessible name instead of the labelled heading (a window of `frame="none"` that draws its own title). */
        ariaLabel?: string;
        /** `data-test` of the root — the `role="dialog"` element the focus is trapped in. */
        dataTest?: string;
        /** `data-test` of the card itself, when a test or a spec addresses it apart from the root. */
        panelTestId?: string;
        /** A guided tour's anchor (`data-tour`) on the card. */
        tour?: string;
        /** The card's greatest width: `md` 28 rem (the default), `lg` 32, `xl` 36, `2xl` 42; `full` is the whole screen. */
        size?: 'md' | 'lg' | 'xl' | '2xl' | 'full';
        /** `center`: the card in the middle of the screen; `sheet`: a bottom sheet on a phone, the card from `sm:` up. */
        placement?: 'center' | 'sheet';
        /** `card` (the default) draws the panel, heading and footer; `none` draws only the root, the children are the panel. */
        frame?: 'card' | 'none';
        /** `dark`: the half-black backdrop of a dialog (the default); `light`: the light one of the side panel. */
        backdrop?: 'dark' | 'light';
        /**
         * Which layer the window sits on: `top` (z-50, the default), `base` (z-40, under the toasts: a dialog opened
         * from a card that may answer with a toast), `bar` (z-60, over the pinned top bar: the side panel), `screen`
         * (z-70, the full-screen camera over everything).
         */
        layer?: 'base' | 'top' | 'bar' | 'screen';
        /** Escape closes the window. Default true; a gate with no way past it passes false. */
        closeOnEscape?: boolean;
        /** A click on the backdrop closes the window. Default false. */
        closeOnBackdrop?: boolean;
        /** Asked before Escape, the backdrop and the ✕ close the window: false keeps it open (an answer still pending). */
        canClose?: () => boolean;
        /** Called when the window closes itself — Escape, the backdrop or the ✕ — before `open` becomes false. */
        onClose?: () => void;
        /**
         * The opener gets the focus back when the window goes away. Default true; `PlatformCountryGate` passes false:
         * its answer re-reads the page and the window never comes back, so there is no opener to return to.
         */
        restoreFocus?: boolean;
        /**
         * Default true. False (a side panel on a computer): no backdrop, the page behind stays
         * usable and scrollable, Tab is not trapped, a click outside does not close it. Escape still closes it, and the
         * focus still moves in on opening and back on closing. The window's children must take `pointer-events-auto`.
         */
        modal?: boolean;
        /** The accessible name of a ✕ button in the heading row; without it there is no ✕. */
        closeLabel?: string;
        /** `data-test` of the ✕; `<dataTest>-close` by default. */
        closeTestId?: string;
        /** Classes added to the root: the place of a `frame="none"` panel (`justify-end`). */
        class?: string;
        /** The content; left out when the title asks the question and the footer holds the answers. */
        children?: Snippet;
        /**
         * The band under the content of a card that stands still while the content scrolls: the window's buttons, always one
         * `WindowButtons` (`window-buttons-policy.test.ts`). A form in the content is sent by its button here through `form="<id>"`.
         */
        footer?: Snippet;
    }

    let {
        open = $bindable(false),
        title,
        heading,
        titleId: givenTitleId,
        ariaLabel,
        dataTest,
        panelTestId,
        tour,
        size = 'md',
        placement = 'center',
        frame = 'card',
        backdrop = 'dark',
        layer = 'top',
        closeOnEscape = true,
        closeOnBackdrop = false,
        modal = true,
        canClose = () => true,
        onClose,
        restoreFocus = true,
        closeLabel,
        closeTestId,
        class: className,
        children,
        footer
    }: IProps = $props();

    const uid = $props.id();
    const titleId = $derived(givenTitleId ?? `modal-window-title-${uid}`);

    const LAYER_CLASS: Record<NonNullable<IProps['layer']>, string> = {
        base: 'z-40',
        top: 'z-50',
        bar: 'z-[60]',
        screen: 'z-[70]'
    };
    const SIZE_CLASS: Record<NonNullable<IProps['size']>, string> = {
        md: 'max-w-md',
        lg: 'max-w-lg',
        xl: 'max-w-xl',
        '2xl': 'max-w-2xl',
        full: ''
    };

    const isFull = $derived(size === 'full');
    const isCard = $derived(frame === 'card' && !isFull);
    const hasHeadingRow = $derived(Boolean(title || heading || closeLabel));

    const rootClass = $derived(
        cn(
            'fixed inset-0 flex',
            LAYER_CLASS[layer],
            !modal ? 'pointer-events-none' : backdrop === 'light' ? 'bg-ink-900/20' : 'bg-black/50',
            isCard &&
                (placement === 'sheet' ? 'items-end justify-center sm:items-center' : 'items-center justify-center'),
            className
        )
    );
    const panelClass = $derived(
        isFull
            ? 'flex h-full w-full flex-col bg-black text-white'
            : cn(
                  'flex max-h-[90vh] w-full flex-col bg-white shadow-lg',
                  SIZE_CLASS[size],
                  placement === 'sheet' ? 'rounded-t sm:mx-4 sm:rounded' : 'mx-4 rounded'
              )
    );
    const headingRowClass = $derived(
        isFull
            ? 'flex shrink-0 items-center justify-between gap-3 px-4 pt-[max(0.75rem,env(safe-area-inset-top))] pb-3'
            : 'flex shrink-0 items-start justify-between gap-3 px-6 py-3.75'
    );
    const bodyClass = $derived(
        isFull
            ? 'flex min-h-0 flex-1 flex-col'
            : cn('min-h-0 flex-1 overflow-y-auto px-6', hasHeadingRow ? 'pt-1' : 'pt-6', footer ? 'pb-1' : 'pb-6')
    );

    let root = $state<HTMLDivElement | undefined>();

    function close(): void {
        onClose?.();
        open = false;
    }

    function requestClose(): void {
        if (canClose()) close();
    }

    function handleWindowKeydown(event: KeyboardEvent): void {
        if (!open) return;
        if (event.key === 'Escape') {
            if (!closeOnEscape || !canClose()) {
                event.preventDefault();
                return;
            }
            close();
            return;
        }
        if (modal) trapTabKey(event, root);
    }

    function handleBackdropClick(event: MouseEvent): void {
        if (modal && closeOnBackdrop && isBackdropClick(event)) requestClose();
    }

    // Teardown rather than an `else` branch, so the page gets its scrolling, its interactivity and the opener's focus
    // back when the window closes AND when it is destroyed while still open (a route change). Restoring must not depend
    // on `root`: by teardown time `{#if open}` has unmounted it; `setBackgroundInert` reads `root` only when marking.
    $effect(() => {
        if (!open) return;

        const previouslyFocused = document.activeElement as HTMLElement | null;
        const unlockScroll = modal ? lockBodyScroll() : () => {};
        if (modal) setBackgroundInert(root, true);
        void tick().then(() => focusableElements(root)[0]?.focus());

        return () => {
            unlockScroll();
            if (modal) setBackgroundInert(root, false);
            if (restoreFocus) previouslyFocused?.focus();
        };
    });
</script>

<svelte:window onkeydown={handleWindowKeydown} />

{#if open}
    <Portal>
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
            bind:this={root}
            class={rootClass}
            role="dialog"
            aria-modal={modal ? 'true' : 'false'}
            aria-labelledby={ariaLabel ? undefined : titleId}
            aria-label={ariaLabel}
            tabindex="-1"
            onclick={handleBackdropClick}
            data-test={dataTest}
        >
            {#if frame === 'none'}
                {@render children?.()}
            {:else}
                <div class={panelClass} data-test={panelTestId} data-tour={tour}>
                    {#if hasHeadingRow}
                        <div class={headingRowClass}>
                            <div class="flex min-w-0 items-center gap-1">
                                {#if title}
                                    <h2 id={titleId} class={isFull ? 'text-base' : 'font-display text-lg text-ink-900'}>
                                        {title}
                                    </h2>
                                {/if}
                                {@render heading?.()}
                            </div>
                            {#if closeLabel}
                                <Button
                                    variant={isFull ? 'ghost-inverse' : 'ghost'}
                                    onclick={requestClose}
                                    ariaLabel={closeLabel}
                                    class={isFull ? undefined : '-mt-2 -mr-2 text-ink-400'}
                                    dataTest={closeTestId ?? (dataTest ? `${dataTest}-close` : undefined)}
                                >
                                    {#snippet icon()}<XIcon
                                            class={isFull ? 'size-6' : 'size-5'}
                                            aria-hidden="true"
                                        />{/snippet}
                                </Button>
                            {/if}
                        </div>
                    {/if}
                    {#if children}
                        <div class={bodyClass}>
                            {@render children()}
                        </div>
                    {/if}
                    {#if footer && !isFull}
                        <div class="shrink-0 px-6 py-3.75">
                            {@render footer()}
                        </div>
                    {/if}
                </div>
            {/if}
        </div>
    </Portal>
{/if}

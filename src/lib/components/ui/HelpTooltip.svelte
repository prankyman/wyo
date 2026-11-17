<script lang="ts">
    import type { Snippet } from 'svelte';
    import QuestionIcon from 'phosphor-svelte/lib/QuestionIcon';
    import { dismissable } from './dismissable';
    import HelpLinkList from './HelpLinkList.svelte';
    import Portal from './Portal.svelte';
    import type { IHelpLink } from './types';
    import PopupCloseButton from './PopupCloseButton.svelte';
    import { FOCUS_RING, PRESS_FEEDBACK } from './press';
    import { VIEWPORT_MARGIN, horizontalPlacement, verticalPlacement } from './tooltip-position';

    interface IProps {
        tooltip: string;
        /** Links shown under the paragraphs, each opening in a new tab. */
        links?: IHelpLink[];
        ariaLabel: string;
        testId?: string;
        class?: string;
        tooltipClass?: string;
        iconSize?: string;
        position?: 'left' | 'center' | 'right';
        /**
         * Another glyph in place of the «?» (for example the green check of a size already added): the button opens and closes the
         * popup by a click or a tap just the same, and `class` gives the glyph its colour.
         */
        icon?: Snippet;
        /** No space before the glyph: the «?» stands in a box of its own that sets the distance (a tap box in a row of icons). */
        flush?: boolean;
    }

    let {
        tooltip,
        links = [],
        ariaLabel,
        testId = 'helptooltip',
        class: buttonClass = 'text-ink-400 pressable:hover:text-ink-600',
        tooltipClass = '',
        iconSize = '0.875rem',
        position = 'center',
        icon,
        flush = false
    }: IProps = $props();

    /**
     * The widest the popup gets on a wide screen. It is sized to its text, so a short hint stays compact
     * and a long one grows sideways before it grows tall — up to this width, or up to the screen minus
     * its margins when the screen is narrower, which keeps it on a 320 px phone (about 300 px there).
     */
    const TOOLTIP_MAX_WIDTH = 500;

    let triggerEl = $state<HTMLButtonElement | null>(null);
    let glyphEl = $state<HTMLSpanElement | null>(null);
    let tooltipEl = $state<HTMLDivElement | null>(null);
    let tooltipVisible = $state(false);
    let tooltipStyle = $state('');

    function updatePosition() {
        if (!triggerEl) return;
        const trigger = (glyphEl ?? triggerEl).getBoundingClientRect();
        // Zero until the popup is in the document: the first pass anchors it, the measuring pass
        // below corrects it before the browser paints.
        const popup = tooltipEl?.getBoundingClientRect();
        const width = popup?.width ?? 0;
        const height = popup?.height ?? 0;

        // The height cap is what keeps a long text readable: placement can hold the top edge on
        // screen, but a popup taller than the screen would still run off the bottom with nothing to
        // scroll. Capped, it measures at most the screen's height and scrolls inside.
        tooltipStyle =
            `${verticalPlacement(trigger, height)} ${horizontalPlacement(trigger, width, position)}` +
            ` max-width: min(${TOOLTIP_MAX_WIDTH}px, calc(100vw - ${VIEWPORT_MARGIN * 2}px));` +
            ` max-height: calc(100vh - ${VIEWPORT_MARGIN * 2}px); overflow-y: auto;`;
    }

    function toggleTooltip() {
        tooltipVisible = !tooltipVisible;
        if (tooltipVisible) updatePosition();
    }

    function hideTooltip() {
        tooltipVisible = false;
    }

    function closeFromButton() {
        hideTooltip();
        triggerEl?.focus();
    }

    // The popup opens and closes only by a click or a tap on the «?» (never by hovering). It also closes
    // on Escape, on a click or tap anywhere outside it, and on its own close cross — a phone screen can be
    // covered by the popup completely, leaving no «outside» to tap.
    $effect(() => {
        if (!tooltipVisible || !tooltipEl) return;

        return dismissable(tooltipEl, { onDismiss: hideTooltip, trigger: () => triggerEl });
    });

    // The popup is measured only once it is on screen: the portal it is rendered into is still
    // hidden while the element is being attached, and a hidden element measures as nothing. The
    // frame callback runs before the browser paints, so the corrected position is the first one
    // the visitor ever sees.
    $effect(() => {
        if (!tooltipVisible || !tooltipEl) return;

        updatePosition();
        if (typeof requestAnimationFrame !== 'function') return;

        const frame = requestAnimationFrame(updatePosition);
        return () => cancelAnimationFrame(frame);
    });

    $effect(() => {
        if (!tooltipVisible) return;

        window.addEventListener('scroll', updatePosition, true);
        window.addEventListener('resize', updatePosition);

        return () => {
            window.removeEventListener('scroll', updatePosition, true);
            window.removeEventListener('resize', updatePosition);
        };
    });

    const defaultTooltipClass = 'fixed z-10000 bg-warn-600 text-white text-[14px] px-2 py-1 rounded shadow-lg w-max';
</script>

<!--
    The «?» is the size of its icon, with a 0.5 rem space to the word before it. The space is the
    button's own left padding, so a press on it opens the popup: as a margin of the button it was a strip
    that hit the wrapper — which the heading row of a section cancels — and as a margin of the wrapper it hit the row and folded
    the section. The padding is horizontal only, so a label row is as tall as its text. It was
    a 44 × 44 px button, which made every label that held one 44 px tall and pushed its box
    28 px lower than the box of a label without one. It takes no height of its own, so a label row is as tall as its
    text. It is a help icon inside a line of text, which WCAG 2.5.5 and 2.5.8 let stand below the tap-target size (the
    "Inline" exception); `data-help-icon` marks it for the phone sweep, which skips only its size (`e2e/helpers/phone.ts`).
    There is no negative margin — a box pulled back over its neighbours would sit on top of the field under the label.
    `align-middle` keeps a «?» in running text centred on the line.
    The popup is placed against this box, with a 4 px gap between them. It opens and closes by a click or a tap on the
    box and never by hovering; it also closes on Escape, on a click outside it, and on the white cross in its corner.
-->
<div class="relative inline-block align-middle" data-help-icon>
    <button
        bind:this={triggerEl}
        type="button"
        data-test="{testId}-button"
        aria-label={ariaLabel}
        class="flex shrink-0 cursor-pointer {flush
            ? ''
            : 'pl-2'} items-center justify-center rounded-full border-0 bg-transparent {PRESS_FEEDBACK} {FOCUS_RING} pressable:active:opacity-60 {buttonClass}"
        onclick={toggleTooltip}
    >
        <!-- The popup stands against the glyph, not against the button, whose left padding is part of the press only. -->
        <span bind:this={glyphEl} class="flex">
            {#if icon}
                {@render icon()}
            {:else}
                <QuestionIcon size={iconSize} aria-hidden="true" />
            {/if}
        </span>
    </button>

    {#if tooltipVisible}
        <Portal>
            <div
                bind:this={tooltipEl}
                role="tooltip"
                data-test="{testId}-tooltip"
                class="{defaultTooltipClass} {tooltipClass}"
                style={tooltipStyle}
            >
                <PopupCloseButton onclick={closeFromButton} testId="{testId}-close" />
                {#each tooltip.split(/\n{2,}/) as paragraph, index (index)}
                    <p class={index > 0 ? 'mt-2' : ''}>{paragraph}</p>
                {/each}
                <HelpLinkList {links} testId="{testId}-links" linkTestId="{testId}-link" tone="onAmber" class="mt-2" />
            </div>
        </Portal>
    {/if}
</div>

<style>
    :global(.help-tooltip-icon) {
        display: inline-block;
    }
</style>

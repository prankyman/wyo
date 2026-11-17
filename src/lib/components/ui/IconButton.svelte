<script lang="ts">
    import type { Snippet } from 'svelte';
    import IconTooltip from './IconTooltip.svelte';
    import { FOCUS_RING, PRESS_FEEDBACK } from './press';
    import type { TTooltipSide } from './tooltip-position';

    let {
        icon,
        tooltip,
        description,
        tooltipSide = 'above',
        variant = 'default',
        type = 'button',
        ariaLabel,
        href,
        onclick,
        disabled = false,
        class: className = '',
        'data-test': dataTest,
        'data-tour': dataTour
    }: {
        icon?: Snippet;
        tooltip: string;
        /** One sentence of what the button does, under its name in the tooltip. */
        description?: string;
        /** Which side of the button the tooltip prefers (`IconTooltip`). */
        tooltipSide?: TTooltipSide;
        variant?: 'default' | 'delete' | 'archive' | 'unarchive';
        type?: 'button' | 'submit';
        ariaLabel: string;
        href?: string;
        onclick?: () => void;
        disabled?: boolean;
        class?: string;
        'data-test'?: string;
        /** Anchor for a guided tour or a walk-through. Distinct from `data-test`. */
        'data-tour'?: string;
    } = $props();

    /**
     * Pressed, it shrinks under the finger (`press.ts`); a keyboard focus shows a ring. Kept as its
     * own element rather than a `Button` because of the tooltip it carries (`IconTooltip`, shown on hover
     * and on keyboard focus) — an allowlisted exception in `press-feedback-policy.test.ts`.
     */
    const pressClasses = `${PRESS_FEEDBACK} ${FOCUS_RING}`;

    const variantClasses = {
        default: 'text-ink-400 pressable:hover:text-ink-900',
        delete: 'text-stop-600 pressable:hover:text-stop-800',
        archive: 'text-warn-600 pressable:hover:text-warn-800',
        unarchive: 'text-ok-600 pressable:hover:text-ok-800'
    };
</script>

{#snippet content()}
    {#if icon}
        {@render icon()}
    {/if}
{/snippet}

<IconTooltip name={tooltip} {description} side={tooltipSide} testId={dataTest ? `${dataTest}-tip` : undefined}>
    {#if href}
        <!-- A link that cannot be followed has no href and says so, as a `Button` link does. -->
        <a
            href={disabled ? undefined : href}
            {onclick}
            aria-disabled={disabled || undefined}
            class="inline-flex min-h-11 min-w-11 items-center justify-center p-2 {variantClasses[
                variant
            ]} {pressClasses} {className} {disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
            aria-label={ariaLabel}
            data-test={dataTest}
            data-tour={dataTour}
        >
            {@render content()}
        </a>
    {:else}
        <button
            {type}
            {onclick}
            {disabled}
            class="inline-flex min-h-11 min-w-11 items-center justify-center p-2 {variantClasses[
                variant
            ]} {pressClasses} {className} {disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
            aria-label={ariaLabel}
            data-test={dataTest}
            data-tour={dataTour}
        >
            {@render content()}
        </button>
    {/if}
</IconTooltip>

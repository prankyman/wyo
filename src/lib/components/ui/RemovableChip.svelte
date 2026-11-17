<script lang="ts">
    import XIcon from 'phosphor-svelte/lib/X';
    import { FOCUS_RING, PRESS_FEEDBACK } from './press';

    /**
     * One chip that takes a choice off: a picker's chosen row, or a filter in use. The whole chip is the control — its words
     * and the cross together, at least 44 px tall — so a tap anywhere on it removes the choice.
     * `ariaLabel` quotes everything the chip shows and says what a tap does ("Убрать Статус: Активен").
     *
     * A chip is a button when the screen changes its own state (`onRemove`) and a link when the choice lives in the address
     * (`href` — the address without that choice).
     */
    interface IProps {
        label: string;
        /** A quieter second part after the label (a code and its name). */
        sublabel?: string;
        /** What a screen reader hears: the chip's words and what a tap does. */
        ariaLabel: string;
        onRemove?: () => void;
        /** Where a tap goes instead of calling `onRemove`. */
        href?: string;
        dataTest: string;
    }

    let { label, sublabel, ariaLabel, onRemove, href, dataTest }: IProps = $props();

    const chipClass = `inline-flex min-h-11 items-center gap-2 border border-ink-300 bg-ink-50 px-3 text-xs text-ink-700 pressable:hover:bg-ink-100 pressable:active:bg-ink-200 ${PRESS_FEEDBACK} ${FOCUS_RING}`;
</script>

{#snippet words()}
    {label}
    {#if sublabel}<span class="text-ink-400">{sublabel}</span>{/if}
    <XIcon class="size-3 shrink-0 text-ink-400" aria-hidden="true" />
{/snippet}

{#if href !== undefined}
    <a {href} aria-label={ariaLabel} data-test={dataTest} class={chipClass}>{@render words()}</a>
{:else}
    <button type="button" onclick={() => onRemove?.()} aria-label={ariaLabel} data-test={dataTest} class={chipClass}>
        {@render words()}
    </button>
{/if}

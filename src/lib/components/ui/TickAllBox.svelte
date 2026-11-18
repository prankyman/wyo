<script lang="ts">
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import IconTooltip from './IconTooltip.svelte';
    import type { TTickAllState } from './tick-all';

    /**
     * The tick box in the heading of a list whose rows are ticked, in place of the buttons «Отметить все
     * показанные» and «Снять отметки»: empty while no row is ticked, a dash while some are, ticked while all are. A press ticks
     * every row unless all are ticked, then it unticks them; its name on hover says which. The box always shows `status`, the
     * page's own count of ticked rows, never what the browser's press alone made of it.
     */
    interface IProps {
        lang: TLocale;
        status: TTickAllState;
        /** true: tick every row; false: untick them. */
        onTick: (all: boolean) => void;
        dataTest: string;
        /** The name while not every row is ticked; «Отметить все показанные» without it. */
        tickLabel?: string;
        /** The name while every row is ticked; «Снять отметки» without it. */
        untickLabel?: string;
        disabled?: boolean;
        /** The box's own look, the same as the rows' boxes under it. */
        inputClass?: string;
        /** Where the box stands in its 44 px tap area: in the middle (the lists' rows), or at its start (a row's box before its words). */
        align?: 'center' | 'start';
        /** The 44 px tap area's place (`absolute top-0 right-1` over the cards). */
        class?: string;
    }

    let {
        lang,
        status,
        onTick,
        dataTest,
        tickLabel,
        untickLabel,
        disabled = false,
        inputClass = '',
        align = 'center',
        class: className = ''
    }: IProps = $props();

    let input = $state<HTMLInputElement | undefined>(undefined);
    const label = $derived(
        status === 'all' ? (untickLabel ?? t('app.tickAll.untick', lang)) : (tickLabel ?? t('app.tickAll.tick', lang))
    );

    /** The box shows the page's state: on drawing, on every change of it, and back after the browser's own toggle. */
    function show(): void {
        if (!input) return;
        input.checked = status === 'all';
        input.indeterminate = status === 'some';
    }
    $effect(show);

    function press(): void {
        onTick(status !== 'all');
        show();
    }
</script>

<IconTooltip name={label} side="above" testId="{dataTest}-tip" class={className}>
    <label
        class="flex min-h-11 min-w-11 cursor-pointer items-center {align === 'start'
            ? 'justify-start'
            : 'justify-center'}"
    >
        <input
            bind:this={input}
            type="checkbox"
            {disabled}
            aria-label={label}
            onchange={press}
            data-test={dataTest}
            data-state={status}
            class={inputClass}
        />
    </label>
</IconTooltip>

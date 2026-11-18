<script lang="ts">
    import HelpTooltip from './HelpTooltip.svelte';

    /**
     * The facts of one record as a list of names and values — an account's seller and key, an order's article and barcodes, a
     * journal row's error. A name with a help text carries its «?» right after it. The values come already worded; a fact with
     * nothing to say is left out by the caller, never drawn empty.
     */
    interface IFact {
        /** The fact's name in the list: its value is `<dataTest>-<key>`, its «?» `<dataTest>-<key>-help`. */
        key: string;
        label: string;
        value: string;
        /** What the fact is and why it is shown; without it there is no «?». */
        help?: string;
    }

    interface IProps {
        items: IFact[];
        dataTest: string;
    }

    let { items, dataTest }: IProps = $props();
</script>

<dl class="grid grid-cols-1 gap-x-4 gap-y-2 text-sm sm:grid-cols-[minmax(0,14rem)_minmax(0,1fr)]" data-test={dataTest}>
    {#each items as item (item.key)}
        <dt class="flex items-center gap-1 text-xs text-ink-600 sm:text-sm">
            {item.label}
            {#if item.help}
                <HelpTooltip tooltip={item.help} ariaLabel={item.help} testId="{dataTest}-{item.key}-help" />
            {/if}
        </dt>
        <dd class="break-words text-ink-800" data-test="{dataTest}-{item.key}">{item.value}</dd>
    {/each}
</dl>

<script lang="ts" generics="TRow extends IDataListRow">
    import type { Snippet } from 'svelte';
    import ClickableCard from '$lib/components/ui/ClickableCard.svelte';
    import ClickableRow from '$lib/components/ui/ClickableRow.svelte';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import StatusBadge from '$lib/components/ui/StatusBadge.svelte';
    import StretchedLink from '$lib/components/ui/StretchedLink.svelte';
    import TickAllBox from '$lib/components/ui/TickAllBox.svelte';
    import { tickAllState } from '$lib/components/ui/tick-all';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import ColumnHeading from './ColumnHeading.svelte';
    import ColumnLegend from './ColumnLegend.svelte';
    import type { IDataListColumn, IDataListRow } from './types';

    interface IProps {
        lang: TLocale;
        columns: IDataListColumn<TRow>[];
        rows: TRow[];
        /** i18n key shown when there is nothing to list. */
        emptyKey?: string;
        dataTest?: string;
        /** The tour anchor (`data-tour`) on the rows, in both layouts; absent while the list is empty. */
        dataTour?: string;
        /**
         * A checkbox before each row, for a press on several rows at once («Удалить выбранные»). The label is a message key
         * worded with the row's primary text as `{name}`; the ticked row ids are `selected`.
         */
        selectLabelKey?: string;
        selected?: string[];
        /**
         * The name of the heading's tick box while not every row is ticked («Отметить все на странице» on a list of pages);
         * «Отметить все показанные» without it. The box stands in the table's first heading cell and, on a phone, above the
         * cards, right over the cards' boxes.
         */
        tickAllKey?: string;
        /**
         * The page's line above the list — what it does with the ticked rows. On a phone the heading's tick box
         * stands at its right end, over the cards' boxes, so the box takes no line of its own.
         */
        toolbar?: Snippet;
        /** The line's spacing from the list (`mb-3` where the page does not space its blocks by a gap). */
        toolbarClass?: string;
        /**
         * A press of the row's own, drawn after its cells and outside the row's link. Nothing is drawn for a row the snippet
         * leaves empty.
         */
        rowAction?: Snippet<[TRow]>;
        /** Draws the cell of a column marked `custom`; the primary one is the card's heading on a phone. */
        cell?: Snippet<[TRow, IDataListColumn<TRow>]>;
        /** The names and «?» of the columns that have a `helpKey`, listed above the cards (phone only). */
        legend?: boolean;
        /** The width from which the table replaces the cards. */
        breakpoint?: 'sm' | 'md';
        /** Every table cell carries `data-test="{dataTest}-{column.key}"` (the table only, not the cards). */
        cellTestIds?: boolean;
    }

    /** Whole class strings, so Tailwind finds them. */
    const BREAKPOINTS = {
        sm: { table: 'hidden overflow-x-auto sm:block', cards: 'space-y-2 sm:hidden', cardsTick: 'sm:hidden' },
        md: { table: 'hidden overflow-x-auto md:block', cards: 'space-y-2 md:hidden', cardsTick: 'md:hidden' }
    } as const;

    let {
        lang,
        columns,
        rows,
        emptyKey = 'app.list.empty',
        dataTest = 'data-list',
        dataTour,
        selectLabelKey,
        selected = $bindable([]),
        tickAllKey,
        toolbar,
        toolbarClass = '',
        rowAction,
        cell,
        legend = false,
        breakpoint = 'md',
        cellTestIds = false
    }: IProps = $props();

    const primary = $derived(columns.find((column) => column.emphasis === 'primary') ?? columns[0]);
    const secondary = $derived(columns.find((column) => column.emphasis === 'secondary'));
    const cardRest = $derived(
        columns.filter((column) => column !== primary && column !== secondary && !column.desktopOnly)
    );

    const shownIds = $derived(rows.map((row) => row.id));
    const tickStatus = $derived(tickAllState(shownIds.filter((id) => selected.includes(id)).length, shownIds.length));
    /** The heading's box: ticks every row shown, or unticks them; rows ticked elsewhere stay as they are. */
    function tickShown(all: boolean): void {
        selected = all
            ? [...selected, ...shownIds.filter((id) => !selected.includes(id))]
            : selected.filter((id) => !shownIds.includes(id));
    }

    const text = (row: TRow, column: IDataListColumn<TRow> | undefined): string => {
        if (!column) return '';
        const value = row.cells[column.key];
        return value === null || value === undefined ? '—' : String(value);
    };
</script>

<!--
    ONE column definition, two renderings. Below `md` a table is unreadable on a phone — it either
    scrolls sideways, which hides the columns that matter, or squeezes every column to three
    characters. So the same rows become stacked cards there. Writing the two layouts as separate
    markup was the alternative, and it means every new column has to be added twice and will
    eventually be added once.
-->

{#if rows.length === 0}
    <EmptyState message={t(emptyKey, lang)} dataTest="{dataTest}-empty" />
{:else}
    {#if toolbar || selectLabelKey}
        <!--
            The page's line, and on a phone the heading's tick box at its right end, at the line's bottom: when the page's buttons
            wrap to more lines, the box stays right over the cards. The box's place is as wide as the cards' box's
            place and, like a card, has a 1 px border: the box is `absolute top-0 right-1` in it, as the cards' box is `top-1 right-1`.
            Without the page's line and on a wide screen, where the box is in the table, a line holding only the hidden box is not drawn.
        -->
        <div
            class="flex items-end gap-2 {toolbar ? '' : BREAKPOINTS[breakpoint].cardsTick} {toolbarClass}"
            data-test="{dataTest}-toolbar"
        >
            <div class="min-w-0 flex-1">{@render toolbar?.()}</div>
            {#if selectLabelKey}
                <div
                    class="relative ml-auto h-11 w-12 shrink-0 border border-transparent {BREAKPOINTS[breakpoint]
                        .cardsTick}"
                    data-test="{dataTest}-toolbar-tick"
                >
                    {@render tickAll('absolute top-0 right-1', `${dataTest}-cards-tick-all`)}
                </div>
            {/if}
        </div>
    {/if}
    <!-- Desktop: a real table. -->
    <div class={BREAKPOINTS[breakpoint].table} data-tour={dataTour}>
        <table class="w-full border-collapse text-sm" data-test="{dataTest}-table">
            <thead>
                <tr class="border-b border-ink-200 text-left text-xs text-ink-500">
                    {#if selectLabelKey}
                        <th class="w-12 px-3 py-1">{@render tickAll('relative', `${dataTest}-tick-all`)}</th>
                    {/if}
                    {#each columns as column (column.key)}
                        <th class="px-3 py-2 font-medium {column.align === 'end' ? 'text-right' : ''}">
                            {#if column.helpKey}
                                <ColumnHeading {lang} {column} testPrefix={dataTest} />
                            {:else}
                                {t(column.labelKey, lang)}
                            {/if}
                        </th>
                    {/each}
                    {#if rowAction}
                        <th class="px-3 py-2"><span class="sr-only">{t('app.list.actions', lang)}</span></th>
                    {/if}
                </tr>
            </thead>
            <tbody>
                {#each rows as row (row.id)}
                    {@const tone = row.alert ? 'text-stop-700' : row.muted ? 'text-ink-400' : 'text-ink-800'}
                    {#if row.href}
                        <!-- The whole row opens the record; its number/name is the one link. -->
                        <ClickableRow
                            class="border-b border-ink-100 {tone}"
                            dataTest="{dataTest}-row"
                            data-row-id={row.id}
                            {...row.attributes}
                        >
                            {@render cells(row, row.href)}
                        </ClickableRow>
                    {:else}
                        <tr
                            class="border-b border-ink-100 hover:bg-ink-50 {tone}"
                            data-test="{dataTest}-row"
                            data-row-id={row.id}
                            {...row.attributes}
                        >
                            {@render cells(row, undefined)}
                        </tr>
                    {/if}
                {/each}
            </tbody>
        </table>
    </div>

    <!-- Phone: the same rows as cards. A card with an address opens it wherever it is pressed. -->
    {#if legend}
        <ColumnLegend {lang} {columns} testPrefix={dataTest} {breakpoint} />
    {/if}
    <ul class={BREAKPOINTS[breakpoint].cards} data-test="{dataTest}-cards" data-tour={dataTour}>
        {#each rows as row (row.id)}
            {@const tone = row.alert ? 'text-stop-700' : row.muted ? 'text-ink-400' : 'text-ink-800'}
            {#if row.href}
                <ClickableCard
                    as="li"
                    href={row.href}
                    class={tone}
                    dataTest="{dataTest}-card"
                    linkDataTest="{dataTest}-card-link"
                    linkClass="flex min-h-11 flex-col items-start justify-center gap-0 font-normal"
                    data-row-id={row.id}
                    {...row.attributes}
                >
                    {#snippet title()}
                        {@render heading(row)}
                    {/snippet}
                    <!-- Outside the card's link: a checkbox inside a link would open the record. -->
                    {@render checkbox(row, 'absolute top-1 right-1')}
                    {@render cardRows(row)}
                    {#if rowAction}<div class="relative z-10 mt-2">{@render rowAction(row)}</div>{/if}
                </ClickableCard>
            {:else}
                <li
                    class="relative rounded border border-ink-200 bg-white px-4 py-3 {tone}"
                    data-test="{dataTest}-card"
                    data-row-id={row.id}
                    {...row.attributes}
                >
                    {@render checkbox(row, 'absolute top-1 right-1')}
                    {@render heading(row)}
                    {@render cardRows(row)}
                    {#if rowAction}<div class="mt-2">{@render rowAction(row)}</div>{/if}
                </li>
            {/if}
        {/each}
    </ul>
{/if}

{#snippet badge(row: TRow)}
    {#if row.badge}
        <span
            class="ml-2 inline-block bg-wait-100 px-2 py-0.5 align-middle text-[10px] font-normal text-wait-800"
            data-test="{dataTest}-badge"
        >
            {row.badge}
        </span>
    {/if}
{/snippet}

{#snippet tickAll(placement: string, testId: string)}
    <TickAllBox
        {lang}
        status={tickStatus}
        onTick={tickShown}
        tickLabel={tickAllKey ? t(tickAllKey, lang) : undefined}
        dataTest={testId}
        inputClass="size-5 accent-ink-900"
        class="{placement} z-10"
    />
{/snippet}

{#snippet checkbox(row: TRow, placement: string)}
    {#if selectLabelKey}
        <label class="{placement} z-10 flex min-h-11 min-w-11 cursor-pointer items-center justify-center">
            <input
                type="checkbox"
                value={row.id}
                bind:group={selected}
                aria-label={t(selectLabelKey, lang, { name: text(row, primary) })}
                data-test="{dataTest}-select"
                data-row-id={row.id}
                class="size-5 accent-ink-900"
            />
        </label>
    {/if}
{/snippet}

{#snippet cells(row: TRow, href: string | undefined)}
    {#if selectLabelKey}
        <td class="w-12 px-3 py-1">{@render checkbox(row, 'relative')}</td>
    {/if}
    {#each columns as column (column.key)}
        <td
            class="px-3 py-2 {column.align === 'end' ? 'text-right tabular-nums' : ''}"
            data-test={cellTestIds ? `${dataTest}-${column.key}` : undefined}
        >
            {#if column.custom && cell}
                {@render cell(row, column)}
            {:else if column.badge}
                {@render badgeCell(row, column)}
            {:else if column === primary && href}
                <StretchedLink {href} class="underline-offset-2 hover:underline" dataTest="{dataTest}-link">
                    {text(row, column)}
                </StretchedLink>
            {:else}
                {text(row, column)}
            {/if}
            {#if column === primary}{@render badge(row)}{/if}
        </td>
    {/each}
    {#if rowAction}
        <td class="relative z-10 px-3 py-1 text-right">{@render rowAction(row)}</td>
    {/if}
{/snippet}

{#snippet heading(row: TRow)}
    {#if primary?.custom && cell}
        <span class="block {selectLabelKey ? 'pr-10' : ''}">{@render cell(row, primary)}{@render badge(row)}</span>
    {:else}
        <span class="block font-medium {selectLabelKey ? 'pr-10' : ''}">{text(row, primary)}{@render badge(row)}</span>
    {/if}
    {#if secondary}
        <!-- A custom secondary column draws itself here too. -->
        <span class="block text-xs text-ink-500">
            {#if secondary.custom && cell}{@render cell(row, secondary)}{:else}{text(row, secondary)}{/if}
        </span>
    {/if}
{/snippet}

{#snippet cardRows(row: TRow)}
    {#if cardRest.length > 0}
        <dl class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
            {#each cardRest as column (column.key)}
                <dt class="text-ink-500">{t(column.labelKey, lang)}</dt>
                <dd class="text-right tabular-nums">
                    {#if column.custom && cell}
                        {@render cell(row, column)}
                    {:else if column.badge}
                        {@render badgeCell(row, column)}
                    {:else}
                        {text(row, column)}
                    {/if}
                </dd>
            {/each}
        </dl>
    {/if}
{/snippet}

{#snippet badgeCell(row: TRow, column: IDataListColumn<TRow>)}
    {#if column.badge}
        <StatusBadge
            label={column.badge.label(row)}
            colorClass={column.badge.colorClass(row)}
            dataTest={column.badge.dataTest}
        />
    {/if}
{/snippet}

<script lang="ts" generics="TRow extends IDataListRow">
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import ColumnHeading from './ColumnHeading.svelte';
    import type { IDataListColumn, IDataListRow } from './types';

    /**
     * On a phone the rows are cards without column headings, so the names of the columns that have a «?» are listed once above
     * the cards (a part of `DataList`); a computer reads them in the table's headings.
     */
    interface IProps {
        lang: TLocale;
        columns: readonly IDataListColumn<TRow>[];
        testPrefix: string;
        breakpoint?: 'sm' | 'md';
    }

    let { lang, columns, testPrefix, breakpoint = 'md' }: IProps = $props();

    /** Whole class strings, so Tailwind finds them. */
    const HIDDEN_FROM = { sm: 'sm:hidden', md: 'md:hidden' } as const;
    const helped = $derived(columns.filter((column) => column.helpKey));
</script>

<div
    class="rounded border border-ink-200 bg-ink-50 px-3 py-2 text-xs text-ink-600 {HIDDEN_FROM[breakpoint]}"
    data-test="{testPrefix}-legend"
>
    <p class="mb-1">{t('app.list.legend', lang)}</p>
    <ul class="flex flex-wrap gap-x-3 gap-y-1">
        {#each helped as column (column.key)}
            <li><ColumnHeading {lang} {column} testPrefix="{testPrefix}-legend" /></li>
        {/each}
    </ul>
</div>

<script lang="ts">
    import HelpTooltip from '$lib/components/ui/HelpTooltip.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { IDataListColumn } from './types';

    /** A column's heading with its «?», a part of `DataList`: the label, and the help that a reader may open alone. */
    interface IProps {
        lang: TLocale;
        column: IDataListColumn;
        testPrefix: string;
    }

    let { lang, column, testPrefix }: IProps = $props();
</script>

<span class="inline-flex items-center gap-1">
    {t(column.labelKey, lang)}
    {#if column.helpKey}
        <HelpTooltip
            tooltip={t(column.helpKey, lang)}
            ariaLabel={t(column.labelKey, lang)}
            testId="{testPrefix}-{column.key}-help"
        />
    {/if}
</span>

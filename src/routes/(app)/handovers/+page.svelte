<script lang="ts">
    import { invalidateAll } from '$app/navigation';
    import { page } from '$app/state';
    import { refusalKey } from '$lib/api/refusal-key';
    import { problemSentence } from '$lib/components/exchange/problem-text';
    import { batchKindOf } from '$lib/components/handovers/batch-kind';
    import HandoverDetails from '$lib/components/handovers/HandoverDetails.svelte';
    import HandoverStateBadge from '$lib/components/handovers/HandoverStateBadge.svelte';
    import type { THandover, THandoverDetails } from '$lib/components/handovers/types';
    import Button from '$lib/components/ui/Button.svelte';
    import DataList from '$lib/components/ui/DataList.svelte';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import RowDetailsPanel from '$lib/components/ui/RowDetailsPanel.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import type { IDataListColumn, IDataListRow } from '$lib/components/ui/types';
    import { formatIsoDay } from '$lib/i18n/format-date';
    import { getLocale, t } from '$lib/i18n/i18n';
    import { addressWith } from '$lib/lists/list-address';
    import type { PageData } from './$types';

    /**
     * «Передачи»: the hand-overs the fulfilment centre asked documents for, newest first (`DataList`: a table on a computer,
     * cards on a phone) — the fulfilment centre's number, the day, the state, the marketplace's numbers of the batches, how many
     * orders and the problem in a line. A row opens the hand-over beside the list (`RowDetailsPanel`, `?handover=<id>`): its
     * steps, its problem, the shipping point to choose and «Повторить» are `HandoverDetails`. The page adds only the list and the
     * panel; after a repeat or a saved point it reads its data again.
     */
    let { data }: { data: PageData } = $props();

    const lang = getLocale();

    interface IHandoverRow extends IDataListRow {
        handover: THandover;
    }

    const columns: IDataListColumn<IHandoverRow>[] = [
        {
            key: 'number',
            labelKey: 'handovers.column.number',
            helpKey: 'handovers.column.number.help',
            emphasis: 'primary'
        },
        { key: 'day', labelKey: 'handovers.column.day', helpKey: 'handovers.column.day.help', emphasis: 'secondary' },
        { key: 'state', labelKey: 'handovers.column.state', helpKey: 'handovers.column.state.help', custom: true },
        { key: 'batches', labelKey: 'handovers.column.batches', helpKey: 'handovers.column.batches.help' },
        {
            key: 'orders',
            labelKey: 'handovers.column.orders',
            helpKey: 'handovers.column.orders.help',
            align: 'end'
        },
        { key: 'problem', labelKey: 'handovers.column.problem', helpKey: 'handovers.column.problem.help' }
    ];

    const rows = $derived<IHandoverRow[]>(
        (data.list?.handovers ?? []).map((handover) => ({
            id: String(handover.id),
            handover,
            href: addressWith(page.url, { handover: String(handover.id) }),
            cells: {
                number: handover.number,
                day: formatIsoDay(handover.day, lang),
                batches:
                    [handover.batches.join(', '), batchKindOf(handover, lang)?.text].filter(Boolean).join(' · ') ||
                    null,
                orders: handover.orders,
                problem: handover.problem ? problemSentence(handover.problem, lang) : null
            },
            attributes: { 'data-handover-id': String(handover.id), 'data-state': handover.state }
        }))
    );
</script>

<Stack as="section" dataTest="handovers">
    <PageHeading
        title={t('handovers.title', lang)}
        help={t('handovers.help', lang)}
        headingTestId="handovers-heading"
        helpTestId="handovers-help"
    />

    {#if data.refusal}
        <NoteBox tone="error" role="alert" dataTest="handovers-refusal">{t(refusalKey(data.refusal), lang)}</NoteBox>
        <div>
            <Button variant="secondary" dataTest="handovers-retry" onclick={() => invalidateAll()}>
                {t('common.retry', lang)}
            </Button>
        </div>
    {:else if rows.length === 0}
        <EmptyState message={t('handovers.empty', lang)} dataTest="handovers-empty" />
    {:else}
        <DataList {lang} {columns} {rows} legend dataTest="handovers-list">
            {#snippet cell(row: IHandoverRow, column: IDataListColumn<IHandoverRow>)}
                {#if column.key === 'state'}
                    <HandoverStateBadge {lang} state={row.handover.state} testId="handovers-list-state" />
                {/if}
            {/snippet}
        </DataList>
    {/if}

    <RowDetailsPanel
        {lang}
        param="handover"
        fromLoad={data.opened}
        idOf={(details: THandoverDetails) => String(details.handover.id)}
        pick={(loaded) => loaded.opened as THandoverDetails | null | undefined}
        listHref={addressWith(page.url, { handover: null })}
        title={(details: THandoverDetails) => t('handovers.details.title', lang, { number: details.handover.number })}
        dataTest="handovers-details"
    >
        {#snippet children(details: THandoverDetails)}
            <HandoverDetails {lang} {details} onChanged={() => invalidateAll()} />
        {/snippet}
    </RowDetailsPanel>
</Stack>

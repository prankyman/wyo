<script lang="ts">
    import DataList from '$lib/components/ui/DataList.svelte';
    import DateRangeFields from '$lib/components/ui/DateRangeFields.svelte';
    import FactList from '$lib/components/ui/FactList.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { toneClass, type TTone } from '$lib/components/ui/status-tones';
    import TickAllBox from '$lib/components/ui/TickAllBox.svelte';
    import type { TTickAllState } from '$lib/components/ui/tick-all';
    import type { IDataListColumn, IDataListRow } from '$lib/components/ui/types';
    import { getLocale, t } from '$lib/i18n/i18n';

    /**
     * The showcase of the list components of the library: `DataList` (a table on a computer, cards on a phone) with its column
     * headings' «?» (`ColumnHeading`), the legend above the cards (`ColumnLegend`) and the heading's tick box (`TickAllBox`);
     * `DateRangeFields`; `FactList`. Every one is drawn in each of its states with made-up records, so the theme can be judged by
     * eye and the browser tests can measure the page at 390 pixels. Each component is tested once in its own file; this section
     * only draws them.
     */
    const lang = getLocale();
    const text = (key: string, params?: Record<string, string | number>): string =>
        t(`showcase.lists.${key}`, lang, params);

    type TStatus = 'active' | 'late' | 'closed';
    interface IRecordRow extends IDataListRow {
        status: TStatus;
    }

    const STATUS: Record<TStatus, { label: string; tone: TTone }> = {
        active: { label: text('status.active'), tone: 'ok' },
        late: { label: text('status.late'), tone: 'stop' },
        closed: { label: text('status.closed'), tone: 'off' }
    };

    const COLUMNS: IDataListColumn<IRecordRow>[] = [
        {
            key: 'number',
            labelKey: 'showcase.lists.column.number',
            helpKey: 'showcase.lists.column.number.help',
            emphasis: 'primary'
        },
        { key: 'name', labelKey: 'showcase.lists.column.name', emphasis: 'secondary' },
        {
            key: 'status',
            labelKey: 'showcase.lists.column.status',
            helpKey: 'showcase.lists.column.status.help',
            badge: {
                label: (row) => STATUS[row.status].label,
                colorClass: (row) => toneClass(STATUS[row.status].tone),
                dataTest: 'showcase-lists-list-status'
            }
        },
        {
            key: 'quantity',
            labelKey: 'showcase.lists.column.quantity',
            helpKey: 'showcase.lists.column.quantity.help',
            align: 'end'
        },
        { key: 'created', labelKey: 'showcase.lists.column.created', desktopOnly: true }
    ];

    const ROWS: IRecordRow[] = [
        {
            id: '1001',
            href: '/showcase',
            status: 'active',
            badge: text('badge'),
            cells: { number: '1001', name: text('row.first'), quantity: 12, created: '18.11.2026' }
        },
        {
            id: '1002',
            href: '/showcase',
            status: 'late',
            alert: true,
            cells: { number: '1002', name: text('row.second'), quantity: 3, created: '16.11.2026' }
        },
        {
            id: '1003',
            status: 'closed',
            muted: true,
            cells: { number: '1003', name: text('row.third'), quantity: null, created: '10.11.2026' }
        }
    ];

    let selected = $state(['1001']);

    const TICKS: { state: TTickAllState; label: string }[] = [
        { state: 'none', label: text('ticks.none') },
        { state: 'some', label: text('ticks.some') },
        { state: 'all', label: text('ticks.all') }
    ];

    let from = $state('2026-11-01');
    let to = $state('2026-11-07');

    const FACTS = [
        { key: 'number', label: text('facts.number'), value: '1001', help: text('facts.number.help') },
        { key: 'created', label: text('facts.created'), value: '18.11.2026, 12:00', help: text('facts.created.help') },
        { key: 'note', label: text('facts.note'), value: text('facts.noteValue') }
    ];
</script>

<Stack as="section" dataTest="showcase-lists">
    <HeadingWithHelp text={text('title')} look="title" level="h2" />

    <Stack as="section" dataTest="showcase-lists-list">
        <HeadingWithHelp text={text('list.title')} look="section" level="h2" />
        <DataList
            {lang}
            columns={COLUMNS}
            rows={ROWS}
            bind:selected
            selectLabelKey="showcase.lists.select"
            legend
            dataTest="showcase-lists-list"
        />
    </Stack>

    <Stack as="section" dataTest="showcase-lists-empty">
        <HeadingWithHelp text={text('empty.title')} look="section" level="h2" />
        <DataList
            {lang}
            columns={COLUMNS}
            rows={[]}
            emptyKey="showcase.lists.empty.message"
            dataTest="showcase-lists-empty-list"
        />
    </Stack>

    <Stack as="section" dataTest="showcase-lists-ticks">
        <HeadingWithHelp text={text('ticks.title')} look="section" level="h2" />
        <div class="flex flex-wrap items-center gap-4">
            {#each TICKS as { state, label } (state)}
                <span class="inline-flex items-center gap-1">
                    <TickAllBox
                        {lang}
                        status={state}
                        onTick={() => {}}
                        inputClass="size-5 accent-ink-900"
                        dataTest="showcase-lists-tick-{state}"
                    />
                    {label}
                </span>
            {/each}
            <span class="inline-flex items-center gap-1">
                <TickAllBox
                    {lang}
                    status="none"
                    onTick={() => {}}
                    disabled
                    inputClass="size-5 accent-ink-900"
                    dataTest="showcase-lists-tick-disabled"
                />
                {text('ticks.disabled')}
            </span>
        </div>
    </Stack>

    <Stack as="section" dataTest="showcase-lists-dates">
        <HeadingWithHelp text={text('dates.title')} look="section" level="h2" />
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <DateRangeFields
                bind:from
                bind:to
                fromLabel={text('dates.from')}
                toLabel={text('dates.to')}
                fromHelp={text('dates.fromHelp')}
                testPrefix="showcase-lists-dates-plain"
            />
            <DateRangeFields
                from="2026-11-07"
                to="2026-11-01"
                fromLabel={text('dates.from')}
                toLabel={text('dates.to')}
                errors={{ to: text('dates.refusal') }}
                testPrefix="showcase-lists-dates-refused"
            />
            <DateRangeFields
                from="2026-11-01"
                to="2026-11-07"
                fromLabel={text('dates.from')}
                toLabel={text('dates.to')}
                disabled
                testPrefix="showcase-lists-dates-disabled"
            />
        </div>
    </Stack>

    <Stack as="section" dataTest="showcase-lists-facts">
        <HeadingWithHelp text={text('facts.title')} look="section" level="h2" />
        <FactList items={FACTS} dataTest="showcase-lists-fact-list" />
    </Stack>
</Stack>

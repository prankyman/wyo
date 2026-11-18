<script lang="ts">
    import { goto, invalidateAll } from '$app/navigation';
    import { page } from '$app/state';
    import { refusalKey } from '$lib/api/refusal-key';
    import JournalOutcomeBadge from '$lib/components/journal/JournalOutcomeBadge.svelte';
    import type { TJournalEntry } from '$lib/components/journal/types';
    import MarketplaceMark from '$lib/components/marketplaces/MarketplaceMark.svelte';
    import Button from '$lib/components/ui/Button.svelte';
    import DataList from '$lib/components/ui/DataList.svelte';
    import DateRangeFields from '$lib/components/ui/DateRangeFields.svelte';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import FactList from '$lib/components/ui/FactList.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import FilterSection from '$lib/components/ui/FilterSection.svelte';
    import FormButtons from '$lib/components/ui/FormButtons.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import Pagination from '$lib/components/ui/Pagination.svelte';
    import RowDetailsPanel from '$lib/components/ui/RowDetailsPanel.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import type { IDataListColumn, IDataListRow } from '$lib/components/ui/types';
    import { pageSearch } from '$lib/forms/page-search.svelte';
    import { tError } from '$lib/i18n/form-error';
    import { formatDateTime, formatIsoDay } from '$lib/i18n/format-date';
    import { getLocale, hasMessage, localizePath, t } from '$lib/i18n/i18n';
    import { filtersInWords } from '$lib/lists/filter-words';
    import { addressWith, filterChips } from '$lib/lists/list-address';
    import { accountNameOf, accountOptions } from '$lib/marketplaces/account-options';
    import { JOURNAL_FILTER_ID, JOURNAL_OUTCOMES, journalFilterSchema } from '$lib/validation/list-filters';
    import type { PageData } from './$types';

    /**
     * «Журнал запросов»: one row for every request the wyo server sent to a marketplace, newest first, 50 a page (`DataList`).
     * The filters — account, outcome, period — live in the address (`FilterSection`), so a filtered journal can be opened again by
     * its link; a row opens the whole request beside the list (`RowDetailsPanel`, `?entry=<id>`), its error's text with it. What
     * the journal is for and that it holds no key, no query and no body is said by the page's «?».
     */
    let { data }: { data: PageData } = $props();

    const lang = getLocale();
    const listPath = localizePath('/journal', lang);

    const search = pageSearch(journalFilterSchema, () => ({ ...data.filter }), JOURNAL_FILTER_ID);
    const { form: filters, errors: filterErrors, enhance, submitting } = search;

    interface IEntryRow extends IDataListRow {
        entry: TJournalEntry;
    }

    const columns: IDataListColumn<IEntryRow>[] = [
        { key: 'at', labelKey: 'journal.column.at', helpKey: 'journal.column.at.help', emphasis: 'primary' },
        {
            key: 'request',
            labelKey: 'journal.column.request',
            helpKey: 'journal.column.request.help',
            emphasis: 'secondary'
        },
        { key: 'account', labelKey: 'journal.column.account', helpKey: 'journal.column.account.help', custom: true },
        { key: 'job', labelKey: 'journal.column.job', helpKey: 'journal.column.job.help' },
        { key: 'status', labelKey: 'journal.column.status', helpKey: 'journal.column.status.help', align: 'end' },
        { key: 'outcome', labelKey: 'journal.column.outcome', helpKey: 'journal.column.outcome.help', custom: true },
        { key: 'duration', labelKey: 'journal.column.duration', helpKey: 'journal.column.duration.help', align: 'end' },
        { key: 'error', labelKey: 'journal.column.error', helpKey: 'journal.column.error.help', desktopOnly: true }
    ];

    const jobText = (job: string) => (hasMessage(`journal.job.${job}`, lang) ? t(`journal.job.${job}`, lang) : job);
    const statusText = (status: number) => (status === 0 ? t('journal.status.none', lang) : String(status));
    const requestText = (entry: TJournalEntry) => `${entry.method} ${entry.path}`;

    const rows = $derived<IEntryRow[]>(
        (data.list?.entries ?? []).map((entry) => ({
            id: String(entry.id),
            entry,
            href: addressWith(page.url, { entry: String(entry.id) }, { keep: ['page'] }),
            cells: {
                at: formatDateTime(entry.at, lang),
                request: requestText(entry),
                job: jobText(entry.job),
                status: statusText(entry.status),
                duration: t('journal.duration', lang, { ms: entry.durationMs }),
                error: entry.errorCode ?? null
            },
            attributes: { 'data-entry-id': String(entry.id), 'data-outcome': entry.outcome }
        }))
    );

    const markOf = (code: string) => data.marketplaces.find((marketplace) => marketplace.code === code)?.mark;

    const chips = $derived(
        filterChips(page.url, [
            {
                key: 'account',
                label: data.filter.account
                    ? t('journal.chip.account', lang, { value: accountNameOf(data.accounts, data.filter.account) })
                    : null
            },
            {
                key: 'outcome',
                label: data.filter.outcome
                    ? t('journal.chip.outcome', lang, { value: t(`journal.outcome.${data.filter.outcome}`, lang) })
                    : null
            },
            {
                key: 'from',
                label: data.filter.from
                    ? t('journal.chip.from', lang, { value: formatIsoDay(data.filter.from, lang) })
                    : null
            },
            {
                key: 'to',
                label: data.filter.to ? t('journal.chip.to', lang, { value: formatIsoDay(data.filter.to, lang) }) : null
            }
        ])
    );

    /** Every fact of one request, for its details beside the list; a fact it does not have is left out. */
    function entryFacts(entry: TJournalEntry) {
        const facts = [
            { key: 'at', label: t('journal.column.at', lang), value: formatDateTime(entry.at, lang) },
            { key: 'account', label: t('journal.column.account', lang), value: entry.accountName ?? '' },
            { key: 'job', label: t('journal.column.job', lang), value: jobText(entry.job) },
            {
                key: 'request',
                label: t('journal.column.request', lang),
                value: requestText(entry),
                help: t('journal.column.request.help', lang)
            },
            {
                key: 'status',
                label: t('journal.column.status', lang),
                value: statusText(entry.status),
                help: t('journal.column.status.help', lang)
            },
            {
                key: 'duration',
                label: t('journal.column.duration', lang),
                value: t('journal.duration', lang, { ms: entry.durationMs })
            },
            { key: 'errorCode', label: t('journal.column.error', lang), value: entry.errorCode ?? '' },
            { key: 'errorText', label: t('journal.field.errorText', lang), value: entry.errorText ?? '' }
        ];
        return facts.filter((fact) => fact.value !== '');
    }

    const any = t('app.list.any', lang);
</script>

<Stack as="section" dataTest="journal">
    <PageHeading
        title={t('journal.title', lang)}
        help={t('journal.help', lang)}
        headingTestId="journal-heading"
        helpTestId="journal-help"
    />

    {#if data.refusal}
        <NoteBox tone="error" role="alert" dataTest="journal-refusal">{t(refusalKey(data.refusal), lang)}</NoteBox>
        <div>
            <Button variant="secondary" dataTest="journal-retry" onclick={() => invalidateAll()}>
                {t('common.retry', lang)}
            </Button>
        </div>
    {:else}
        <FilterSection {lang} id="journal-filter" {chips} resetHref={listPath} help={t('journal.filter.help', lang)}>
            <GuardedForm
                method="GET"
                id="journal-filter-form"
                superform={{ enhance, submitting }}
                dataTest="journal-filter-form"
            >
                <Stack>
                    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
                        <Field
                            label={t('journal.filter.account', lang)}
                            help={t('journal.filter.account.help', lang)}
                            for="journal-filter-account"
                        >
                            <SelectBox
                                id="journal-filter-account"
                                name="account"
                                bind:value={$filters.account}
                                blank={any}
                                options={accountOptions(data.accounts)}
                                dataTest="journal-filter-account"
                            />
                        </Field>
                        <Field
                            label={t('journal.filter.outcome', lang)}
                            help={t('journal.filter.outcome.help', lang)}
                            for="journal-filter-outcome"
                        >
                            <SelectBox
                                id="journal-filter-outcome"
                                name="outcome"
                                bind:value={$filters.outcome}
                                blank={any}
                                options={JOURNAL_OUTCOMES.map((outcome) => ({
                                    value: outcome,
                                    label: t(`journal.outcome.${outcome}`, lang)
                                }))}
                                dataTest="journal-filter-outcome"
                            />
                        </Field>
                        <DateRangeFields
                            bind:from={$filters.from}
                            bind:to={$filters.to}
                            fromLabel={t('journal.filter.from', lang)}
                            toLabel={t('journal.filter.to', lang)}
                            fromHelp={t('journal.filter.from.help', lang)}
                            errors={{ to: tError($filterErrors.to, lang) || null }}
                            testPrefix="journal-filter"
                        />
                    </div>
                    <FormButtons>
                        <Button type="submit" variant="secondary" dataTest="journal-filter-apply">
                            {t('orders.filter.apply', lang)}
                        </Button>
                    </FormButtons>
                </Stack>
            </GuardedForm>
        </FilterSection>

        {#if rows.length === 0}
            {#if chips.length > 0}
                <EmptyState
                    message={t('journal.empty.filtered', lang, { filters: filtersInWords(chips, lang) })}
                    dataTest="journal-empty-filtered"
                />
            {:else}
                <EmptyState message={t('journal.empty.none', lang)} dataTest="journal-empty" />
            {/if}
        {:else}
            <DataList {lang} {columns} {rows} legend dataTest="journal-list">
                {#snippet cell(row: IEntryRow, column: IDataListColumn<IEntryRow>)}
                    {#if column.key === 'account'}
                        <span class="inline-flex flex-col">
                            {#if row.entry.marketplace}
                                <MarketplaceMark
                                    {lang}
                                    code={row.entry.marketplace}
                                    mark={markOf(row.entry.marketplace)}
                                    dataTest="journal-list-mark"
                                />
                            {/if}
                            <span class="text-xs text-ink-600">{row.entry.accountName ?? '—'}</span>
                        </span>
                    {:else if column.key === 'outcome'}
                        <JournalOutcomeBadge {lang} outcome={row.entry.outcome} testId="journal-list-outcome" />
                    {/if}
                {/snippet}
            </DataList>
            {#if data.list}
                <Pagination
                    {lang}
                    page={data.list.page}
                    perPage={data.list.pageSize}
                    total={data.list.total}
                    dataTest="journal-pagination"
                    onNavigate={(next) => goto(addressWith(page.url, { page: String(next) }, { keep: ['page'] }))}
                />
            {/if}
        {/if}
    {/if}

    <RowDetailsPanel
        {lang}
        param="entry"
        fromLoad={data.opened}
        idOf={(entry: TJournalEntry) => String(entry.id)}
        pick={(loaded) => loaded.opened as TJournalEntry | null | undefined}
        listHref={addressWith(page.url, { entry: null }, { keep: ['page'] })}
        title={(entry: TJournalEntry) => t('journal.details.title', lang, { request: requestText(entry) })}
        dataTest="journal-details"
    >
        {#snippet children(entry: TJournalEntry)}
            <Stack>
                <div>
                    <JournalOutcomeBadge {lang} outcome={entry.outcome} testId="journal-entry-outcome" />
                </div>
                <FactList items={entryFacts(entry)} dataTest="journal-entry-facts" />
            </Stack>
        {/snippet}
    </RowDetailsPanel>
</Stack>

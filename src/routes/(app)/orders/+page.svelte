<script lang="ts">
    import { goto, invalidateAll } from '$app/navigation';
    import { page } from '$app/state';
    import { refusalKey } from '$lib/api/refusal-key';
    import MarketplaceMark from '$lib/components/marketplaces/MarketplaceMark.svelte';
    import DeadlineCell from '$lib/components/orders/DeadlineCell.svelte';
    import OrderDetails from '$lib/components/orders/OrderDetails.svelte';
    import OrderStateBadge from '$lib/components/orders/OrderStateBadge.svelte';
    import RefreshButton from '$lib/components/orders/RefreshButton.svelte';
    import SchemeBadge from '$lib/components/orders/SchemeBadge.svelte';
    import type { TOrder, TOrderDetails } from '$lib/components/orders/types';
    import Button from '$lib/components/ui/Button.svelte';
    import DataList from '$lib/components/ui/DataList.svelte';
    import DateRangeFields from '$lib/components/ui/DateRangeFields.svelte';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
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
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import type { IDataListColumn, IDataListRow } from '$lib/components/ui/types';
    import { pageSearch } from '$lib/forms/page-search.svelte';
    import { tError } from '$lib/i18n/form-error';
    import { formatDateTime, formatIsoDay } from '$lib/i18n/format-date';
    import { getLocale, localizePath, t } from '$lib/i18n/i18n';
    import { filtersInWords } from '$lib/lists/filter-words';
    import { addressWith, filterChips } from '$lib/lists/list-address';
    import { accountNameOf, accountOptions } from '$lib/marketplaces/account-options';
    import { productText } from '$lib/orders/product-text';
    import { ORDER_FILTER_ID, ORDER_SCHEMES, ORDER_STATES, orderFilterSchema } from '$lib/validation/list-filters';
    import { onDestroy } from 'svelte';
    import type { PageData } from './$types';

    /**
     * «Заказы»: every order of every account and scheme in one list (`DataList`: a table on a computer, cards on a phone), newest
     * first, 50 a page. The filters live in the address (`FilterSection`, a search form that changes the address), so a filtered
     * list can be opened again by its link; a row opens the order's details beside the list (`RowDetailsPanel`, `?order=<id>`).
     * «Обновить» reads the orders at once, at most once in 5 minutes (`RefreshButton`); after a press the page reads its list
     * again a few seconds later, when the first answers are in. Every empty list says why it is empty and what to do.
     */
    let { data }: { data: PageData } = $props();

    const lang = getLocale();
    const listPath = localizePath('/orders', lang);
    /** The working states: an account in one of them is read by «Обновить». */
    const READ_STATES = ['working', 'key_expiring', 'no_connection'];
    /** When the page reads its list again after «Обновить»: the jobs answer within seconds. */
    const REREAD_AFTER_MS = [3_000, 10_000];

    const search = pageSearch(orderFilterSchema, () => ({ ...data.filter }), ORDER_FILTER_ID);
    const { form: filters, errors: filterErrors, enhance, submitting } = search;

    interface IOrderRow extends IDataListRow {
        order: TOrder;
    }

    const columns: IDataListColumn<IOrderRow>[] = [
        { key: 'number', labelKey: 'orders.column.number', helpKey: 'orders.column.number.help', emphasis: 'primary' },
        {
            key: 'product',
            labelKey: 'orders.column.product',
            helpKey: 'orders.column.product.help',
            emphasis: 'secondary'
        },
        {
            key: 'marketplace',
            labelKey: 'orders.column.marketplace',
            helpKey: 'orders.column.marketplace.help',
            custom: true
        },
        { key: 'scheme', labelKey: 'orders.column.scheme', helpKey: 'orders.column.scheme.help', custom: true },
        { key: 'warehouse', labelKey: 'orders.column.warehouse', helpKey: 'orders.column.warehouse.help' },
        { key: 'created', labelKey: 'orders.column.created', helpKey: 'orders.column.created.help' },
        { key: 'deadline', labelKey: 'orders.column.deadline', helpKey: 'orders.column.deadline.help', custom: true },
        { key: 'state', labelKey: 'orders.column.state', helpKey: 'orders.column.state.help', custom: true }
    ];

    const rows = $derived<IOrderRow[]>(
        (data.list?.orders ?? []).map((order) => ({
            id: String(order.id),
            order,
            href: addressWith(page.url, { order: String(order.id) }, { keep: ['page'] }),
            cells: {
                number: order.number,
                product: productText(order, lang) || null,
                warehouse: order.warehouse ?? null,
                created: formatDateTime(order.createdAt, lang)
            },
            attributes: { 'data-order-id': String(order.id), 'data-state': order.state, 'data-scheme': order.scheme }
        }))
    );

    const markOf = (code: string) => data.marketplaces.find((marketplace) => marketplace.code === code)?.mark;
    const adapterOf = (code: string) => data.marketplaces.find((marketplace) => marketplace.code === code);

    const chips = $derived(
        filterChips(page.url, [
            {
                key: 'account',
                label: data.filter.account
                    ? t('orders.chip.account', lang, { value: accountNameOf(data.accounts, data.filter.account) })
                    : null
            },
            {
                key: 'scheme',
                label: data.filter.scheme
                    ? t('orders.chip.scheme', lang, { value: t(`orders.scheme.${data.filter.scheme}`, lang) })
                    : null
            },
            {
                key: 'state',
                label: data.filter.state
                    ? t('orders.chip.state', lang, { value: t(`orders.state.${data.filter.state}`, lang) })
                    : null
            },
            {
                key: 'from',
                label: data.filter.from
                    ? t('orders.chip.from', lang, { value: formatIsoDay(data.filter.from, lang) })
                    : null
            },
            {
                key: 'to',
                label: data.filter.to ? t('orders.chip.to', lang, { value: formatIsoDay(data.filter.to, lang) }) : null
            },
            { key: 'q', label: data.filter.q ? t('orders.chip.q', lang, { value: data.filter.q }) : null }
        ])
    );

    const canRefresh = $derived(data.accounts.some((account) => READ_STATES.includes(account.state)));

    let rereads: ReturnType<typeof setTimeout>[] = [];
    function refreshed(): void {
        rereads = REREAD_AFTER_MS.map((delay) => setTimeout(() => void invalidateAll(), delay));
    }
    onDestroy(() => rereads.forEach((timer) => clearTimeout(timer)));

    const any = t('app.list.any', lang);
</script>

<Stack as="section" dataTest="orders">
    <PageHeading
        title={t('orders.title', lang)}
        help={t('orders.help', lang)}
        headingTestId="orders-heading"
        helpTestId="orders-help"
    >
        {#snippet actions()}
            <RefreshButton {lang} refresh={data.refresh} {canRefresh} onRefreshed={refreshed} />
        {/snippet}
    </PageHeading>

    {#if data.refusal}
        <NoteBox tone="error" role="alert" dataTest="orders-refusal">{t(refusalKey(data.refusal), lang)}</NoteBox>
        <div>
            <Button variant="secondary" dataTest="orders-retry" onclick={() => invalidateAll()}>
                {t('common.retry', lang)}
            </Button>
        </div>
    {:else if data.accounts.length === 0 && rows.length === 0}
        <EmptyState message={t('orders.empty.noAccounts', lang)} dataTest="orders-empty-no-accounts">
            <Button href={localizePath('/marketplaces', lang)} variant="secondary" dataTest="orders-open-marketplaces">
                {t('orders.empty.openMarketplaces', lang)}
            </Button>
        </EmptyState>
    {:else}
        <FilterSection {lang} id="orders-filter" {chips} resetHref={listPath} help={t('orders.filter.help', lang)}>
            <GuardedForm
                method="GET"
                id="orders-filter-form"
                superform={{ enhance, submitting }}
                dataTest="orders-filter-form"
            >
                <Stack>
                    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
                        <Field
                            label={t('orders.filter.account', lang)}
                            help={t('orders.filter.account.help', lang)}
                            for="orders-filter-account"
                        >
                            <SelectBox
                                id="orders-filter-account"
                                name="account"
                                bind:value={$filters.account}
                                blank={any}
                                options={accountOptions(data.accounts)}
                                dataTest="orders-filter-account"
                            />
                        </Field>
                        <Field
                            label={t('orders.filter.scheme', lang)}
                            help={t('orders.column.scheme.help', lang)}
                            for="orders-filter-scheme"
                        >
                            <SelectBox
                                id="orders-filter-scheme"
                                name="scheme"
                                bind:value={$filters.scheme}
                                blank={any}
                                options={ORDER_SCHEMES.map((scheme) => ({
                                    value: scheme,
                                    label: t(`orders.scheme.${scheme}`, lang)
                                }))}
                                dataTest="orders-filter-scheme"
                            />
                        </Field>
                        <Field
                            label={t('orders.filter.state', lang)}
                            help={t('orders.column.state.help', lang)}
                            for="orders-filter-state"
                        >
                            <SelectBox
                                id="orders-filter-state"
                                name="state"
                                bind:value={$filters.state}
                                blank={any}
                                options={ORDER_STATES.map((state) => ({
                                    value: state,
                                    label: t(`orders.state.${state}`, lang)
                                }))}
                                dataTest="orders-filter-state"
                            />
                        </Field>
                        <DateRangeFields
                            bind:from={$filters.from}
                            bind:to={$filters.to}
                            fromLabel={t('orders.filter.from', lang)}
                            toLabel={t('orders.filter.to', lang)}
                            fromHelp={t('orders.filter.from.help', lang)}
                            errors={{ to: tError($filterErrors.to, lang) || null }}
                            testPrefix="orders-filter"
                        />
                        <Field
                            label={t('orders.filter.q', lang)}
                            help={t('orders.filter.q.help', lang)}
                            for="orders-filter-q"
                            error={tError($filterErrors.q, lang)}
                        >
                            <TextBox
                                id="orders-filter-q"
                                type="search"
                                name="q"
                                bind:value={$filters.q}
                                invalid={Boolean($filterErrors.q)}
                                dataTest="orders-filter-q"
                            />
                        </Field>
                    </div>
                    <FormButtons>
                        <Button type="submit" variant="secondary" dataTest="orders-filter-apply">
                            {t('orders.filter.apply', lang)}
                        </Button>
                    </FormButtons>
                </Stack>
            </GuardedForm>
        </FilterSection>

        {#if rows.length === 0}
            {#if chips.length > 0}
                <EmptyState
                    message={t('orders.empty.filtered', lang, { filters: filtersInWords(chips, lang) })}
                    dataTest="orders-empty-filtered"
                />
            {:else}
                <EmptyState message={t('orders.empty.none', lang)} dataTest="orders-empty" />
            {/if}
        {:else}
            <DataList {lang} {columns} {rows} legend dataTest="orders-list">
                {#snippet cell(row: IOrderRow, column: IDataListColumn<IOrderRow>)}
                    {#if column.key === 'marketplace'}
                        <span class="inline-flex flex-col">
                            <MarketplaceMark
                                {lang}
                                code={row.order.marketplace}
                                mark={markOf(row.order.marketplace)}
                                dataTest="orders-list-mark"
                            />
                            <span class="text-xs text-ink-600">{row.order.accountName}</span>
                        </span>
                    {:else if column.key === 'scheme'}
                        <SchemeBadge {lang} scheme={row.order.scheme} />
                    {:else if column.key === 'deadline'}
                        <DeadlineCell {lang} deadlineAt={row.order.deadlineAt} state={row.order.state} />
                    {:else if column.key === 'state'}
                        <OrderStateBadge
                            {lang}
                            state={row.order.state}
                            cancelledBy={row.order.cancelledBy}
                            marketplaceStatus={row.order.marketplaceStatus}
                            testId="orders-list-state"
                        />
                    {/if}
                {/snippet}
            </DataList>
        {/if}
        {#if data.list && rows.length > 0}
            <Pagination
                {lang}
                page={data.list.page}
                perPage={data.list.pageSize}
                total={data.list.total}
                dataTest="orders-pagination"
                onNavigate={(next) => goto(addressWith(page.url, { page: String(next) }, { keep: ['page'] }))}
            />
        {/if}
    {/if}

    <RowDetailsPanel
        {lang}
        param="order"
        fromLoad={data.opened}
        idOf={(details: TOrderDetails) => String(details.order.id)}
        pick={(loaded) => loaded.opened as TOrderDetails | null | undefined}
        listHref={addressWith(page.url, { order: null }, { keep: ['page'] })}
        title={(details: TOrderDetails) => t('orders.details.title', lang, { number: details.order.number })}
        dataTest="order-details"
    >
        {#snippet children(details: TOrderDetails)}
            <OrderDetails {lang} {details} marketplace={adapterOf(details.order.marketplace)} />
        {/snippet}
    </RowDetailsPanel>
</Stack>

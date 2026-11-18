<script lang="ts">
    import MarketplaceMark from '$lib/components/marketplaces/MarketplaceMark.svelte';
    import type { TMarketplace } from '$lib/components/marketplaces/types';
    import FactList from '$lib/components/ui/FactList.svelte';
    import OrderExchangeDetails from '$lib/components/exchange/OrderExchangeDetails.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { formatDateTime } from '$lib/i18n/format-date';
    import { hasMessage, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { deadlineMatters } from '$lib/orders/deadline';
    import DeadlineCell from './DeadlineCell.svelte';
    import OrderLines from './OrderLines.svelte';
    import OrderStateBadge from './OrderStateBadge.svelte';
    import SchemeBadge from './SchemeBadge.svelte';
    import type { TOrderDetails } from './types';

    /**
     * One order's details beside the order list (`GET /api/v1/orders/{id}`): the marketplace, the scheme and the state; the
     * deadline of an FBS order while it waits to be handed over (new or assembling); every field the order has — the field it lacks is left out, not drawn empty —
     * and, after them, its products with their quantities (`OrderLines`); the order a split order came from and the orders a
     * split order was split into; the marketplace's own words for the state as received; and the history of states, oldest first,
     * each in the list's words (with who cancelled) and with where the change came from. An FBS order that has an exchange with a fulfilment centre draws it last
     * (`OrderExchangeDetails`); the page is told (`onExchangeRetried`) when «Передать заново» was pressed, to read the order again.
     */
    interface IProps {
        lang: TLocale;
        details: TOrderDetails;
        /** The order's adapter, for its colour mark. */
        marketplace?: TMarketplace;
        /** «Передать заново» was pressed and accepted: the page reads the order again. */
        onExchangeRetried?: () => void;
    }

    let { lang, details, marketplace, onExchangeRetried }: IProps = $props();

    const order = $derived(details.order);

    const facts = $derived.by(() => {
        const items: { key: string; label: string; value: string; help?: string }[] = [];
        const add = (key: string, labelKey: string, value: string | undefined, helpKey?: string) => {
            if (value === undefined || value === '') return;
            items.push({ key, label: t(labelKey, lang), value, help: helpKey ? t(helpKey, lang) : undefined });
        };
        add('number', 'orders.column.number', order.number, 'orders.column.number.help');
        add('account', 'orders.field.account', order.accountName);
        add('basket', 'orders.field.basket', order.basket);
        add('warehouse', 'orders.column.warehouse', order.warehouse, 'orders.column.warehouse.help');
        add('parent', 'orders.field.parent', order.parentNumber, 'orders.field.parent.help');
        add('parts', 'orders.field.parts', details.parts.join(', '), 'orders.field.parts.help');
        add('createdAt', 'orders.column.created', formatDateTime(order.createdAt, lang), 'orders.column.created.help');
        add('updatedAt', 'orders.field.updatedAt', order.updatedAt ? formatDateTime(order.updatedAt, lang) : undefined);
        add(
            'marketplaceStatus',
            'orders.details.marketplaceStatus',
            [order.marketplaceStatus.seller, order.marketplaceStatus.marketplace].filter(Boolean).join(' / '),
            'orders.details.marketplaceStatus.help'
        );
        return items;
    });

    const sourceText = (source: string) =>
        hasMessage(`orders.source.${source}`, lang) ? t(`orders.source.${source}`, lang) : source;
</script>

<Stack dataTest="order-details">
    <div class="flex flex-wrap items-center gap-2">
        <MarketplaceMark {lang} code={order.marketplace} mark={marketplace?.mark} dataTest="order-mark" />
        <SchemeBadge {lang} scheme={order.scheme} testId="order-scheme" />
        <OrderStateBadge
            {lang}
            state={order.state}
            cancelledBy={order.cancelledBy}
            marketplaceStatus={order.marketplaceStatus}
            size="md"
            testId="order-state"
        />
    </div>
    {#if order.deadlineAt && deadlineMatters(order.state)}
        <p class="text-sm">
            <span class="text-ink-600">{t('orders.column.deadline', lang)}:</span>
            <DeadlineCell {lang} deadlineAt={order.deadlineAt} state={order.state} testId="order-deadline" />
        </p>
    {/if}
    <FactList items={facts} dataTest="order-facts" />
    <OrderLines {lang} lines={order.lines} />
    {#if details.events.length > 0}
        <div>
            <HeadingWithHelp text={t('orders.details.history', lang)} level="h3" look="section" />
            <ol class="mt-2 flex list-none flex-col gap-2 text-sm" data-test="order-history">
                {#each details.events as event, index (index)}
                    <li class="flex flex-wrap items-center gap-2">
                        <span class="text-ink-600 tabular-nums">{formatDateTime(event.at, lang)}</span>
                        <OrderStateBadge
                            {lang}
                            state={event.state}
                            cancelledBy={event.cancelledBy}
                            marketplaceStatus={event.marketplaceStatus}
                            testId="order-history-state"
                        />
                        <span class="text-ink-700">{sourceText(event.source)}</span>
                    </li>
                {/each}
            </ol>
        </div>
    {/if}
    {#if details.exchange}
        <OrderExchangeDetails
            {lang}
            orderId={order.id}
            orderNumber={order.number}
            details={details.exchange}
            onRetried={onExchangeRetried}
        />
    {/if}
</Stack>

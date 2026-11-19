<script lang="ts">
    import HelpTooltip from '$lib/components/ui/HelpTooltip.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import OrderStateBadge from './OrderStateBadge.svelte';
    import type { TOrder } from './types';

    /**
     * The state of an order in the order list: its badge (`OrderStateBadge`) and, under it, for an order the marketplace split into
     * parts, the line «Разделён на части» with the parts' numbers and a «?» that says where its products went. Such an order reads
     * «Отменён» at the marketplace, and the line says why.
     */
    interface IProps {
        lang: TLocale;
        order: Pick<TOrder, 'state' | 'cancelledBy' | 'marketplaceStatus' | 'parts'>;
        /** The badge is `<testId>`, the parts' line `<testId>-parts`, its «?» `<testId>-parts-help`. */
        testId: string;
    }

    let { lang, order, testId }: IProps = $props();

    const parts = $derived(order.parts ?? []);
</script>

<span class="inline-flex flex-col items-start gap-1">
    <OrderStateBadge
        {lang}
        state={order.state}
        cancelledBy={order.cancelledBy}
        marketplaceStatus={order.marketplaceStatus}
        {testId}
    />
    {#if parts.length > 0}
        <span class="inline-flex items-center text-xs text-ink-600" data-test="{testId}-parts">
            {t('orders.field.parts', lang)}: {parts.join(', ')}
            <HelpTooltip
                tooltip={t('orders.field.parts.help', lang)}
                ariaLabel={t('orders.field.parts.help', lang)}
                testId="{testId}-parts-help"
            />
        </span>
    {/if}
</span>

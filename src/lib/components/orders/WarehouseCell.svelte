<script lang="ts">
    import HelpTooltip from '$lib/components/ui/HelpTooltip.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { sellerPacks } from '$lib/orders/scheme';
    import type { TOrderScheme } from './types';

    /**
     * The warehouse of an order in the list: the warehouse's name. An order the seller packs (FBS, Express) that the marketplace
     * gave no warehouse — the shop has several warehouses and the marketplace did not say which one — says «Склад не определён»
     * with a «?» that explains why, and the marketplace's own text of the warehouse beside it when the order carries one (`Order.warehouseText`), with a «?» that says why it is kept. An order the marketplace packs has no warehouse of the seller's and shows a quiet dash.
     */
    interface IProps {
        lang: TLocale;
        scheme: TOrderScheme;
        warehouse?: string;
        /** The marketplace's own text of the warehouse, given when the warehouse could not be told by its number. */
        warehouseText?: string;
        /** The cell is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { lang, scheme, warehouse, warehouseText, testId }: IProps = $props();

    const unknown = $derived(warehouse === undefined && sellerPacks(scheme));
</script>

<span class="inline-flex items-center gap-1" data-test={testId}>
    {#if warehouse !== undefined}
        {warehouse}
    {:else if unknown}
        <span class="text-ink-600">{t('orders.warehouse.unknown', lang)}</span>
        <HelpTooltip
            tooltip={t('orders.warehouse.unknown.help', lang)}
            ariaLabel={t('orders.warehouse.unknown.help', lang)}
            testId={testId ? `${testId}-help` : undefined}
        />
        {#if warehouseText}
            <span class="break-words text-ink-800" data-test={testId ? `${testId}-text` : undefined}>
                {warehouseText}
            </span>
            <HelpTooltip
                tooltip={t('orders.warehouse.text.help', lang)}
                ariaLabel={t('orders.warehouse.text.help', lang)}
                testId={testId ? `${testId}-text-help` : undefined}
            />
        {/if}
    {:else}
        —
    {/if}
</span>

<script lang="ts">
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TOrderLine } from './types';

    /**
     * The product cell of the order list: the first product of the order in one line (its name, kind, brand and size when the
     * marketplace gave them, then the seller's article and the first barcode, joined by « · »; what was not given is left out),
     * how many units of it when that is more than one («× 2»), and, when the order has more products, how many more («ещё
     * товаров: 2»), because the list has room for one. All the products are in the order's details (`OrderLines`). An order whose
     * product the marketplace told nothing about shows a quiet dash.
     */
    interface IProps {
        lang: TLocale;
        /** The order's products, at least one (`Order.lines`). */
        lines: TOrderLine[];
        /** The cell is `<testId>`, the first product `<testId>-first`, the quantity `<testId>-quantity`, the rest `<testId>-more`. */
        testId?: string;
    }

    let { lang, lines, testId }: IProps = $props();

    const first = $derived<TOrderLine | undefined>(lines[0]);
    const text = $derived.by(() => {
        if (first === undefined) return '';
        const parts = [
            first.name,
            first.kind,
            first.brand,
            first.size ? t('orders.product.size', lang, { size: first.size }) : undefined,
            first.article ? t('orders.product.article', lang, { article: first.article }) : undefined,
            first.barcodes[0] ? t('orders.product.barcode', lang, { barcode: first.barcodes[0] }) : undefined
        ];
        return parts.filter((part): part is string => Boolean(part)).join(' · ');
    });
    const quantity = $derived(first?.quantity ?? 1);
    const more = $derived(Math.max(lines.length - 1, 0));
</script>

<span data-test={testId}>
    <span data-test={testId ? `${testId}-first` : undefined}>{text === '' ? '—' : text}</span>
    {#if quantity > 1}
        <span class="font-medium whitespace-nowrap text-ink-800" data-test={testId ? `${testId}-quantity` : undefined}>
            {t('orders.product.quantity', lang, { quantity })}
        </span>
    {/if}
    {#if more > 0}
        <span class="block text-ink-600" data-test={testId ? `${testId}-more` : undefined}>
            {t('orders.product.more', lang, { count: more })}
        </span>
    {/if}
</span>

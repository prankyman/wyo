import type { TOrder } from '$lib/components/orders/types';
import { t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';

/**
 * The product of an order in one line of the order list: the kind, the brand and the size when the marketplace gave them, then the
 * seller's article and the first barcode, joined by « · ». What the marketplace did not give is left out.
 */
export function productText(order: TOrder, lang: TLocale): string {
    const parts = [
        order.kind,
        order.brand,
        order.size ? t('orders.product.size', lang, { size: order.size }) : undefined,
        order.article ? t('orders.product.article', lang, { article: order.article }) : undefined,
        order.barcodes[0] ? t('orders.product.barcode', lang, { barcode: order.barcodes[0] }) : undefined
    ];
    return parts.filter((part): part is string => Boolean(part)).join(' · ');
}

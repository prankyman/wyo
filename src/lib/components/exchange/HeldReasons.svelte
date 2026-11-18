<script lang="ts">
    import { messageParams } from '$lib/api/api';
    import { hasMessage, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { THeldReason } from './types';

    /**
     * The reasons the fulfilment centre holds an order for, each in a sentence of its own with the values it named (the
     * barcode, the article, the warehouse). A reason code wyo has no text for is worded by the general text that names the
     * code. A product the fulfilment centre could not find is named by whichever of three values it gave.
     */
    interface IProps {
        lang: TLocale;
        reasons: THeldReason[];
        /** The list is `<dataTest>`, each reason `<dataTest>-reason`. */
        dataTest: string;
    }

    let { lang, reasons, dataTest }: IProps = $props();

    /** The names of the three values that can name a product the fulfilment centre did not find, in the order they are tried. */
    const PRODUCT_VALUES = ['barcode', 'article', 'marketplace_item_id'] as const;

    function textOf(reason: THeldReason): string {
        const params = messageParams(reason.params) ?? {};
        let key = `exchange.held.${reason.code}`;
        if (reason.code === 'unknown_product') {
            const named = PRODUCT_VALUES.find((name) => params[name] !== undefined);
            if (named) key = `${key}.${named}`;
        }
        return hasMessage(key, lang) ? t(key, lang, params) : t('exchange.held.unknown', lang, { code: reason.code });
    }
</script>

<ul class="flex list-disc flex-col gap-1 pl-5 text-sm text-ink-800" data-test={dataTest}>
    {#each reasons as reason, index (index)}
        <li data-test="{dataTest}-reason">{textOf(reason)}</li>
    {/each}
</ul>

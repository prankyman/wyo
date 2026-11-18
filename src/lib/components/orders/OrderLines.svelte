<script lang="ts">
    import FactList from '$lib/components/ui/FactList.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { t } from '$lib/i18n/i18n';
    import { tPlural } from '$lib/i18n/plural';
    import type { TLocale } from '$lib/i18n/types';
    import type { TOrderLine } from './types';

    /**
     * An order's products with how many units of each, for the details beside the order list. Every product is a block of facts
     * — its name, article, barcodes, the marketplace's own number of it, kind, brand and size — and its quantity in words
     * («3 единицы товара»); a fact the marketplace did not give is left out, never drawn empty. An order of several products
     * numbers its blocks («Позиция 1 из 3»); an order of one draws no number. The exchange's details reuse this component.
     */
    interface IProps {
        lang: TLocale;
        /** The order's products, at least one (`Order.lines`). */
        lines: TOrderLine[];
        /** The block is `<testId>`; a product's facts are `<testId>-<n>-<fact>` (n from 1) and its number `<testId>-<n>-position`. */
        testId?: string;
    }

    let { lang, lines, testId = 'order-lines' }: IProps = $props();

    function factsOf(line: TOrderLine) {
        const items: { key: string; label: string; value: string; help?: string }[] = [];
        const add = (key: string, value: string | undefined, help?: string) => {
            if (value === undefined || value === '') return;
            items.push({ key, label: t(`orders.field.${key}`, lang), value, help: help ? t(help, lang) : undefined });
        };
        add('name', line.name);
        add('article', line.article);
        add('barcodes', line.barcodes.join(', '));
        add('productId', line.productId);
        add('kind', line.kind);
        add('brand', line.brand);
        add('size', line.size);
        add('quantity', tPlural('orders.field.quantity.value', line.quantity, lang), 'orders.field.quantity.help');
        return items;
    }
</script>

<Stack dataTest={testId}>
    <HeadingWithHelp
        text={t('orders.lines.title', lang)}
        help={t('orders.lines.help', lang)}
        level="h3"
        look="section"
        testId="{testId}-help"
    />
    {#each lines as line, index (index)}
        <Stack>
            {#if lines.length > 1}
                <p class="text-xs font-medium text-ink-700" data-test="{testId}-{index + 1}-position">
                    {t('orders.lines.position', lang, { n: index + 1, total: lines.length })}
                </p>
            {/if}
            <FactList items={factsOf(line)} dataTest="{testId}-{index + 1}" />
        </Stack>
    {/each}
</Stack>

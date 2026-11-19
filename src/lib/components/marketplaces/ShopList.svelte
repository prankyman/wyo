<script lang="ts">
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import { hasMessage, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TShop } from './types';

    /**
     * The shops of one account, for a marketplace that keeps several shops under one key: each with its name, its number at the
     * marketplace and its scheme (`orders.scheme.<scheme in lower case>`, the marketplace's own word when the interface has no
     * words for it). A shop whose API the marketplace switched off says so, with a «?» that explains what wyo then does: it does not
     * read that shop's orders.
     */
    interface IProps {
        lang: TLocale;
        shops: TShop[];
        /** The block is `<testId>`, its heading `<testId>-heading`, its «?» `<testId>-help`, its list `<testId>-list`. */
        testId?: string;
    }

    let { lang, shops, testId = 'shops' }: IProps = $props();

    const schemeKey = (shop: TShop) => `orders.scheme.${shop.scheme.toLowerCase()}`;
</script>

<Stack dataTest={testId}>
    <HeadingWithHelp
        text={t('accounts.shops.title', lang)}
        help={t('accounts.shops.title.help', lang)}
        level="h3"
        look="section"
        testId="{testId}-help"
        headingTestId="{testId}-heading"
    />
    <ul class="flex list-none flex-col gap-2 text-sm" data-test="{testId}-list">
        {#each shops as shop (shop.number)}
            <li class="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span class="font-medium break-words text-ink-900">{shop.name}</span>
                <span class="text-ink-600">{t('accounts.shops.number', lang, { number: shop.number })}</span>
                <StatusBadgeWithHelp
                    label={hasMessage(schemeKey(shop), lang) ? t(schemeKey(shop), lang) : shop.scheme}
                    tone="off"
                    help={hasMessage(`${schemeKey(shop)}.help`, lang) ? t(`${schemeKey(shop)}.help`, lang) : undefined}
                    testId="{testId}-scheme-{shop.number}"
                />
                {#if shop.apiOff}
                    <StatusBadgeWithHelp
                        label={t('accounts.shops.apiOff', lang)}
                        tone="warn"
                        help={t('accounts.shops.apiOff.help', lang)}
                        testId="{testId}-api-off-{shop.number}"
                    />
                {/if}
            </li>
        {/each}
    </ul>
</Stack>

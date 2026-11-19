<script lang="ts">
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * The mark of a test order (`Order.test`): an order the seller made in the marketplace's cabinet to try the whole walk. It
     * is drawn beside the scheme in the order list and in the order's details, with a «?» that says what makes it different: no
     * buyer, no money, and no place in any number of the home page. A badge of one kind of status, drawn through
     * `StatusBadgeWithHelp`.
     */
    interface IProps {
        lang: TLocale;
        /** The badge is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { lang, testId }: IProps = $props();
</script>

<StatusBadgeWithHelp label={t('orders.test.label', lang)} tone="info" help={t('orders.test.help', lang)} {testId} />

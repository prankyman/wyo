<script lang="ts">
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TOrder, TOrderState } from './types';

    /**
     * The connector's own state of an order (`api/wyo.yaml`, `OrderState`), the same for every marketplace: a badge in the tone of
     * the state and a «?» that says what it means. A cancelled order says who cancelled it. A state the table of states does not
     * hold (`unknown`) shows the marketplace's own words as received, in the attention tone, so it is noticed.
     */
    interface IProps {
        lang: TLocale;
        state: TOrderState;
        cancelledBy?: TOrder['cancelledBy'];
        marketplaceStatus: TOrder['marketplaceStatus'];
        size?: 'sm' | 'md';
        /** The badge is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { lang, state, cancelledBy, marketplaceStatus, size = 'sm', testId }: IProps = $props();

    const TONE: Readonly<Record<TOrderState, TTone>> = {
        new: 'info',
        assembling: 'wait',
        in_delivery: 'wait',
        received: 'ok',
        cancelled: 'off',
        unknown: 'warn'
    };

    const label = $derived.by(() => {
        if (state === 'cancelled' && cancelledBy !== undefined) return t(`orders.state.cancelled.${cancelledBy}`, lang);
        if (state === 'unknown') {
            const words = [marketplaceStatus.seller, marketplaceStatus.marketplace].filter(Boolean).join(' / ');
            if (words !== '') return words;
        }
        return t(`orders.state.${state}`, lang);
    });
</script>

<StatusBadgeWithHelp {label} tone={TONE[state]} help={t(`orders.state.${state}.help`, lang)} {size} {testId} />

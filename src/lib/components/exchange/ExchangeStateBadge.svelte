<script lang="ts">
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TExchangeState, TOrderExchange } from './types';

    /**
     * The exchange's own state of an order (`api/wyo.yaml`, `ExchangeState`), shown beside the marketplace's state: a badge in the
     * tone of the state and a «?» that says what it means. «проблема» in the stop tone replaces the state while a problem is
     * present; an order that needs the owner is drawn in the attention tone. The «?» also names the reason a not-sent order is
     * not sent and what the owner can do when the order needs him, so the badge alone says all of it.
     */
    interface IProps {
        lang: TLocale;
        exchange: Pick<TOrderExchange, 'state' | 'reason' | 'attention' | 'problem'>;
        size?: 'sm' | 'md';
        /** The badge is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { lang, exchange, size = 'sm', testId }: IProps = $props();

    const TONE: Readonly<Record<TExchangeState, TTone>> = {
        not_sent: 'off',
        sent: 'wait',
        held: 'warn',
        reserved: 'wait',
        label_sent: 'wait',
        packed: 'wait',
        codes_checking: 'wait',
        codes_accepted: 'ok',
        handed_over: 'ok',
        cancelled: 'off'
    };

    const key = $derived(exchange.problem ? 'exchange.state.problem' : `exchange.state.${exchange.state}`);
    const tone = $derived<TTone>(exchange.problem ? 'stop' : exchange.attention ? 'warn' : TONE[exchange.state]);
    const help = $derived(
        [
            t(`${key}.help`, lang),
            exchange.reason ? t(`exchange.reason.${exchange.reason}`, lang) : '',
            exchange.attention ? t(`exchange.attention.${exchange.attention}`, lang) : ''
        ]
            .filter((part) => part !== '')
            .join(' ')
    );
</script>

<StatusBadgeWithHelp label={t(key, lang)} {tone} {help} {size} {testId} />

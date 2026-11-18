<script lang="ts">
    import { api } from '$lib/api/api';
    import BlockedButton from '$lib/components/ui/BlockedButton.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import { ApiAction } from '$lib/forms/api-action.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { minutesUntil } from '$lib/orders/refresh';
    import { ACCOUNT_REFUSALS } from '$lib/marketplaces/refusals';
    import type { TRefreshState } from './types';

    /**
     * «Обновить» on the order list: starts the three order jobs of every working account at once (`POST /api/v1/sync/refresh`),
     * and is answered at most once in 5 minutes — the marketplaces limit the requests, and the requests spent here are missing
     * from the readings on schedule. Until the wyo server allows the next press the button stays greyed out with the sentence
     * «Обновить можно через N мин.» under it (`BlockedButton`); the minutes count down by themselves. The time comes from the
     * page (`GET /api/v1/sync/refresh`), from the answer of a press, or from the refusal `too_soon` when another browser pressed
     * first.
     */
    interface IProps {
        lang: TLocale;
        /** The refresh state the page read (a prop named `state` would shadow the `$state` rune). */
        refresh: TRefreshState;
        /** Some account works; without one there is nothing to read, and the button says so. */
        canRefresh: boolean;
        /** Told the new state after a press that started the reading, so the page can read its list again. */
        onRefreshed?: (state: TRefreshState) => void;
    }

    let { lang, refresh, canRefresh, onRefreshed }: IProps = $props();

    /** How often the minutes left are counted again. */
    const TICK_MS = 15_000;

    // The page's state is where the button starts; a press moves it on.
    // svelte-ignore state_referenced_locally
    let next = $state(refresh.nextAllowedAt);
    let now = $state(Date.now());
    $effect(() => {
        const timer = setInterval(() => (now = Date.now()), TICK_MS);
        return () => clearInterval(timer);
    });

    const minutes = $derived(minutesUntil(next, new Date(now)));
    const reason = $derived.by(() => {
        if (!canRefresh) return t('orders.refresh.noAccounts', lang);
        return minutes > 0 ? t('orders.refresh.wait', lang, { minutes }) : null;
    });

    const action = new ApiAction();

    async function press(): Promise<void> {
        const result = await action.run(
            () => api.post<TRefreshState>('/api/v1/sync/refresh', {}),
            'orders.refresh.started',
            ACCOUNT_REFUSALS
        );
        if (!result) return;
        now = Date.now();
        if (result.ok) {
            next = result.data.nextAllowedAt;
            onRefreshed?.(result.data);
        } else if (result.refusal.code === 'too_soon' && typeof result.refusal.params?.nextAllowedAt === 'string') {
            next = result.refusal.params.nextAllowedAt;
        }
    }
</script>

<div>
    <BlockedButton
        variant="secondary"
        pending={action.pending}
        {reason}
        reasonTestId="orders-refresh-reason"
        help={t('orders.refresh.help', lang)}
        helpLabel={t('orders.refresh', lang)}
        helpTestId="orders-refresh-help"
        dataTest="orders-refresh"
        onclick={press}
    >
        {t('orders.refresh', lang)}
    </BlockedButton>
    <FormAnswer saved={action.saved} error={action.error} dataTest="orders-refresh-answer" />
</div>

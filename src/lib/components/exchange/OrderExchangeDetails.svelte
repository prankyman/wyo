<script lang="ts">
    import { api } from '$lib/api/api';
    import ButtonWithHelp from '$lib/components/ui/ButtonWithHelp.svelte';
    import FactList from '$lib/components/ui/FactList.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { ApiAction } from '$lib/forms/api-action.svelte';
    import { formatDateTime } from '$lib/i18n/format-date';
    import { hasMessage, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import CodeVerdictList from './CodeVerdictList.svelte';
    import ExchangeStateBadge from './ExchangeStateBadge.svelte';
    import HeldReasons from './HeldReasons.svelte';
    import ParcelList from './ParcelList.svelte';
    import ProblemLine from './ProblemLine.svelte';
    import type { TOrderExchangeDetails } from './types';

    /**
     * The part of an order's details about its exchange with the fulfilment centre (`OrderDetails.exchange`): the exchange's
     * state, why the order is not sent or what it needs from the owner, the fulfilment centre's number of the order, the order's parcels with the marketplace's number of each (`ParcelList`), the reasons
     * the fulfilment centre holds it for, the problem in wyo's words and the other party's, the marketplace's verdicts on the
     * order's codes, the fulfilment centre's answer to a cancellation passed on, and the history of the exchange, oldest first.
     * «Передать заново» repeats the step that failed (`POST /api/v1/orders/{id}/exchange/retry`) when the wyo server says it
     * may; the page is told (`onRetried`) so it reads the order again.
     */
    interface IProps {
        lang: TLocale;
        orderId: number;
        /** The marketplace's own number of the order, for `ParcelList` to tell a parcel that kept it from one the marketplace numbered. */
        orderNumber: string;
        details: TOrderExchangeDetails;
        onRetried?: () => void;
    }

    let { lang, orderId, orderNumber, details, onRetried }: IProps = $props();

    const retry = new ApiAction();

    async function press(): Promise<void> {
        const result = await retry.run(
            () => api.post(`/api/v1/orders/${orderId}/exchange/retry`, {}),
            'exchange.retry.started'
        );
        if (result?.ok) onRetried?.();
    }

    const facts = $derived(
        details.fcNumber
            ? [
                  {
                      key: 'fcNumber',
                      label: t('exchange.details.fcNumber', lang),
                      value: details.fcNumber,
                      help: t('exchange.details.fcNumber.help', lang)
                  }
              ]
            : []
    );

    const eventText = (code: string | undefined) =>
        code !== undefined && hasMessage(`exchange.event.${code}`, lang)
            ? t(`exchange.event.${code}`, lang)
            : t('exchange.event.unknown', lang, { code: code ?? '' });
</script>

<Stack dataTest="order-exchange">
    <HeadingWithHelp
        text={t('exchange.details.title', lang)}
        help={t('exchange.details.help', lang)}
        level="h3"
        look="section"
        testId="exchange-details-help"
    />
    <div>
        <ExchangeStateBadge {lang} exchange={details} size="md" testId="exchange-state" />
    </div>
    {#if details.reason}
        <p class="text-sm text-ink-700" data-test="exchange-reason">{t(`exchange.reason.${details.reason}`, lang)}</p>
    {/if}
    {#if details.attention}
        <NoteBox tone="warning" dataTest="exchange-attention">
            {t(`exchange.attention.${details.attention}`, lang)}
        </NoteBox>
    {/if}
    {#if facts.length > 0}
        <FactList items={facts} dataTest="exchange-facts" />
    {/if}
    <ParcelList {lang} parcels={details.parcels} {orderNumber} />
    {#if details.heldReasons.length > 0}
        <div>
            <HeadingWithHelp
                text={t('exchange.details.held', lang)}
                help={t('exchange.details.held.help', lang)}
                level="h4"
                testId="exchange-held-help"
            />
            <HeldReasons {lang} reasons={details.heldReasons} dataTest="exchange-held" />
        </div>
    {/if}
    {#if details.problem}
        <ProblemLine {lang} problem={details.problem} testId="exchange-problem" />
    {/if}
    {#if details.verdicts.length > 0}
        <div>
            <HeadingWithHelp
                text={t('exchange.details.verdicts', lang)}
                help={t('exchange.details.verdicts.help', lang)}
                level="h4"
                testId="exchange-verdicts-help"
            />
            <CodeVerdictList {lang} verdicts={details.verdicts} dataTest="exchange-verdicts" />
        </div>
    {/if}
    {#if details.cancelAnswer}
        <p class="text-sm text-ink-800" data-test="exchange-cancel">
            {t(`exchange.cancel.${details.cancelAnswer}`, lang)}
        </p>
    {/if}
    {#if details.history.length > 0}
        <div>
            <HeadingWithHelp
                text={t('exchange.details.history', lang)}
                help={t('exchange.details.history.help', lang)}
                level="h4"
                testId="exchange-history-help"
            />
            <ol class="flex list-none flex-col gap-2 text-sm" data-test="exchange-history">
                {#each details.history as event, index (index)}
                    <li class="flex flex-wrap items-baseline gap-2" data-test="exchange-history-row">
                        <span class="text-ink-600 tabular-nums">{formatDateTime(event.at, lang)}</span>
                        <span class="text-ink-800">{eventText(event.code)}</span>
                    </li>
                {/each}
            </ol>
        </div>
    {/if}
    {#if details.retryable}
        <div>
            <ButtonWithHelp
                variant="secondary"
                pending={retry.pending}
                dataTest="exchange-retry"
                help={t('exchange.retry.help', lang)}
                helpTestId="exchange-retry-help"
                helpLabel={t('exchange.retry', lang)}
                onclick={press}
            >
                {t('exchange.retry', lang)}
            </ButtonWithHelp>
            <FormAnswer saved={retry.saved} error={retry.error} dataTest="exchange-retry-answer" />
        </div>
    {/if}
</Stack>

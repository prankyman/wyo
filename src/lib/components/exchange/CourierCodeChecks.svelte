<script lang="ts">
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { formatDateTime } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import { tPlural } from '$lib/i18n/plural';
    import type { TLocale } from '$lib/i18n/types';
    import type { TCourierCheck } from './types';

    /**
     * The codes a courier named for an order that leaves alone with him (`OrderExchangeDetails.courierChecks`), oldest first, and
     * the marketplace's answer to each: when the courier named the code, the answer — «ответа ждём» while there is none, «код принят»,
     * «код не принят» or «нужен новый код» — in the tone of the answer, when the marketplace answered, how many tries it still
     * allows and its own words as received. The code itself is never here: the wyo server does not keep it. Before the courier
     * has named a code the block draws nothing, never an empty list.
     */
    interface IProps {
        lang: TLocale;
        /** The checks, oldest first; may be empty. */
        checks: TCourierCheck[];
        /** The block is `<testId>`, its «?» `<testId>-help`, a row `<testId>-row`; the row n (from 1) has `<testId>-<n>-at`, `-result`, `-answered`, `-attempts` and `-words`. */
        testId?: string;
    }

    let { lang, checks, testId = 'courier-checks' }: IProps = $props();

    const TONE: Readonly<Record<'awaited' | NonNullable<TCourierCheck['result']>, TTone>> = {
        awaited: 'wait',
        accepted: 'ok',
        refused: 'stop',
        new_code_needed: 'warn'
    };

    const attemptsText = (left: number) =>
        left > 0 ? tPlural('exchange.courier.attemptsLeft', left, lang) : t('exchange.courier.attemptsNone', lang);
</script>

{#if checks.length > 0}
    <Stack dataTest={testId}>
        <HeadingWithHelp
            text={t('exchange.courier.title', lang)}
            help={t('exchange.courier.help', lang)}
            level="h4"
            testId="{testId}-help"
        />
        <ol class="flex list-none flex-col gap-3 text-sm">
            {#each checks as check, index (index)}
                {@const answer = check.result ?? 'awaited'}
                {@const n = index + 1}
                <li class="flex flex-col gap-1" data-test="{testId}-row">
                    <p class="text-ink-800" data-test="{testId}-{n}-at">
                        {t('exchange.courier.at', lang, { at: formatDateTime(check.at, lang) })}
                    </p>
                    <div>
                        <StatusBadgeWithHelp
                            label={t(`exchange.courier.${answer}`, lang)}
                            tone={TONE[answer]}
                            help={t(`exchange.courier.${answer}.help`, lang)}
                            testId="{testId}-{n}-result"
                        />
                    </div>
                    {#if check.answeredAt}
                        <p class="text-ink-600" data-test="{testId}-{n}-answered">
                            {t('exchange.courier.answeredAt', lang, { at: formatDateTime(check.answeredAt, lang) })}
                        </p>
                    {/if}
                    {#if check.attemptsLeft !== undefined}
                        <p class="text-ink-800" data-test="{testId}-{n}-attempts">{attemptsText(check.attemptsLeft)}</p>
                    {/if}
                    {#if check.words}
                        <p class="break-words text-ink-800" data-test="{testId}-{n}-words">
                            {t('exchange.courier.words', lang, { words: check.words })}
                        </p>
                    {/if}
                </li>
            {/each}
        </ol>
    </Stack>
{/if}

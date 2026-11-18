<script lang="ts">
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TCodeVerdict } from './types';

    /**
     * The marketplace's verdict on each identifier of an order that it asked for (a marking code, a phone's number, ...): the
     * identifier's name, the verdict in the tone of the verdict with its «?», how many units of goods the identifiers were sent
     * for, and the marketplace's own words for the verdict as received. A marking code itself is never drawn: the wyo server never
     * sends one to the browser.
     */
    interface IProps {
        lang: TLocale;
        verdicts: TCodeVerdict[];
        /** The list is `<dataTest>`, each row `<dataTest>-row`, each verdict `<dataTest>-verdict`. */
        dataTest: string;
    }

    let { lang, verdicts, dataTest }: IProps = $props();

    const TONE: Readonly<Record<TCodeVerdict['verdict'], TTone>> = {
        accepted: 'ok',
        not_required: 'off',
        pending: 'wait',
        unfinished: 'warn',
        missing: 'stop',
        refused: 'stop'
    };
</script>

<ul class="flex list-none flex-col gap-3" data-test={dataTest}>
    {#each verdicts as verdict, index (index)}
        <li class="flex flex-col gap-1 text-sm" data-test="{dataTest}-row">
            <span class="flex flex-wrap items-center gap-2">
                <span class="text-ink-800">{t(`exchange.identifier.${verdict.identifier}`, lang)}</span>
                <StatusBadgeWithHelp
                    label={t(`exchange.verdict.${verdict.verdict}`, lang)}
                    tone={TONE[verdict.verdict]}
                    help={t(`exchange.verdict.${verdict.verdict}.help`, lang)}
                    testId="{dataTest}-verdict"
                />
            </span>
            {#if verdict.units !== undefined}
                <span class="text-ink-600">{t('exchange.verdict.units', lang, { units: verdict.units })}</span>
            {/if}
            {#if verdict.words}
                <span class="break-words text-ink-600"
                    >{t('exchange.verdict.words', lang, { words: verdict.words })}</span
                >
            {/if}
        </li>
    {/each}
</ul>

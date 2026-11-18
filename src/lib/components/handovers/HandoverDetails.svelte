<script lang="ts">
    import { api } from '$lib/api/api';
    import ProblemLine from '$lib/components/exchange/ProblemLine.svelte';
    import ButtonWithHelp from '$lib/components/ui/ButtonWithHelp.svelte';
    import FactList from '$lib/components/ui/FactList.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { ApiAction } from '$lib/forms/api-action.svelte';
    import { formatIsoDay } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import HandoverStateBadge from './HandoverStateBadge.svelte';
    import HandoverSteps from './HandoverSteps.svelte';
    import ShippingPointPicker from './ShippingPointPicker.svelte';
    import type { THandoverDetails } from './types';

    /**
     * One hand-over beside the list on «Передачи» (`GET /api/v1/handovers/{id}`): its state, the strip of its six steps with
     * «Сейчас: …», its facts (the fulfilment centre's number, the day, the marketplace's numbers of the batches, the orders, the
     * boxes), the problem in wyo's words and the other party's, the choice of the shipping point when the owner must make it,
     * «Повторить» when the wyo server says the failed step may be repeated, and a note that nothing can be undone once the batch
     * is closed. The page is told (`onChanged`) after a repeat or a saved point, to read the hand-over again.
     */
    interface IProps {
        lang: TLocale;
        details: THandoverDetails;
        onChanged?: () => void;
    }

    let { lang, details, onChanged }: IProps = $props();

    const handover = $derived(details.handover);
    const retry = new ApiAction();

    async function press(): Promise<void> {
        const result = await retry.run(
            () => api.post(`/api/v1/handovers/${handover.id}/retry`, {}),
            'handovers.retry.started'
        );
        if (result?.ok) onChanged?.();
    }

    const facts = $derived.by(() => {
        const items: { key: string; label: string; value: string; help: string }[] = [];
        const add = (key: string, value: string) => {
            if (value !== '') {
                items.push({
                    key,
                    value,
                    label: t(`handovers.fact.${key}`, lang),
                    help: t(`handovers.fact.${key}.help`, lang)
                });
            }
        };
        add('number', handover.number);
        add('day', formatIsoDay(handover.day, lang));
        add('batches', handover.batches.join(', '));
        add('orders', details.orderList.map((order) => order.number).join(', '));
        add('boxes', String(handover.boxes));
        return items;
    });
</script>

<Stack dataTest="handover-details">
    <div>
        <HandoverStateBadge {lang} state={handover.state} size="md" testId="handover-state" />
    </div>
    <HandoverSteps {lang} steps={details.steps} dataTest="handover-steps" />
    {#if handover.problem}
        <ProblemLine {lang} problem={handover.problem} testId="handover-problem" />
    {/if}
    <FactList items={facts} dataTest="handover-facts" />
    {#if details.needsShippingPoint}
        <ShippingPointPicker {lang} handoverId={handover.id} onSaved={() => onChanged?.()} />
    {/if}
    {#if details.retryable}
        <div>
            <ButtonWithHelp
                variant="secondary"
                pending={retry.pending}
                dataTest="handover-retry"
                help={t('handovers.retry.help', lang)}
                helpTestId="handover-retry-help"
                helpLabel={t('handovers.retry', lang)}
                onclick={press}
            >
                {t('handovers.retry', lang)}
            </ButtonWithHelp>
            <FormAnswer saved={retry.saved} error={retry.error} dataTest="handover-retry-answer" />
        </div>
    {/if}
    {#if details.closed}
        <NoteBox tone="info" dataTest="handover-closed">{t('handovers.closed', lang)}</NoteBox>
    {/if}
</Stack>

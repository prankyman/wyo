<script lang="ts">
    import { api } from '$lib/api/api';
    import { refusalKey } from '$lib/api/refusal-key';
    import Button from '$lib/components/ui/Button.svelte';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import FormButtons from '$lib/components/ui/FormButtons.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { answerLines } from '$lib/forms/answer';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { shippingPointSchema } from '$lib/validation/handover';
    import { onMount } from 'svelte';
    import type { THandoverDetails, TShippingPointChoice } from './types';

    /**
     * The choice of the shipping point of a hand-over. The marketplace gives the list of points only for a batch that already holds
     * orders, so the list is asked of it now (`GET /api/v1/handovers/{id}/shipping-point`) and the owner picks one; wyo never
     * guesses. The remembered point is marked and chosen. «Запомнить и продолжить» (`PUT` of the same address) remembers the point
     * for the account, for this and the next hand-overs, and the steps go on; the page is told the hand-over as it now stands.
     */
    interface IProps {
        lang: TLocale;
        handoverId: number;
        onSaved: (details: THandoverDetails) => void;
    }

    let { lang, handoverId, onSaved }: IProps = $props();

    // svelte-ignore state_referenced_locally
    const id = handoverId;
    const path = `/api/v1/handovers/${id}/shipping-point`;

    let choice = $state<TShippingPointChoice | null>(null);
    let refusal = $state<string | null>(null);

    const made = spaForm(
        shippingPointSchema,
        { point: '' },
        {
            id: 'shipping-point-form',
            resetForm: false,
            send: async (values) => {
                const result = await api.put<THandoverDetails>(path, { point: values.point });
                if (result.ok) onSaved(result.data);
                return result;
            }
        }
    );
    const { form, errors, message, enhance, submitting } = made;
    const answer = $derived(answerLines($message, lang, t('handovers.shippingPoint.saved', lang)));

    async function read(): Promise<void> {
        refusal = null;
        const result = await api.get<TShippingPointChoice>(path);
        if (!result.ok) {
            refusal = result.refusal.code;
            return;
        }
        choice = result.data;
        made.form.update((values) => ({ ...values, point: result.data.chosen ?? '' }), { taint: false });
    }

    const optionLabel = (point: TShippingPointChoice['points'][number]) => {
        const name = point.address ? `${point.name} — ${point.address}` : point.name;
        return point.id === choice?.chosen ? t('handovers.shippingPoint.remembered', lang, { name }) : name;
    };

    onMount(() => {
        void read();
    });
</script>

<Stack dataTest="shipping-point-part">
    {#if refusal}
        <NoteBox tone="error" role="alert" dataTest="shipping-point-refusal">{t(refusalKey(refusal), lang)}</NoteBox>
        <div>
            <Button variant="secondary" dataTest="shipping-point-retry" onclick={read}>{t('common.retry', lang)}</Button
            >
        </div>
    {:else if choice === null}
        <p class="text-ink-600" data-test="shipping-point-loading">{t('handovers.shippingPoint.loading', lang)}</p>
    {:else if choice.points.length === 0}
        <EmptyState message={t('handovers.shippingPoint.empty', lang)} dataTest="shipping-point-empty" />
    {:else}
        <GuardedForm id="shipping-point-form" dataTest="shipping-point" superform={{ enhance, submitting }}>
            <Stack>
                <Field
                    label={t('handovers.shippingPoint.label', lang)}
                    help={t('handovers.shippingPoint.help', lang)}
                    for="shipping-point-box"
                    testId="shipping-point-field"
                    error={tError($errors.point, lang)}
                >
                    <SelectBox
                        id="shipping-point-box"
                        name="point"
                        bind:value={$form.point}
                        blank={choice.chosen ? undefined : t('handovers.shippingPoint.blank', lang)}
                        invalid={Boolean($errors.point)}
                        options={choice.points.map((point) => ({ value: point.id, label: optionLabel(point) }))}
                        dataTest="shipping-point-box"
                    />
                </Field>
                <FormButtons>
                    <Button type="submit" dataTest="shipping-point-send"
                        >{t('handovers.shippingPoint.send', lang)}</Button
                    >
                    {#snippet below()}
                        <FormAnswer saved={answer.saved} error={answer.error} dataTest="shipping-point-answer" />
                    {/snippet}
                </FormButtons>
            </Stack>
        </GuardedForm>
    {/if}
</Stack>

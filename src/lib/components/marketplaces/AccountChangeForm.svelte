<script lang="ts">
    import { api } from '$lib/api/api';
    import Button from '$lib/components/ui/Button.svelte';
    import DisclosureClose from '$lib/components/ui/DisclosureClose.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import FormButtons from '$lib/components/ui/FormButtons.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import NumberField from '$lib/components/ui/NumberField.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import { answerLines } from '$lib/forms/answer';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { accountChangePayload, accountChangeSchema, DEADLINE_HOURS } from '$lib/validation/account';
    import { accountRefusalLookup } from '$lib/marketplaces/refusals';
    import type { TAccount } from './types';

    /**
     * «Изменить» on an account's card: the account's own name and its hours to hand an order over («Срок передачи заказа, часов»,
     * 48 by default), saved with `PATCH /api/v1/accounts/{id}`. The hours are checked in the browser against the bounds the wyo
     * server holds too. A marketplace that gives each order its own deadline (`givesDeadline`) has no use for the hours, so the
     * form asks only the name and sends only the name. The form keeps the saved values: it changes a record, it does not add one.
     */
    interface IProps {
        lang: TLocale;
        account: TAccount;
        /** The account's marketplace gives each order its own deadline: the hours are not asked. */
        givesDeadline?: boolean;
        /** The form's id and the prefix of every `data-test` in it. */
        id: string;
        /** Told the account as the wyo server stored it, after a save. */
        onSaved: (account: TAccount) => void;
    }

    let { lang, account, givesDeadline = false, id, onSaved }: IProps = $props();

    // svelte-ignore state_referenced_locally
    const changed = account;
    // svelte-ignore state_referenced_locally
    const withHours = !givesDeadline;

    // svelte-ignore state_referenced_locally
    const formId = id;

    const made = spaForm(
        accountChangeSchema,
        { name: changed.name, deadlineHours: String(changed.deadlineHours) },
        {
            id: formId,
            resetForm: false,
            refusals: accountRefusalLookup(changed.marketplace),
            send: async (values) => {
                const result = await api.patch<TAccount>(
                    `/api/v1/accounts/${changed.id}`,
                    withHours ? accountChangePayload(values) : { name: values.name }
                );
                if (result.ok) onSaved(result.data);
                return result;
            }
        }
    );
    const { form, errors, message, enhance, submitting } = made;

    const answer = $derived(answerLines($message, lang, t('accounts.change.saved', lang)));
</script>

<GuardedForm {id} dataTest={id} superform={{ enhance, submitting }}>
    <Stack>
        <Field
            label={t('accounts.form.name', lang)}
            help={t('accounts.form.name.help', lang)}
            for="{id}-name-box"
            testId="{id}-name"
            dataTest="{id}-name-field"
            error={tError($errors.name, lang)}
        >
            <TextBox
                id="{id}-name-box"
                name="name"
                bind:value={$form.name}
                invalid={Boolean($errors.name)}
                class="sm:max-w-md"
                dataTest="{id}-name"
            />
        </Field>
        {#if withHours}
            <Field
                label={t('accounts.form.deadlineHours', lang)}
                help={t('accounts.form.deadlineHours.help', lang, { min: DEADLINE_HOURS.min, max: DEADLINE_HOURS.max })}
                for="{id}-hours-box"
                testId="{id}-hours"
                dataTest="{id}-hours-field"
                error={tError($errors.deadlineHours, lang)}
            >
                <NumberField
                    id="{id}-hours-box"
                    name="deadlineHours"
                    {lang}
                    decimals={0}
                    bind:value={$form.deadlineHours}
                    class="sm:max-w-xs"
                    dataTest="{id}-hours"
                />
            </Field>
        {/if}
        <FormButtons>
            <Button type="submit" dataTest="{id}-send">{t('app.common.save', lang)}</Button>
            <DisclosureClose />
            {#snippet below()}
                <FormAnswer saved={answer.saved} error={answer.error} dataTest="{id}-answer" />
            {/snippet}
        </FormButtons>
    </Stack>
</GuardedForm>

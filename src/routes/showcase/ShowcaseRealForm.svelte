<script lang="ts">
    import type { SuperForm } from 'sveltekit-superforms';
    import { z } from 'zod';
    import type { TApiResult } from '$lib/api/api';
    import Button from '$lib/components/ui/Button.svelte';
    import DisclosureClose from '$lib/components/ui/DisclosureClose.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import FormButtons from '$lib/components/ui/FormButtons.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import NumberField from '$lib/components/ui/NumberField.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import Textarea from '$lib/components/ui/Textarea.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import { answerLines } from '$lib/forms/answer';
    import { spaForm } from '$lib/forms/super-form';
    import { getLocale, t } from '$lib/i18n/i18n';
    import { tError } from '$lib/i18n/form-error';
    import { NUMBER_RULES, numberField } from '$lib/validation/number-input';
    import { stubSend } from './showcase-stub';

    /**
     * The one real form of the showcase page, and the proof that the form library works with no server behind the interface
     * (constitution, "Forms"): a zod schema checks it in the browser, `send` (a stub, here) takes the place of the call to the
     * wyo server, and a refusal is laid onto the boxes and under the button by the same function every form uses. Drawn twice
     * on the page — alone, and inside a `DisclosureForm`, where `DisclosureClose` stands beside its button.
     */
    interface IProps {
        /** The form's id and the prefix of every `data-test` in it. */
        id?: string;
        send?: (values: { name: string }) => Promise<TApiResult<unknown>>;
    }

    let { id = 'real-form', send = stubSend }: IProps = $props();

    const lang = getLocale();

    const schema = z.object({
        name: z.string().min(2, 'errors.invalid'),
        kind: z.string().min(1, 'errors.invalid'),
        amount: numberField(NUMBER_RULES.money, 'errors.invalid'),
        note: z.string().max(200, 'errors.invalid')
    });

    // svelte-ignore state_referenced_locally
    const made = spaForm(schema, { name: '', kind: '', amount: '', note: '' }, { id, resetForm: false, send });
    const { form, errors, message, enhance, submitting } = made;

    const answer = $derived(answerLines($message, lang, t('showcase.forms.answer.saved', lang)));
</script>

<GuardedForm {id} dataTest={id} superform={{ enhance, submitting }}>
    <Stack>
        <Field
            label={t('showcase.forms.label.name', lang)}
            help={t('showcase.forms.help.name', lang)}
            for="{id}-name-box"
            testId="{id}-name-field"
            error={tError($errors.name, lang)}
        >
            <TextBox
                id="{id}-name-box"
                name="name"
                bind:value={$form.name}
                invalid={Boolean($errors.name)}
                dataTest="{id}-name"
            />
        </Field>
        <Field
            label={t('showcase.forms.label.kind', lang)}
            for="{id}-kind-box"
            testId="{id}-kind-field"
            error={tError($errors.kind, lang)}
        >
            <SelectBox
                id="{id}-kind-box"
                name="kind"
                bind:value={$form.kind}
                blank={t('showcase.forms.blank', lang)}
                invalid={Boolean($errors.kind)}
                options={[
                    { value: 'first', label: t('showcase.forms.option.first', lang) },
                    { value: 'second', label: t('showcase.forms.option.second', lang) }
                ]}
                dataTest="{id}-kind"
            />
        </Field>
        <Field
            label={t('showcase.forms.label.amount', lang)}
            for="{id}-amount-box"
            testId="{id}-amount-field"
            error={tError($errors.amount, lang)}
        >
            <NumberField
                id="{id}-amount-box"
                name="amount"
                {lang}
                decimals={2}
                bind:value={$form.amount}
                dataTest="{id}-amount"
            />
        </Field>
        <Field label={t('showcase.forms.label.note', lang)} for="{id}-note-box" testId="{id}-note-field">
            <Textarea
                id="{id}-note-box"
                form={made as unknown as SuperForm<Record<string, unknown>>}
                name="note"
                {lang}
                maxlength={200}
                testId="{id}-note"
            />
        </Field>
        <FormButtons>
            <Button type="submit" dataTest="{id}-send">{t('showcase.forms.form.send', lang)}</Button>
            <DisclosureClose />
            {#snippet below()}
                <FormAnswer saved={answer.saved} error={answer.error} dataTest="{id}-answer" />
            {/snippet}
        </FormButtons>
    </Stack>
</GuardedForm>

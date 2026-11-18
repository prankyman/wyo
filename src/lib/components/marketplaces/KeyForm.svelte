<script lang="ts">
    import { api } from '$lib/api/api';
    import Button from '$lib/components/ui/Button.svelte';
    import DisclosureClose from '$lib/components/ui/DisclosureClose.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import FormButtons from '$lib/components/ui/FormButtons.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import { answerLines } from '$lib/forms/answer';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { accountCreateSchema, emptyKey, keyReplaceSchema } from '$lib/validation/account';
    import { ACCOUNT_REFUSALS } from '$lib/marketplaces/refusals';
    import KeyHelpSteps from './KeyHelpSteps.svelte';
    import type { TAccount, TMarketplace } from './types';

    /**
     * The key form of a marketplace account, drawn from the adapter's description: a box for every field of its `keyFields`,
     * named by the texts `marketplace.<code>.key.<field>.label` and `.help`. Without `account` it adds an account (its own name
     * and the key, `POST /api/v1/accounts`); with one it replaces that account's key (`PUT /api/v1/accounts/{id}/key`) and names
     * the stored key only by its last four characters. The steps of making the key stand above the boxes (`KeyHelpSteps`).
     *
     * A secret field is a secret box: what is typed is hidden, the browser is asked not to remember it, and the box empties after
     * a save, because the wyo server never sends a key back. A refused save keeps everything typed and lays the refusal onto its
     * box, worded from the accounts' own texts (`accounts.refusal.<code>`).
     */
    interface IProps {
        lang: TLocale;
        marketplace: TMarketplace;
        /** The account whose key is replaced; left out, the form adds an account. */
        account?: TAccount;
        /** The form's id and the prefix of every `data-test` in it. */
        id: string;
        /** Told the account as the wyo server stored it, after a save. */
        onSaved: (account: TAccount) => void;
    }

    let { lang, marketplace, account, id, onSaved }: IProps = $props();

    // The form is made once, for the adapter and the account it was drawn for.
    // svelte-ignore state_referenced_locally
    const replacing = account;
    // svelte-ignore state_referenced_locally
    const adapter = marketplace;
    const schema = replacing ? keyReplaceSchema(adapter.keyFields) : accountCreateSchema(adapter.keyFields);

    // svelte-ignore state_referenced_locally
    const formId = id;

    const made = spaForm(
        schema,
        { name: '', key: emptyKey(adapter.keyFields) },
        {
            id: formId,
            resetForm: true,
            dataType: 'json',
            refusals: ACCOUNT_REFUSALS,
            send: async (values) => {
                const result = replacing
                    ? await api.put<TAccount>(`/api/v1/accounts/${replacing.id}/key`, { key: values.key })
                    : await api.post<TAccount>('/api/v1/accounts', {
                          marketplace: adapter.code,
                          name: values.name,
                          key: values.key
                      });
                if (result.ok) onSaved(result.data);
                return result;
            }
        }
    );
    const { form, errors, message, enhance, submitting } = made;

    const answer = $derived(
        answerLines($message, lang, t(replacing ? 'accounts.form.replaced' : 'accounts.form.added', lang))
    );
    const keyText = (field: string, part: 'label' | 'help') =>
        t(`marketplace.${adapter.code}.key.${field}.${part}`, lang);
</script>

<Stack>
    <KeyHelpSteps {lang} code={adapter.code} id="{id}-steps" open={replacing === undefined} />
    {#if replacing}
        <p class="text-sm text-ink-700" data-test="{id}-current">
            {t('accounts.form.current', lang, { last4: replacing.keyLast4 })}
        </p>
    {/if}
    <GuardedForm {id} dataTest={id} superform={{ enhance, submitting }}>
        <Stack>
            {#if !replacing}
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
            {/if}
            {#each adapter.keyFields as field (field.name)}
                <Field
                    label={keyText(field.name, 'label')}
                    help={keyText(field.name, 'help')}
                    for="{id}-key-{field.name}-box"
                    testId="{id}-key-{field.name}"
                    dataTest="{id}-key-{field.name}-field"
                    error={tError($errors.key?.[field.name], lang)}
                >
                    <TextBox
                        id="{id}-key-{field.name}-box"
                        name="key.{field.name}"
                        type={field.secret ? 'password' : 'text'}
                        autocomplete="off"
                        spellcheck="false"
                        bind:value={$form.key[field.name]}
                        invalid={Boolean($errors.key?.[field.name])}
                        dataTest="{id}-key-{field.name}"
                    />
                </Field>
            {/each}
            <FormButtons>
                <Button type="submit" dataTest="{id}-send">
                    {t(replacing ? 'accounts.form.replace' : 'accounts.form.add', lang)}
                </Button>
                <DisclosureClose />
                {#snippet below()}
                    <FormAnswer saved={answer.saved} error={answer.error} dataTest="{id}-answer" />
                {/snippet}
            </FormButtons>
        </Stack>
    </GuardedForm>
</Stack>

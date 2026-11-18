<script lang="ts">
    import { api } from '$lib/api/api';
    import ButtonWithHelp from '$lib/components/ui/ButtonWithHelp.svelte';
    import ConfirmationModal from '$lib/components/ui/ConfirmationModal.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import { ApiAction } from '$lib/forms/api-action.svelte';
    import type { ISectionSuperForm } from '$lib/forms/section-form';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { connectSchemaFor } from '$lib/validation/external-api';
    import ConnectionCheckResult from './ConnectionCheckResult.svelte';
    import type { TExternalApi } from './types';

    /**
     * The section «Подключение» of «Внешний API»: the address of the fulfilment centre's API and its key. «Проверить и сохранить»
     * (`PUT /api/v1/external-api`) asks the fulfilment centre's system who the key belongs to and saves the address and the key only
     * when it answered and the key may write; the answer is drawn under the boxes (`ConnectionCheckResult`). The key box is a
     * secret box: hidden as it is typed, never remembered by the browser, emptied after a save because the wyo server never sends
     * a key back; the stored key is named by its last four characters. «Проверить снова» (`POST …/check`) asks again; «Отключить»
     * (`DELETE`), behind a window, empties the setting. The page is told every new state (`onChanged`) and the disconnection.
     * A released wyo server takes only an `https://` address; the page tells (`allowPlainHttp`) when the wyo server is a build for
     * development and tests, which takes plain `http://` too, for the fake fulfilment centre.
     */
    interface IProps {
        lang: TLocale;
        connection: TExternalApi;
        allowPlainHttp?: boolean;
        onChanged: (connection: TExternalApi) => void;
        onDisconnected: () => void;
    }

    let { lang, connection, allowPlainHttp = false, onChanged, onDisconnected }: IProps = $props();

    const filled = $derived(connection.state !== 'empty');
    const recheck = new ApiAction();
    const disconnect = new ApiAction();
    let asking = $state(false);

    const made = spaForm(
        connectSchemaFor(() => allowPlainHttp),
        // svelte-ignore state_referenced_locally
        { address: connection.address ?? '', key: '' },
        {
            id: 'external-api-connection-form',
            resetForm: false,
            saved: 'section',
            send: async (values) => {
                const result = await api.put<TExternalApi>('/api/v1/external-api', values);
                if (result.ok) {
                    onChanged(result.data);
                    return result;
                }
                // The address is the box an invalid address is about, as well as the whole form's answer.
                return result.refusal.code === 'invalid_address' && !result.refusal.issues
                    ? {
                          ...result,
                          refusal: { ...result.refusal, issues: [{ path: 'address', code: 'invalid_address' }] }
                      }
                    : result;
            },
            // The form library puts the sent values back into the boxes when a send is over; the key is emptied after that.
            onUpdated: ({ form: sent }) => {
                if (sent.valid) made.form.update((one) => ({ ...one, key: '' }), { taint: false });
            }
        }
    );
    const { form, errors } = made;
    const section = made as unknown as ISectionSuperForm;

    async function check(): Promise<void> {
        const result = await recheck.run(
            () => api.post<TExternalApi>('/api/v1/external-api/check', {}),
            'externalApi.connection.checked'
        );
        if (result?.ok) onChanged(result.data);
    }

    async function remove(): Promise<void> {
        const result = await disconnect.run(() => api.delete('/api/v1/external-api'));
        if (result?.ok) onDisconnected();
    }
</script>

<SectionForm
    {lang}
    id="external-api-connection"
    title={t('externalApi.connection.title', lang)}
    help={t('externalApi.connection.help', lang)}
    saveLabel={t('externalApi.connection.save', lang)}
    saveHelp={t('externalApi.connection.save.help', lang)}
    form={section}
>
    <Stack>
        <p class="text-sm text-ink-700" data-test="external-api-connection-intro">
            {t('externalApi.connection.intro', lang)}
        </p>
        {#if filled}
            <ConnectionCheckResult {lang} {connection} testId="connection-result" />
        {/if}
        <Field
            label={t('externalApi.connection.address', lang)}
            help={t('externalApi.connection.address.help', lang)}
            for="connection-address-box"
            testId="connection-address"
            dataTest="connection-address-field"
            error={tError($errors.address, lang)}
        >
            <TextBox
                id="connection-address-box"
                name="address"
                type="url"
                autocomplete="off"
                spellcheck="false"
                bind:value={$form.address}
                invalid={Boolean($errors.address)}
                dataTest="connection-address-box"
            />
        </Field>
        {#if connection.keyLast4}
            <p class="text-sm text-ink-700" data-test="connection-current">
                {t('externalApi.connection.current', lang, { last4: connection.keyLast4 })}
            </p>
        {/if}
        <Field
            label={t('externalApi.connection.key', lang)}
            help={t('externalApi.connection.key.help', lang)}
            for="connection-key-box"
            testId="connection-key"
            dataTest="connection-key-field"
            error={tError($errors.key, lang)}
        >
            <TextBox
                id="connection-key-box"
                name="key"
                type="password"
                autocomplete="off"
                spellcheck="false"
                bind:value={$form.key}
                invalid={Boolean($errors.key)}
                dataTest="connection-key-box"
            />
        </Field>
        {#if filled}
            <div>
                <div class="flex flex-wrap items-center gap-2">
                    <ButtonWithHelp
                        variant="secondary"
                        pending={recheck.pending}
                        dataTest="connection-check"
                        help={t('externalApi.connection.check.help', lang)}
                        helpTestId="connection-check-help"
                        helpLabel={t('externalApi.connection.check', lang)}
                        onclick={check}
                    >
                        {t('externalApi.connection.check', lang)}
                    </ButtonWithHelp>
                    <ButtonWithHelp
                        variant="danger-outline"
                        pending={disconnect.pending}
                        dataTest="connection-disconnect"
                        help={t('externalApi.connection.disconnect.help', lang)}
                        helpTestId="connection-disconnect-help"
                        helpLabel={t('externalApi.connection.disconnect', lang)}
                        onclick={() => (asking = true)}
                    >
                        {t('externalApi.connection.disconnect', lang)}
                    </ButtonWithHelp>
                </div>
                <FormAnswer
                    saved={recheck.saved}
                    error={recheck.error ?? disconnect.error}
                    dataTest="connection-check-answer"
                />
            </div>
        {/if}
    </Stack>
</SectionForm>

<ConfirmationModal
    bind:open={asking}
    title={t('externalApi.connection.disconnectWindow.title', lang)}
    message={t('externalApi.connection.disconnectWindow.message', lang)}
    confirmText={t('externalApi.connection.disconnectWindow.confirm', lang)}
    cancelText={t('app.common.cancel', lang)}
    dataTest="connection-disconnect-window"
    onConfirm={() => {
        void remove();
    }}
/>

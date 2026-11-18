<script lang="ts">
    import { api } from '$lib/api/api';
    import CopyButton from '$lib/components/ui/CopyButton.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import { COPY_BOX_CLASS } from '$lib/components/ui/field-classes';
    import FactList from '$lib/components/ui/FactList.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import type { ISectionSuperForm } from '$lib/forms/section-form';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { formatDateTime } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { eventsPayload, eventsSchema } from '$lib/validation/external-api';
    import type { TEventsSetting } from './types';

    /**
     * The section «События» of «Внешний API»: how the wyo server learns what the fulfilment centre did. «Запрашивать раз в минуту»
     * works on any server. «Принимать уведомления» shows the wyo server's own address to copy into the fulfilment centre's system
     * and a box for the signing secret that system shows (a secret box: hidden as it is typed, never remembered by the browser,
     * emptied after a save (once the form library has put the sent values back), because the wyo server never sends a secret back), and the times of the last test notification and
     * the last notification. The wyo server makes the address when notifications are first saved. Saved with
     * `PUT /api/v1/external-api/events`; the page is told the setting as the wyo server stored it.
     */
    interface IProps {
        lang: TLocale;
        events: TEventsSetting;
        onSaved: (events: TEventsSetting) => void;
    }

    let { lang, events, onSaved }: IProps = $props();

    // The form is made once, from the setting as it was when the section was drawn; a save replaces what the section shows.
    // svelte-ignore state_referenced_locally
    let current = $state(events);

    // svelte-ignore state_referenced_locally
    const made = spaForm(
        eventsSchema(() => current.secretSaved === true),
        { mode: events.mode, secret: '' },
        {
            id: 'external-api-events-form',
            resetForm: false,
            saved: 'section',
            send: async (values) => {
                const result = await api.put<TEventsSetting>('/api/v1/external-api/events', eventsPayload(values));
                if (result.ok) {
                    current = result.data;
                    onSaved(result.data);
                }
                return result;
            },
            // The form library puts the sent values back into the boxes when a send is over; the secret is emptied after that.
            onUpdated: ({ form: sent }) => {
                if (sent.valid) made.form.update((one) => ({ ...one, secret: '' }), { taint: false });
            }
        }
    );
    const { form, errors } = made;
    const section = made as unknown as ISectionSuperForm;

    const facts = $derived.by(() => {
        const items: { key: string; label: string; value: string; help: string }[] = [];
        const add = (key: string, value: string) =>
            items.push({
                key,
                value,
                label: t(`externalApi.events.${key}`, lang),
                help: t(`externalApi.events.${key}.help`, lang)
            });
        add(
            'lastPing',
            current.lastPingAt ? formatDateTime(current.lastPingAt, lang) : t('externalApi.events.lastPing.never', lang)
        );
        if (current.lastNotificationAt) add('lastNotification', formatDateTime(current.lastNotificationAt, lang));
        return items;
    });
</script>

<SectionForm
    {lang}
    id="external-api-events"
    title={t('externalApi.events.title', lang)}
    help={t('externalApi.events.titleHelp', lang)}
    form={section}
>
    <Stack>
        <p class="text-sm text-ink-700" data-test="external-api-events-intro">{t('externalApi.events.intro', lang)}</p>
        <Field
            label={t('externalApi.events.mode', lang)}
            help={t('externalApi.events.mode.help', lang)}
            for="events-mode-box"
            testId="events-mode"
            dataTest="events-mode-field"
            error={tError($errors.mode, lang)}
        >
            <SelectBox
                id="events-mode-box"
                name="mode"
                bind:value={$form.mode}
                options={[
                    { value: 'poll', label: t('externalApi.events.mode.poll', lang) },
                    { value: 'notify', label: t('externalApi.events.mode.notify', lang) }
                ]}
                class="sm:max-w-md"
                dataTest="events-mode-box"
            />
        </Field>
        {#if $form.mode === 'notify'}
            {#if current.hookUrl}
                <Field
                    label={t('externalApi.events.hookUrl', lang)}
                    help={t('externalApi.events.hookUrl.help', lang)}
                    testId="events-hook"
                    dataTest="events-hook-field"
                >
                    <Stack>
                        <code class={COPY_BOX_CLASS} data-test="events-hook-url">{current.hookUrl}</code>
                        <div>
                            <CopyButton
                                text={current.hookUrl}
                                label={t('app.ui.showOnce.copy', lang)}
                                copiedLabel={t('app.ui.showOnce.copied', lang)}
                                failedLabel={t('app.ui.showOnce.copyFailed', lang)}
                                dataTest="events-hook-copy"
                            />
                        </div>
                    </Stack>
                </Field>
            {:else}
                <p class="text-sm text-ink-700" data-test="events-hook-pending">
                    {t('externalApi.events.hookUrl.pending', lang)}
                </p>
            {/if}
            <Field
                label={t('externalApi.events.secret', lang)}
                help={t('externalApi.events.secret.help', lang)}
                for="events-secret-box"
                testId="events-secret"
                dataTest="events-secret-field"
                error={tError($errors.secret, lang)}
            >
                <TextBox
                    id="events-secret-box"
                    name="secret"
                    type="password"
                    autocomplete="off"
                    spellcheck="false"
                    bind:value={$form.secret}
                    invalid={Boolean($errors.secret)}
                    dataTest="events-secret-box"
                />
            </Field>
            {#if current.secretSaved}
                <p class="text-sm text-ink-700" data-test="events-secret-saved">
                    {t('externalApi.events.secretSaved', lang)}
                </p>
            {/if}
            <FactList items={facts} dataTest="events-facts" />
        {/if}
    </Stack>
</SectionForm>

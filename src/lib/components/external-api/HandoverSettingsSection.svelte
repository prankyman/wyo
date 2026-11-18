<script lang="ts">
    import { api } from '$lib/api/api';
    import type { TMarketplace } from '$lib/components/marketplaces/types';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import type { ISectionSuperForm } from '$lib/forms/section-form';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { handoverSettingsPayload, handoverSettingsSchema } from '$lib/validation/external-api';
    import AdapterSettingsForm from './AdapterSettingsForm.svelte';
    import type { TMarketplaceMatches } from './types';

    /**
     * The section «Передача заказов маркетплейсу» of «Внешний API»: what the marketplace of each account wants to know when a
     * group of orders is closed and handed over (how it travels, which kind of point takes it). The settings are drawn only from
     * what each account's adapter declares (`AdapterSettingsForm`); an account whose adapter declares none is named with a line that says
     * there is nothing to fill in for it. Saved as
     * one form with `PUT /api/v1/external-api/marketplaces`, which leaves the marketplace codes alone.
     */
    interface IProps {
        lang: TLocale;
        matches: TMarketplaceMatches;
        /** The adapters (`GET /api/v1/marketplaces`), for the settings each declares. */
        marketplaces: TMarketplace[];
        onSaved: (matches: TMarketplaceMatches) => void;
    }

    let { lang, matches, marketplaces, onSaved }: IProps = $props();

    // The form is made once, from the matches as they were when the section was drawn.
    // svelte-ignore state_referenced_locally
    const drawn = matches;
    // svelte-ignore state_referenced_locally
    const adapters = marketplaces;
    const adapterOf = (code: string) => adapters.find((adapter) => adapter.code === code);
    /** The rows whose adapter declares a hand-over setting, by their place in the form. */
    const shown = drawn.rows.flatMap((row, index) => {
        const adapter = adapterOf(row.marketplace);
        return adapter && adapter.handoverSettings.length > 0 ? [{ row, index, adapter }] : [];
    });

    /** The rows whose adapter declares none: named, so that an account is never missing without a word. */
    const silent = drawn.rows.filter((row) => !shown.some((one) => one.row.accountId === row.accountId));

    const made = spaForm(
        handoverSettingsSchema,
        {
            rows: drawn.rows.map((row) => ({ accountId: row.accountId, handoverSettings: { ...row.handoverSettings } }))
        },
        {
            id: 'external-api-handover-form',
            resetForm: false,
            dataType: 'json',
            saved: 'section',
            send: async (values) => {
                const result = await api.put<TMarketplaceMatches>(
                    '/api/v1/external-api/marketplaces',
                    handoverSettingsPayload(values)
                );
                if (result.ok) onSaved(result.data);
                return result;
            }
        }
    );
    const { form, errors } = made;
    const section = made as unknown as ISectionSuperForm;

    /** The refusals of one row's settings, worded, by the setting's name. */
    function errorsOf(index: number): Record<string, string | undefined> {
        const issues = ($errors.rows?.[index]?.handoverSettings ?? {}) as Record<string, string[] | undefined>;
        return Object.fromEntries(
            Object.entries(issues).map(([name, messages]) => [name, tError(messages, lang) || undefined])
        );
    }
</script>

<SectionForm
    {lang}
    id="external-api-handover"
    title={t('externalApi.handover.title', lang)}
    help={t('externalApi.handover.help', lang)}
    form={section}
>
    <Stack>
        <p class="text-sm text-ink-700" data-test="external-api-handover-intro">
            {t('externalApi.handover.intro', lang)}
        </p>
        {#if shown.length === 0}
            <EmptyState
                message={t(
                    drawn.rows.length === 0 ? 'externalApi.handover.noAccounts' : 'externalApi.handover.none',
                    lang
                )}
                dataTest="handover-none"
            />
        {:else}
            {#each shown as { row, index, adapter } (row.accountId)}
                <div data-test="handover-account">
                    <Stack>
                        <HeadingWithHelp
                            text={t('externalApi.handover.account', lang, { account: row.accountName })}
                            level="h3"
                            look="section"
                            headingTestId="handover-account-title-{index}"
                        />
                        <AdapterSettingsForm
                            {lang}
                            marketplace={adapter}
                            values={$form.rows[index].handoverSettings}
                            onChange={(name, value) => ($form.rows[index].handoverSettings[name] = value)}
                            errors={errorsOf(index)}
                            idPrefix="handover-{index}"
                        />
                    </Stack>
                </div>
            {/each}
            {#each silent as row (row.accountId)}
                <p class="text-sm text-ink-700" data-test="handover-account-none">
                    {t('externalApi.handover.accountNone', lang, { account: row.accountName })}
                </p>
            {/each}
        {/if}
    </Stack>
</SectionForm>

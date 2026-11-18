<script lang="ts">
    import { api } from '$lib/api/api';
    import type { TMarketplace } from '$lib/components/marketplaces/types';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import type { ISectionSuperForm } from '$lib/forms/section-form';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { exchangeSupported } from '$lib/marketplaces/exchange-support';
    import { marketplacesPayload, marketplacesSchema } from '$lib/validation/external-api';
    import MarketplaceCodeMap from './MarketplaceCodeMap.svelte';
    import type { TMarketplaceMatches } from './types';

    /**
     * The section «Маркетплейсы» of «Внешний API»: for each account, the marketplace's code in the fulfilment centre's own list
     * (`MarketplaceCodeMap`). A row with no saved code gets the proposed one — a code that equals the adapter's — chosen, and the
     * section then offers «Сохранить» at once, so the owner accepts the proposal with one press. An account whose marketplace
     * cannot exchange with a fulfilment centre yet (its adapter holds none of the parts that write to the exchange) says so
     * instead of its box and is proposed no code. Saved as one form with `PUT /api/v1/external-api/marketplaces`, which leaves
     * the hand-over settings alone.
     */
    interface IProps {
        lang: TLocale;
        matches: TMarketplaceMatches;
        /** The adapters (`GET /api/v1/marketplaces`), for what each can do; without them every account is matched as before. */
        marketplaces?: TMarketplace[];
        onSaved: (matches: TMarketplaceMatches) => void;
    }

    let { lang, matches, marketplaces = [], onSaved }: IProps = $props();

    // The form is made once, from the matches as they were when the section was drawn.
    // svelte-ignore state_referenced_locally
    const drawn = matches;
    // svelte-ignore state_referenced_locally
    const adapters = marketplaces;
    /** For each row: its marketplace cannot exchange with a fulfilment centre yet. */
    const unsupported = drawn.rows.map(
        (row) => !exchangeSupported(adapters.find((one) => one.code === row.marketplace))
    );

    const made = spaForm(
        marketplacesSchema,
        { rows: drawn.rows.map((row) => ({ accountId: row.accountId, fcMarketplace: row.fcMarketplace ?? '' })) },
        {
            id: 'external-api-marketplaces-form',
            resetForm: false,
            dataType: 'json',
            saved: 'section',
            send: async (values) => {
                const result = await api.put<TMarketplaceMatches>(
                    '/api/v1/external-api/marketplaces',
                    marketplacesPayload(values)
                );
                if (result.ok) onSaved(result.data);
                return result;
            }
        }
    );
    // The proposed codes are chosen as if the owner had chosen them: the section is then changed, and «Сохранить» is offered.
    const proposals = drawn.rows.flatMap((row, index) =>
        row.fcMarketplace === undefined && row.proposed !== undefined && !unsupported[index]
            ? [{ index, code: row.proposed }]
            : []
    );
    if (proposals.length > 0) {
        made.form.update((values) => {
            for (const { index, code } of proposals) values.rows[index].fcMarketplace = code;
            return values;
        });
    }
    const { form, errors } = made;
    const section = made as unknown as ISectionSuperForm;
    const rowErrors = $derived(
        drawn.rows.map((_row, index) => tError($errors.rows?.[index]?.fcMarketplace, lang) || undefined)
    );
</script>

<SectionForm
    {lang}
    id="external-api-marketplaces"
    title={t('externalApi.marketplaces.title', lang)}
    help={t('externalApi.marketplaces.help', lang)}
    form={section}
>
    <Stack>
        <p class="text-sm text-ink-700" data-test="external-api-marketplaces-intro">
            {t('externalApi.marketplaces.intro', lang)}
        </p>
        {#if drawn.rows.length === 0}
            <EmptyState message={t('externalApi.marketplaces.empty', lang)} dataTest="codes-empty" />
        {:else}
            <MarketplaceCodeMap
                {lang}
                rows={drawn.rows}
                fcMarketplaces={drawn.fcMarketplaces}
                selected={$form.rows.map((row) => row.fcMarketplace)}
                onSelect={(index, code) => ($form.rows[index].fcMarketplace = code)}
                errors={rowErrors}
                {unsupported}
                idPrefix="codes"
            />
        {/if}
    </Stack>
</SectionForm>

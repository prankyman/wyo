<script lang="ts">
    import { api } from '$lib/api/api';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import type { ISectionSuperForm } from '$lib/forms/section-form';
    import { spaForm } from '$lib/forms/super-form';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { warehousesPayload, warehousesSchema } from '$lib/validation/external-api';
    import type { TWarehouseMatches } from './types';
    import WarehouseMatchTable from './WarehouseMatchTable.svelte';

    /**
     * The section «Склады» of «Внешний API»: each warehouse of the seller on a marketplace beside the fulfilment centre's warehouse
     * that stands for it (`WarehouseMatchTable`). Only the orders of a matched warehouse are sent. Saved as one form with
     * `PUT /api/v1/external-api/warehouses`; the page is told the matches as the wyo server stored them.
     */
    interface IProps {
        lang: TLocale;
        matches: TWarehouseMatches;
        onSaved: (matches: TWarehouseMatches) => void;
    }

    let { lang, matches, onSaved }: IProps = $props();

    // The form is made once, from the matches as they were when the section was drawn.
    // svelte-ignore state_referenced_locally
    const drawn = matches;

    const made = spaForm(
        warehousesSchema,
        {
            rows: drawn.rows.map((row) => ({
                accountId: row.accountId,
                warehouseNumber: row.warehouseNumber,
                fcWarehouse: row.fcWarehouse ?? ''
            }))
        },
        {
            id: 'external-api-warehouses-form',
            resetForm: false,
            dataType: 'json',
            saved: 'section',
            send: async (values) => {
                const result = await api.put<TWarehouseMatches>(
                    '/api/v1/external-api/warehouses',
                    warehousesPayload(values)
                );
                if (result.ok) onSaved(result.data);
                return result;
            }
        }
    );
    const { form, errors } = made;
    const section = made as unknown as ISectionSuperForm;
    const rowErrors = $derived(
        drawn.rows.map((_row, index) => tError($errors.rows?.[index]?.fcWarehouse, lang) || undefined)
    );
</script>

<SectionForm
    {lang}
    id="external-api-warehouses"
    title={t('externalApi.warehouses.title', lang)}
    help={t('externalApi.warehouses.help', lang)}
    form={section}
>
    <Stack>
        <p class="text-sm text-ink-700" data-test="external-api-warehouses-intro">
            {t('externalApi.warehouses.intro', lang)}
        </p>
        {#if drawn.rows.length === 0}
            <EmptyState message={t('externalApi.warehouses.empty', lang)} dataTest="warehouses-empty" />
        {:else}
            <WarehouseMatchTable
                {lang}
                rows={drawn.rows}
                fcWarehouses={drawn.fcWarehouses}
                selected={$form.rows.map((row) => row.fcWarehouse)}
                onSelect={(index, code) => ($form.rows[index].fcWarehouse = code)}
                errors={rowErrors}
                idPrefix="warehouses"
            />
        {/if}
    </Stack>
</SectionForm>

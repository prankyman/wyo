<script lang="ts">
    import Field from '$lib/components/ui/Field.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TFcWarehouse, TWarehouseMatch } from './types';

    /**
     * Each warehouse of the seller on a marketplace beside a choice of the fulfilment centre's warehouses: the one that stands for
     * it. Only the orders of a matched warehouse are sent to the fulfilment centre, so a row that matches none says so and says why,
     * in the row itself. A warehouse with the marketplace's automatic assembly (`autoAssembly`: the marketplace itself assembles and packs
     * its orders) warns in its row, and may still be matched. A fulfilment centre that named no warehouse is told once above the rows. The chosen warehouses are the
     * parent form's (`selected`, one for each row, in the rows' order; a choice is told to `onSelect`); a refusal is laid on its
     * row (`errors`, already worded).
     */
    interface IProps {
        lang: TLocale;
        rows: TWarehouseMatch[];
        fcWarehouses: TFcWarehouse[];
        /** The chosen fulfilment warehouse's code for each row, `''` for none. */
        selected: string[];
        /** The owner chose `code` (`''` for none) for the row at `index`. */
        onSelect: (index: number, code: string) => void;
        /** The refusal of each row, already worded. */
        errors?: (string | undefined)[];
        /** The rows are `<idPrefix>-row`, a row's field `<idPrefix>-<n>-field`, its box `<idPrefix>-<n>-box`. */
        idPrefix: string;
    }

    let { lang, rows, fcWarehouses, selected, onSelect, errors = [], idPrefix }: IProps = $props();

    const options = $derived(fcWarehouses.map((one) => ({ value: one.code, label: one.name })));
</script>

<Stack dataTest={idPrefix}>
    {#if fcWarehouses.length === 0}
        <NoteBox tone="warning" dataTest="{idPrefix}-no-fc">{t('externalApi.warehouses.noFc', lang)}</NoteBox>
    {/if}
    {#each rows as row, index (`${row.accountId}:${row.warehouseNumber}`)}
        <div data-test="{idPrefix}-row">
            <Stack>
                <Field
                    label={t('externalApi.warehouses.row', lang, {
                        warehouse: row.warehouseName,
                        account: row.accountName
                    })}
                    help={t('externalApi.warehouses.row.help', lang, { number: row.warehouseNumber })}
                    for="{idPrefix}-{index}-box"
                    testId="{idPrefix}-{index}"
                    dataTest="{idPrefix}-{index}-field"
                    error={errors[index]}
                >
                    <SelectBox
                        id="{idPrefix}-{index}-box"
                        value={selected[index]}
                        onchange={(event) => onSelect(index, event.currentTarget.value)}
                        blank={t('externalApi.warehouses.blank', lang)}
                        {options}
                        invalid={Boolean(errors[index])}
                        class="sm:max-w-md"
                        dataTest="{idPrefix}-{index}-box"
                    />
                </Field>
                {#if selected[index] === ''}
                    <p class="text-sm text-warn-800" data-test="{idPrefix}-{index}-unmatched">
                        {t('externalApi.warehouses.unmatched', lang)}
                    </p>
                {/if}
                {#if row.autoAssembly}
                    <NoteBox tone="warning" dataTest="{idPrefix}-{index}-auto-assembly">
                        {t('externalApi.warehouses.autoAssembly', lang)}
                    </NoteBox>
                {/if}
            </Stack>
        </div>
    {/each}
</Stack>

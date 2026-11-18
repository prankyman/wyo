<script lang="ts">
    import Field from '$lib/components/ui/Field.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TFcMarketplace, TMarketplaceMatch } from './types';

    /**
     * Each marketplace account of the seller beside the marketplace's code in the fulfilment centre's own list: the fulfilment
     * centre has its own name for each marketplace it takes orders of, and wyo cannot guess it. When one of the fulfilment centre's
     * codes equals the adapter's, wyo proposes it and the proposed code is the one chosen for a row that has none saved yet. An
     * account with no code sends no order. An account whose key may only read cannot take an order into assembly, so its row warns
     * with the remedy. An account whose marketplace cannot exchange with a fulfilment centre yet (`unsupported`) says so in a note
     * instead of its box: there is nothing to match for it. The chosen codes are the parent form's (`selected`, one for each row,
     * in the rows' order; a choice is told to `onSelect`); a refusal is laid on its row (`errors`, already worded).
     */
    interface IProps {
        lang: TLocale;
        rows: TMarketplaceMatch[];
        fcMarketplaces: TFcMarketplace[];
        /** The chosen fulfilment-centre code for each row, `''` for none. */
        selected: string[];
        /** The owner chose `code` (`''` for none) for the row at `index`. */
        onSelect: (index: number, code: string) => void;
        errors?: (string | undefined)[];
        /** For each row, in the rows' order: the marketplace of the account cannot exchange with a fulfilment centre yet. */
        unsupported?: boolean[];
        /** The rows are `<idPrefix>-row`, a row's field `<idPrefix>-<n>-field`, its box `<idPrefix>-<n>-box`. */
        idPrefix: string;
    }

    let { lang, rows, fcMarketplaces, selected, onSelect, errors = [], unsupported = [], idPrefix }: IProps = $props();

    const options = $derived(fcMarketplaces.map((one) => ({ value: one.code, label: one.name })));
    const nameOf = (code: string) => fcMarketplaces.find((one) => one.code === code)?.name ?? code;
</script>

<Stack dataTest={idPrefix}>
    {#each rows as row, index (row.accountId)}
        <div data-test="{idPrefix}-row">
            {#if unsupported[index]}
                <NoteBox tone="neutral" dataTest="{idPrefix}-{index}-unsupported">
                    {t('externalApi.marketplaces.unsupported', lang, { account: row.accountName })}
                </NoteBox>
            {:else}
                <Stack>
                    <Field
                        label={row.accountName}
                        help={t('externalApi.marketplaces.row.help', lang, { account: row.accountName })}
                        for="{idPrefix}-{index}-box"
                        testId="{idPrefix}-{index}"
                        dataTest="{idPrefix}-{index}-field"
                        error={errors[index]}
                    >
                        <SelectBox
                            id="{idPrefix}-{index}-box"
                            value={selected[index]}
                            onchange={(event) => onSelect(index, event.currentTarget.value)}
                            blank={t('externalApi.marketplaces.blank', lang)}
                            {options}
                            invalid={Boolean(errors[index])}
                            class="sm:max-w-md"
                            dataTest="{idPrefix}-{index}-box"
                        />
                    </Field>
                    {#if row.fcMarketplace === undefined && row.proposed !== undefined && selected[index] === row.proposed}
                        <p class="text-sm text-ink-700" data-test="{idPrefix}-{index}-proposed">
                            {t('externalApi.marketplaces.proposed', lang, { name: nameOf(row.proposed) })}
                        </p>
                    {/if}
                    {#if selected[index] === ''}
                        <p class="text-sm text-warn-800" data-test="{idPrefix}-{index}-unmatched">
                            {t('externalApi.marketplaces.unmatched', lang)}
                        </p>
                    {/if}
                    {#if row.readOnly}
                        <NoteBox tone="warning" dataTest="{idPrefix}-{index}-read-only">
                            {t('externalApi.marketplaces.readOnly', lang, { account: row.accountName })}
                        </NoteBox>
                    {/if}
                </Stack>
            {/if}
        </div>
    {/each}
</Stack>

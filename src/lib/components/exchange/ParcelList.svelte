<script lang="ts">
    import FactList from '$lib/components/ui/FactList.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { formatDateTime } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TOrderParcel } from './types';

    /**
     * An order's parcels at the fulfilment centre (`OrderExchangeDetails.parcels`): for each, the marketplace's own number of the
     * part the parcel became and whether its label was attached at the fulfilment centre, with the moment. A marketplace that
     * splits an order packed in several parcels gives each part its own number, and one part may keep the order's own number; a
     * number that is the order's own says so in words, which for a single parcel means the marketplace did not split the order. A marketplace that numbers its boxes gives the box the parcel became: its number is a fact of the parcel, left out when the marketplace gave none. Parcels are numbered only when there are several. An order with no parcel yet (the fulfilment
     * centre has not finished packing) draws nothing, never an empty block.
     */
    interface IProps {
        lang: TLocale;
        /** The parcels in the order of their numbers; may be empty. */
        parcels: TOrderParcel[];
        /** The marketplace's own number of the order, to tell a parcel that kept it from one the marketplace numbered. */
        orderNumber: string;
        /** The block is `<testId>`, its «?» `<testId>-help`; a parcel's facts are `<testId>-<n>-<fact>` (n from 1) and its number `<testId>-<n>-position`. */
        testId?: string;
    }

    let { lang, parcels, orderNumber, testId = 'parcels' }: IProps = $props();

    function factsOf(parcel: TOrderParcel) {
        const labelValue = parcel.labelSent
            ? parcel.labelSentAt
                ? t('parcels.label.sentAt', lang, { at: formatDateTime(parcel.labelSentAt, lang) })
                : t('parcels.label.sent', lang)
            : t('parcels.label.notSent', lang);
        const facts = [
            {
                key: 'marketplaceNumber',
                label: t('parcels.fact.marketplaceNumber', lang),
                value: !parcel.marketplaceNumber
                    ? t('parcels.fact.marketplaceNumber.none', lang)
                    : parcel.marketplaceNumber === orderNumber
                      ? t('parcels.fact.marketplaceNumber.same', lang, { number: parcel.marketplaceNumber })
                      : parcel.marketplaceNumber,
                help: t('parcels.fact.marketplaceNumber.help', lang)
            },
            {
                key: 'label',
                label: t('parcels.fact.label', lang),
                value: labelValue,
                help: t('parcels.fact.label.help', lang)
            }
        ];
        return parcel.box
            ? [
                  ...facts,
                  {
                      key: 'box',
                      label: t('parcels.fact.box', lang),
                      value: parcel.box,
                      help: t('parcels.fact.box.help', lang)
                  }
              ]
            : facts;
    }
</script>

{#if parcels.length > 0}
    <Stack dataTest={testId}>
        <HeadingWithHelp
            text={t('parcels.title', lang)}
            help={t('parcels.help', lang)}
            level="h4"
            testId="{testId}-help"
        />
        {#each parcels as parcel (parcel.number)}
            <Stack>
                {#if parcels.length > 1}
                    <p class="text-xs font-medium text-ink-700" data-test="{testId}-{parcel.number}-position">
                        {t('parcels.position', lang, { n: parcel.number, total: parcels.length })}
                    </p>
                {/if}
                <FactList items={factsOf(parcel)} dataTest="{testId}-{parcel.number}" />
            </Stack>
        {/each}
    </Stack>
{/if}

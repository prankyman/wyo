<script lang="ts">
    import FactList from '$lib/components/ui/FactList.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import { formatDateTime } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { EXTERNAL_API_TONE } from './state';
    import type { TExternalApi } from './types';

    /**
     * What the fulfilment centre's system answered at the last check (`GET /api/v1/external-api`, and the answer of «Проверить и
     * сохранить»): the state in its tone with a «?» that says what to do about it, the address, the name the fulfilment centre
     * gives this seller, the last four characters of the key (never more), the marketplaces it accepts orders of, and when it was
     * checked. What the fulfilment centre did not tell is left out. A setting that is not filled shows its state only.
     */
    interface IProps {
        lang: TLocale;
        connection: TExternalApi;
        /** The state is `<testId>-state`, its «?» `<testId>-state-help`, the facts `<testId>-facts-<name>`. */
        testId: string;
    }

    let { lang, connection, testId }: IProps = $props();

    const facts = $derived.by(() => {
        if (connection.state === 'empty') return [];
        const items: { key: string; label: string; value: string; help: string }[] = [];
        const add = (key: string, value: string | undefined) => {
            if (value === undefined || value === '') return;
            items.push({
                key,
                value,
                label: t(`externalApi.result.${key}`, lang),
                help: t(`externalApi.result.${key}.help`, lang)
            });
        };
        add('address', connection.address);
        add('seller', connection.sellerName);
        add(
            'key',
            connection.keyLast4 ? t('externalApi.result.key.value', lang, { last4: connection.keyLast4 }) : undefined
        );
        add(
            'marketplaces',
            connection.marketplaces
                ? connection.marketplaces.length > 0
                    ? connection.marketplaces.map((marketplace) => marketplace.name).join(', ')
                    : t('externalApi.result.marketplaces.none', lang)
                : undefined
        );
        add('checkedAt', connection.checkedAt ? formatDateTime(connection.checkedAt, lang) : undefined);
        return items;
    });
</script>

<Stack dataTest={testId}>
    <div>
        <StatusBadgeWithHelp
            label={t(`externalApi.state.${connection.state}`, lang)}
            tone={EXTERNAL_API_TONE[connection.state]}
            help={t(`externalApi.state.${connection.state}.help`, lang)}
            size="md"
            testId="{testId}-state"
        />
    </div>
    {#if facts.length > 0}
        <FactList items={facts} dataTest="{testId}-facts" />
    {/if}
</Stack>

<script lang="ts">
    import MarketplaceCodeMap from '$lib/components/external-api/MarketplaceCodeMap.svelte';
    import type { TFcMarketplace, TMarketplaceMatch } from '$lib/components/external-api/types';

    /** `MarketplaceCodeMap` with the chosen codes held here, the way a form holds them. */
    interface IProps {
        rows: TMarketplaceMatch[];
        fcMarketplaces: TFcMarketplace[];
        start: string[];
        errors?: (string | undefined)[];
    }

    let { rows, fcMarketplaces, start, errors = [] }: IProps = $props();

    // svelte-ignore state_referenced_locally
    let selected = $state([...start]);
</script>

<MarketplaceCodeMap
    lang="ru"
    {rows}
    {fcMarketplaces}
    {selected}
    onSelect={(index, code) => (selected[index] = code)}
    {errors}
    idPrefix="codes"
/>

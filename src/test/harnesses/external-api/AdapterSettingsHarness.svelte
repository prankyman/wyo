<script lang="ts">
    import AdapterSettingsForm from '$lib/components/external-api/AdapterSettingsForm.svelte';
    import type { TMarketplace } from '$lib/components/marketplaces/types';

    /** `AdapterSettingsForm` with the values held here, shown as text so a test can read what the boxes wrote. */
    interface IProps {
        marketplace: TMarketplace;
        start: Record<string, string>;
        errors?: Record<string, string | undefined>;
    }

    let { marketplace, start, errors = {} }: IProps = $props();

    // svelte-ignore state_referenced_locally
    let values = $state({ ...start });
</script>

<AdapterSettingsForm
    lang="ru"
    {marketplace}
    {values}
    onChange={(name, value) => (values[name] = value)}
    {errors}
    idPrefix="adapter"
/>
<output data-test="adapter-values">{JSON.stringify(values)}</output>

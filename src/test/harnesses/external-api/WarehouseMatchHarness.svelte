<script lang="ts">
    import WarehouseMatchTable from '$lib/components/external-api/WarehouseMatchTable.svelte';
    import type { TFcWarehouse, TWarehouseMatch } from '$lib/components/external-api/types';

    /** `WarehouseMatchTable` with the chosen warehouses held here, the way a form holds them. */
    interface IProps {
        rows: TWarehouseMatch[];
        fcWarehouses: TFcWarehouse[];
        start: string[];
        errors?: (string | undefined)[];
    }

    let { rows, fcWarehouses, start, errors = [] }: IProps = $props();

    // svelte-ignore state_referenced_locally
    let selected = $state([...start]);
</script>

<WarehouseMatchTable
    lang="ru"
    {rows}
    {fcWarehouses}
    {selected}
    onSelect={(index, code) => (selected[index] = code)}
    {errors}
    idPrefix="warehouses"
/>

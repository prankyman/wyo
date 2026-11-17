<script lang="ts">
    import ClickableRow from '$lib/components/ui/ClickableRow.svelte';
    import StretchedLink from '$lib/components/ui/StretchedLink.svelte';

    /** A desktop order row the way a list screen builds one: a checkbox, the record's link, a second link. */
    let { href = '/items/item-1' }: { href?: string } = $props();
</script>

<table>
    <tbody>
        <ClickableRow dataTest="order-row" data-row-id="o-1" class="border-b border-ink-100">
            <td><input type="checkbox" name="orderIds" value="o-1" data-test="order-select" /></td>
            <td><StretchedLink {href} dataTest="order-link">№ 12</StretchedLink></td>
            <td><a href="/shipments/s-1" data-test="order-shipment">S-3</a></td>
        </ClickableRow>
    </tbody>
</table>

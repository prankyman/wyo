<script lang="ts">
    import Button from '$lib/components/ui/Button.svelte';
    import DisclosureClose from '$lib/components/ui/DisclosureClose.svelte';
    import { disclosureFormOf } from '$lib/components/ui/disclosure-form';

    /** A form that binds nothing to any data: its fields are plain boxes, its answer is pressed by the test. */
    interface IProps {
        withClose?: boolean;
    }

    let { withClose = true }: IProps = $props();
    const disclosure = disclosureFormOf();
</script>

<input name="title" value="Zebra" data-test="title" />
<input type="checkbox" name="active" checked data-test="active" />
<input type="hidden" name="id" value="p1" data-test="hidden" />
<div>
    <Button dataTest="succeed" onclick={() => disclosure?.answered(true)}>Save</Button>
    <Button dataTest="refuse" onclick={() => disclosure?.answered(false)}>Refused save</Button>
    {#if withClose}<DisclosureClose />{/if}
</div>

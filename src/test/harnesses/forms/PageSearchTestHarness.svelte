<script lang="ts">
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import { pageSearch } from '$lib/forms/page-search.svelte';
    import { z } from 'zod';

    /** A list screen's filter the way `pageSearch` builds one: its boxes follow the page's data. */
    let { status }: { status: string } = $props();

    const { form, enhance, submitting } = pageSearch(z.object({ status: z.string() }), () => ({ status }), 'harness');
</script>

<GuardedForm method="GET" superform={{ enhance, submitting }} dataTest="harness-filter">
    <input name="status" bind:value={$form.status} data-test="harness-status" />
</GuardedForm>

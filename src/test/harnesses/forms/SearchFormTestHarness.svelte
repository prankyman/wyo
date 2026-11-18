<script lang="ts">
    import Button from '$lib/components/ui/Button.svelte';
    import GuardedForm from '$lib/components/ui/GuardedForm.svelte';
    import { searchForm } from '$lib/forms/search-form';
    import { z } from 'zod';

    /** A search box the way a list screen builds one: a GET form with a query and a status filter. */
    const schema = z.object({ q: z.string().max(5, 'q.tooLong'), status: z.string() });

    const { form, errors, enhance, submitting } = searchForm(
        schema,
        { q: '', status: 'open' },
        { id: 'harness-search', address: () => new URL('http://localhost/app/orders?page=3&sort=date') }
    );
</script>

<GuardedForm method="GET" superform={{ enhance, submitting }} dataTest="harness-search">
    <input name="q" bind:value={$form.q} data-test="search-q" />
    <input name="status" bind:value={$form.status} data-test="search-status" />
    {#if $errors.q}<p data-test="search-q-error">{$errors.q[0]}</p>{/if}
    <Button type="submit" dataTest="search-submit">Find</Button>
</GuardedForm>

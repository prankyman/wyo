<script lang="ts">
    import { onMount } from 'svelte';
    import { api, type TMeta } from '$lib/api/api';
    import { refusalKey } from '$lib/api/refusal-key';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { t } from '$lib/i18n/i18n';

    // The program's name and version, from the wyo server's own API.
    let meta = $state<TMeta | null>(null);
    let refusal = $state<string | null>(null);

    onMount(async () => {
        const result = await api.getMeta();
        if (result.ok) meta = result.data;
        else refusal = refusalKey(result.refusal.code);
    });
</script>

<Stack as="section" dataTest="start">
    <PageHeading
        title={meta?.name ?? t('start.title')}
        help={t('start.help')}
        headingTestId="start-name"
        helpTestId="start-help"
    />
    <p>{t('start.subtitle')}</p>
    {#if meta}
        <p class="text-ink-600" data-test="start-version">{t('start.version', undefined, { version: meta.version })}</p>
    {:else if refusal}
        <p class="text-stop-700" role="alert" data-test="start-refusal">{t(refusal)}</p>
    {:else}
        <p class="text-ink-500" data-test="start-loading">{t('start.loading')}</p>
    {/if}
</Stack>

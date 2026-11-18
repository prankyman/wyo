<script lang="ts">
    import { api } from '$lib/api/api';
    import { refusalKey } from '$lib/api/refusal-key';
    import InstallationStateCard from '$lib/components/overview/InstallationStateCard.svelte';
    import Button from '$lib/components/ui/Button.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import StepStrip from '$lib/components/ui/StepStrip.svelte';
    import { getLocale, t } from '$lib/i18n/i18n';
    import { installationCards, type IInstallationCard, type TInstallation } from '$lib/overview/installation-cards';
    import { overviewSteps } from '$lib/overview/steps';
    import { onMount } from 'svelte';

    /**
     * «Обзор»: where the seller stands (the strip of steps, «Сейчас: …» and one sentence on what to do next) and how the
     * installation stands (a card each for the version, the certificate and the last copy of the database, from
     * `GET /api/v1/installation`). The page adds nothing else: the cards are `InstallationStateCard`, the strip is `StepStrip`.
     */
    const lang = getLocale();
    const { steps, now } = overviewSteps(lang);

    let cards = $state<IInstallationCard[] | null>(null);
    let refusal = $state<string | null>(null);

    async function read(): Promise<void> {
        refusal = null;
        const result = await api.get<TInstallation>('/api/v1/installation');
        if (result.ok) cards = installationCards(result.data, lang);
        else refusal = t(refusalKey(result.refusal.code), lang);
    }

    onMount(() => {
        void read();
    });
</script>

<Stack as="section" dataTest="overview">
    <PageHeading
        title={t('overview.title', lang)}
        help={t('overview.help', lang)}
        headingTestId="overview-heading"
        helpTestId="overview-help"
    />
    <StepStrip {lang} {steps} nowText={now} dataTest="overview-steps" />
    <p class="text-sm text-ink-700" data-test="overview-next">{t('overview.next', lang)}</p>
    {#if cards}
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" data-test="overview-cards">
            {#each cards as card (card.key)}
                <InstallationStateCard
                    testId="overview-card-{card.key}"
                    title={card.title}
                    help={card.help}
                    state={card.state}
                    sentence={card.sentence}
                    action={card.action}
                />
            {/each}
        </div>
    {:else if refusal}
        <NoteBox tone="error" role="alert" dataTest="overview-refusal">{refusal}</NoteBox>
        <div>
            <Button variant="secondary" dataTest="overview-retry" onclick={read}>{t('common.retry', lang)}</Button>
        </div>
    {:else}
        <p class="text-ink-600" data-test="overview-loading">{t('overview.loading', lang)}</p>
    {/if}
</Stack>

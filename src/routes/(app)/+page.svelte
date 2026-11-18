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
    import { externalApiCard, type TExternalApiOverview } from '$lib/overview/external-api-card';
    import { marketplaceCards, type TMarketplacesOverview } from '$lib/overview/marketplace-cards';
    import { currentStep, overviewSteps, type TOverviewStep } from '$lib/overview/steps';
    import { onMount } from 'svelte';

    /**
     * «Обзор»: where the seller stands (the strip of steps, «Сейчас: …» and one sentence on what to do next), how the installation
     * stands (a card each for the version, the certificate and the last copy of the database, from `GET /api/v1/installation`), and
     * how the marketplaces stand (the accounts, the orders waiting and past their deadline, the last successful reading, from
     * `GET /api/v1/overview/marketplaces`), and how «Внешний API» stands (`GET /api/v1/overview/external-api`). Once an account is
     * added the strip moves on to the orders, and once «Внешний API» is filled to its fourth step. The page adds nothing else:
     * the cards are `InstallationStateCard`, the strip is `StepStrip`.
     */
    const lang = getLocale();
    /** What the line under the strip says at each step: the strip's steps with no text of their own say what the step before says. */
    const NEXT_KEY: Record<TOverviewStep, string> = {
        install: 'overview.next',
        keys: 'overview.next',
        orders: 'overview.nextOrders',
        api: 'overview.nextApi'
    };

    let cards = $state<IInstallationCard[] | null>(null);
    let marketplaces = $state<TMarketplacesOverview | null>(null);
    let refusal = $state<string | null>(null);
    let marketplacesRefusal = $state<string | null>(null);
    let externalApi = $state<TExternalApiOverview | null>(null);
    let externalApiRefusal = $state<string | null>(null);

    const current = $derived(currentStep(marketplaces?.accounts ?? 0, externalApi?.state ?? 'empty'));
    const strip = $derived(overviewSteps(lang, current));
    const allCards = $derived(
        cards
            ? [
                  ...cards,
                  ...(marketplaces ? marketplaceCards(marketplaces, lang) : []),
                  ...(externalApi ? [externalApiCard(externalApi, lang)] : [])
              ]
            : null
    );

    async function read(): Promise<void> {
        refusal = null;
        marketplacesRefusal = null;
        externalApiRefusal = null;
        const [result, counts, external] = await Promise.all([
            api.get<TInstallation>('/api/v1/installation'),
            api.get<TMarketplacesOverview>('/api/v1/overview/marketplaces'),
            api.get<TExternalApiOverview>('/api/v1/overview/external-api')
        ]);
        if (counts.ok) marketplaces = counts.data;
        else marketplacesRefusal = t(refusalKey(counts.refusal.code), lang);
        if (external.ok) externalApi = external.data;
        else externalApiRefusal = t(refusalKey(external.refusal.code), lang);
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
    <StepStrip {lang} steps={strip.steps} nowText={strip.now} dataTest="overview-steps" />
    <p class="text-sm text-ink-700" data-test="overview-next">
        {t(NEXT_KEY[current], lang)}
    </p>
    {#if marketplacesRefusal && allCards}
        <NoteBox tone="error" role="alert" dataTest="overview-marketplaces-refusal">{marketplacesRefusal}</NoteBox>
    {/if}
    {#if externalApiRefusal && allCards}
        <NoteBox tone="error" role="alert" dataTest="overview-external-api-refusal">{externalApiRefusal}</NoteBox>
    {/if}
    {#if allCards}
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" data-test="overview-cards">
            {#each allCards as card (card.key)}
                <InstallationStateCard
                    testId="overview-card-{card.key}"
                    title={card.title}
                    help={card.help}
                    state={card.state}
                    sentence={card.sentence}
                    action={card.action}
                    moreActions={card.moreActions}
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

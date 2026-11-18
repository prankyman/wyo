<script lang="ts">
    import { api } from '$lib/api/api';
    import { refusalKey } from '$lib/api/refusal-key';
    import ConnectionSection from '$lib/components/external-api/ConnectionSection.svelte';
    import EventsModeForm from '$lib/components/external-api/EventsModeForm.svelte';
    import HandoverSettingsSection from '$lib/components/external-api/HandoverSettingsSection.svelte';
    import MarketplacesSection from '$lib/components/external-api/MarketplacesSection.svelte';
    import SectionLoading from '$lib/components/external-api/SectionLoading.svelte';
    import type {
        TEventsSetting,
        TExternalApi,
        TMarketplaceMatches,
        TWarehouseMatches
    } from '$lib/components/external-api/types';
    import WarehousesSection from '$lib/components/external-api/WarehousesSection.svelte';
    import type { TMarketplace, TMarketplaceList } from '$lib/components/marketplaces/types';
    import Button from '$lib/components/ui/Button.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { getLocale, t } from '$lib/i18n/i18n';
    import { onMount } from 'svelte';

    /**
     * «Внешний API»: the setting that points the wyo server at a fulfilment centre's system. It starts with what a fulfilment
     * centre is, what the wyo server will do by itself once the exchange works and what it never does. Until the setting is
     * filled the page holds that and the section «Подключение» only; once it is filled, four sections follow — «Склады»,
     * «Маркетплейсы», «События» and «Передача заказов маркетплейсу» — each a folded form with its own «Сохранить», each reading its
     * own data (a section whose data did not come keeps its place with «Повторить»). The exchange works only while the connection's
     * last check passed, and the page says so otherwise. Disconnecting returns the page to its first state. It also asks which
     * kind of wyo server it talks to, so that a build for development and tests may be given a plain `http://` address.
     */
    const lang = getLocale();

    let connection = $state<TExternalApi | null>(null);
    /** The wyo server is a build for development and tests (`GET /api/v1/meta`), which takes a plain `http://` address. */
    let allowPlainHttp = $state(false);
    let refusal = $state<string | null>(null);
    /** Counts disconnections: «Подключение» is drawn anew after one, so its boxes are empty. */
    let disconnections = $state(0);

    let warehouses = $state<TWarehouseMatches | null>(null);
    let warehousesRefusal = $state<string | null>(null);
    let marketplaceMatches = $state<TMarketplaceMatches | null>(null);
    let adapters = $state<TMarketplace[]>([]);
    let marketplacesRefusal = $state<string | null>(null);
    let events = $state<TEventsSetting | null>(null);
    let eventsRefusal = $state<string | null>(null);

    const filled = $derived(connection !== null && connection.state !== 'empty');

    async function readWarehouses(): Promise<void> {
        warehousesRefusal = null;
        const result = await api.get<TWarehouseMatches>('/api/v1/external-api/warehouses');
        if (result.ok) warehouses = result.data;
        else warehousesRefusal = result.refusal.code;
    }

    async function readMarketplaces(): Promise<void> {
        marketplacesRefusal = null;
        const [matches, list] = await Promise.all([
            api.get<TMarketplaceMatches>('/api/v1/external-api/marketplaces'),
            api.get<TMarketplaceList>('/api/v1/marketplaces')
        ]);
        if (matches.ok && list.ok) {
            marketplaceMatches = matches.data;
            adapters = list.data.marketplaces;
        } else {
            const failed = !matches.ok ? matches : list;
            marketplacesRefusal = failed.ok ? null : failed.refusal.code;
        }
    }

    async function readEvents(): Promise<void> {
        eventsRefusal = null;
        const result = await api.get<TEventsSetting>('/api/v1/external-api/events');
        if (result.ok) events = result.data;
        else eventsRefusal = result.refusal.code;
    }

    function readParts(): void {
        void readWarehouses();
        void readMarketplaces();
        void readEvents();
    }

    async function read(): Promise<void> {
        refusal = null;
        const result = await api.get<TExternalApi>('/api/v1/external-api');
        if (!result.ok) {
            refusal = t(refusalKey(result.refusal.code), lang);
            return;
        }
        connection = result.data;
        if (result.data.state !== 'empty') readParts();
    }

    /** The connection was saved or checked: the sections that depend on it are read the first time it is filled. */
    function changed(next: TExternalApi): void {
        const first = !filled && next.state !== 'empty';
        connection = next;
        if (first) readParts();
    }

    function disconnected(): void {
        connection = { state: 'empty', eventsMode: 'poll' };
        warehouses = marketplaceMatches = events = null;
        warehousesRefusal = marketplacesRefusal = eventsRefusal = null;
        disconnections += 1;
    }

    const loadingProps = (id: string, titleKey: string, helpKey: string) => ({
        lang,
        id,
        title: t(titleKey, lang),
        help: t(helpKey, lang)
    });

    async function readMode(): Promise<void> {
        const meta = await api.getMeta();
        if (meta.ok) allowPlainHttp = meta.data.mode !== 'release';
    }

    onMount(() => {
        void read();
        void readMode();
    });
</script>

<Stack as="section" dataTest="external-api">
    <PageHeading
        title={t('externalApi.title', lang)}
        help={t('externalApi.help', lang)}
        headingTestId="external-api-heading"
        helpTestId="external-api-help"
    />
    <NoteBox tone="info" dataTest="external-api-about">
        <Stack>
            <HeadingWithHelp
                text={t('externalApi.about.title', lang)}
                help={t('externalApi.about.title.help', lang)}
                level="h2"
                look="section"
                testId="external-api-about-help"
            />
            <p>{t('externalApi.about.what', lang)}</p>
            <p>{t('externalApi.about.does', lang)}</p>
            <p>{t('externalApi.about.never', lang)}</p>
        </Stack>
    </NoteBox>
    {#if connection}
        {#if filled && connection.state !== 'working'}
            <NoteBox tone="warning" dataTest="external-api-not-working">{t('externalApi.notWorking', lang)}</NoteBox>
        {/if}
        {#key disconnections}
            <ConnectionSection {lang} {connection} {allowPlainHttp} onChanged={changed} onDisconnected={disconnected} />
        {/key}
        {#if filled}
            {#if warehouses}
                <WarehousesSection {lang} matches={warehouses} onSaved={(next) => (warehouses = next)} />
            {:else}
                <SectionLoading
                    {...loadingProps(
                        'external-api-warehouses',
                        'externalApi.warehouses.title',
                        'externalApi.warehouses.help'
                    )}
                    refusal={warehousesRefusal}
                    onRetry={readWarehouses}
                />
            {/if}
            {#if marketplaceMatches}
                <MarketplacesSection
                    {lang}
                    matches={marketplaceMatches}
                    marketplaces={adapters}
                    onSaved={(next) => (marketplaceMatches = next)}
                />
            {:else}
                <SectionLoading
                    {...loadingProps(
                        'external-api-marketplaces',
                        'externalApi.marketplaces.title',
                        'externalApi.marketplaces.help'
                    )}
                    refusal={marketplacesRefusal}
                    onRetry={readMarketplaces}
                />
            {/if}
            {#if events}
                <EventsModeForm {lang} {events} onSaved={(next) => (events = next)} />
            {:else}
                <SectionLoading
                    {...loadingProps('external-api-events', 'externalApi.events.title', 'externalApi.events.titleHelp')}
                    refusal={eventsRefusal}
                    onRetry={readEvents}
                />
            {/if}
            {#if marketplaceMatches}
                <HandoverSettingsSection
                    {lang}
                    matches={marketplaceMatches}
                    marketplaces={adapters}
                    onSaved={(next) => (marketplaceMatches = next)}
                />
            {:else}
                <SectionLoading
                    {...loadingProps(
                        'external-api-handover',
                        'externalApi.handover.title',
                        'externalApi.handover.help'
                    )}
                    refusal={marketplacesRefusal}
                    onRetry={readMarketplaces}
                />
            {/if}
        {:else}
            <p class="text-sm text-ink-700" data-test="external-api-empty-next">{t('externalApi.empty.next', lang)}</p>
        {/if}
    {:else if refusal}
        <NoteBox tone="error" role="alert" dataTest="external-api-refusal">{refusal}</NoteBox>
        <div>
            <Button variant="secondary" dataTest="external-api-retry" onclick={read}>{t('common.retry', lang)}</Button>
        </div>
    {:else}
        <p class="text-ink-600" data-test="external-api-loading">{t('externalApi.loading', lang)}</p>
    {/if}
</Stack>

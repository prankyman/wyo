<script lang="ts">
    import { api } from '$lib/api/api';
    import { refusalKey } from '$lib/api/refusal-key';
    import AccountCard from '$lib/components/marketplaces/AccountCard.svelte';
    import KeyForm from '$lib/components/marketplaces/KeyForm.svelte';
    import type { TAccount, TAccountList, TMarketplace, TMarketplaceList } from '$lib/components/marketplaces/types';
    import Button from '$lib/components/ui/Button.svelte';
    import DisclosureForm from '$lib/components/ui/DisclosureForm.svelte';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { getLocale, hasMessage, t } from '$lib/i18n/i18n';
    import { onMount } from 'svelte';

    /**
     * «Маркетплейсы»: the seller's marketplace accounts, one `AccountCard` each, in the order they were added, and «Добавить
     * аккаунт», which opens the key form of the chosen adapter (`KeyForm`). The page reads the adapters
     * (`GET /api/v1/marketplaces`) and the accounts (`GET /api/v1/accounts`) and keeps the list: a card's answer replaces its
     * account, a deleted one leaves the list, an added one joins it. With no account yet the page says what to do and opens the
     * form on arrival. When the wyo server holds more than one adapter the form asks first which marketplace the account is on.
     */
    const lang = getLocale();

    let marketplaces = $state<TMarketplace[] | null>(null);
    let accounts = $state<TAccount[]>([]);
    let refusal = $state<string | null>(null);
    let adding = $state(false);
    let chosen = $state('');

    const adapter = $derived(marketplaces?.find((marketplace) => marketplace.code === chosen) ?? marketplaces?.[0]);

    async function read(): Promise<void> {
        refusal = null;
        const [adapters, list] = await Promise.all([
            api.get<TMarketplaceList>('/api/v1/marketplaces'),
            api.get<TAccountList>('/api/v1/accounts')
        ]);
        if (!adapters.ok || !list.ok) {
            const failed = !adapters.ok ? adapters : list;
            if (!failed.ok) refusal = t(refusalKey(failed.refusal.code), lang);
            return;
        }
        accounts = list.data.accounts;
        chosen = adapters.data.marketplaces[0]?.code ?? '';
        adding = list.data.accounts.length === 0;
        marketplaces = adapters.data.marketplaces;
    }

    function changed(account: TAccount): void {
        accounts = accounts.map((one) => (one.id === account.id ? account : one));
    }

    function added(account: TAccount): void {
        accounts = [...accounts, account];
    }

    function deleted(id: number): void {
        accounts = accounts.filter((one) => one.id !== id);
    }

    const nameOf = (code: string) =>
        hasMessage(`marketplace.${code}.name`, lang) ? t(`marketplace.${code}.name`, lang) : code;

    onMount(() => {
        void read();
    });
</script>

<Stack as="section" dataTest="marketplaces">
    <PageHeading
        title={t('marketplaces.title', lang)}
        help={t('marketplaces.help', lang)}
        headingTestId="marketplaces-heading"
        helpTestId="marketplaces-help"
    />
    {#if marketplaces}
        <Stack dataTest="marketplaces-loaded">
            {#if accounts.length === 0}
                <EmptyState message={t('accounts.empty', lang)} dataTest="marketplaces-empty" />
            {/if}
            {#each accounts as account (account.id)}
                <AccountCard
                    {lang}
                    {account}
                    marketplace={marketplaces.find((marketplace) => marketplace.code === account.marketplace)}
                    onChanged={changed}
                    onDeleted={deleted}
                />
            {/each}
            {#if adapter}
                <DisclosureForm
                    {lang}
                    bind:open={adding}
                    after="stay"
                    label={t('accounts.add', lang)}
                    testId="account-add"
                    openerTestId="account-add-open"
                >
                    <Stack>
                        {#if marketplaces.length > 1}
                            <Field
                                label={t('marketplaces.chooseMarketplace', lang)}
                                help={t('marketplaces.chooseMarketplace.help', lang)}
                                for="marketplaces-choose-box"
                                testId="marketplaces-choose-field"
                            >
                                <SelectBox
                                    id="marketplaces-choose-box"
                                    bind:value={chosen}
                                    options={marketplaces.map((marketplace) => ({
                                        value: marketplace.code,
                                        label: nameOf(marketplace.code)
                                    }))}
                                    class="sm:max-w-md"
                                    dataTest="marketplaces-choose"
                                />
                            </Field>
                        {/if}
                        {#key adapter.code}
                            <KeyForm {lang} marketplace={adapter} id="account-add-form" onSaved={added} />
                        {/key}
                    </Stack>
                </DisclosureForm>
            {/if}
        </Stack>
    {:else if refusal}
        <NoteBox tone="error" role="alert" dataTest="marketplaces-refusal">{refusal}</NoteBox>
        <div>
            <Button variant="secondary" dataTest="marketplaces-retry" onclick={read}>{t('common.retry', lang)}</Button>
        </div>
    {:else}
        <p class="text-ink-600" data-test="marketplaces-loading">{t('marketplaces.loading', lang)}</p>
    {/if}
</Stack>

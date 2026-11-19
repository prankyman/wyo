<script lang="ts">
    import { api, type TApiResult } from '$lib/api/api';
    import ButtonWithHelp from '$lib/components/ui/ButtonWithHelp.svelte';
    import ConfirmationModal from '$lib/components/ui/ConfirmationModal.svelte';
    import DisclosureForm from '$lib/components/ui/DisclosureForm.svelte';
    import FactList from '$lib/components/ui/FactList.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { ApiAction } from '$lib/forms/api-action.svelte';
    import { formatDate, formatDateTime } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import { tPlural } from '$lib/i18n/plural';
    import type { TLocale } from '$lib/i18n/types';
    import { exchangeSupported } from '$lib/marketplaces/exchange-support';
    import { accountRefusalLookup, accountTextKey } from '$lib/marketplaces/refusals';
    import AccountChangeForm from './AccountChangeForm.svelte';
    import AccountStateBadge from './AccountStateBadge.svelte';
    import KeyForm from './KeyForm.svelte';
    import MarketplaceMark from './MarketplaceMark.svelte';
    import NotificationAddressCard from './NotificationAddressCard.svelte';
    import ShopList from './ShopList.svelte';
    import type { TAccount, TMarketplace } from './types';

    /**
     * One marketplace account on «Маркетплейсы»: the marketplace's mark, the account's own name and state, and what its key and
     * its readings say — the seller, the fields of the key that are not secret in full (an account number), the key's last four
     * characters (never more), what the key allows, the day it ends, the last successful reading, the last refusal in words, the
     * hours to hand an order over (only for a marketplace that does not give each order its own deadline), and how many warehouses
     * a courier takes orders from within hours (Express). A test key, a missing
     * category and the warehouses the seller delivers from itself each say in a note what they hide. An account whose marketplace
     * keeps several shops under one key lists them (`ShopList`), and one whose marketplace sends notifications shows its
     * notification address, its state and the steps that connect it (`NotificationAddressCard`). Every refusal and note is
     * worded by the one lookup of the accounts' texts (`accountTextKey`): the marketplace's own words first. Under them: «Заменить ключ» (`KeyForm`), «Изменить» (`AccountChangeForm`), «Проверить ключ
     * сейчас», «Отключить» or «Включить», and «Удалить» for an account from which no order ever came, behind a window.
     *
     * Every answer is told to the page (`onChanged`, `onDeleted`), which holds the list; the card keeps nothing of its own.
     */
    interface IProps {
        lang: TLocale;
        account: TAccount;
        /** The account's adapter; without it the key cannot be replaced (its form is drawn from the adapter). */
        marketplace?: TMarketplace;
        onChanged: (account: TAccount) => void;
        onDeleted: (id: number) => void;
    }

    let { lang, account, marketplace, onChanged, onDeleted }: IProps = $props();

    const action = new ApiAction();
    let asking = $state(false);

    /** A press that is one call about this account; the account as it now stands goes to the page. */
    async function press(path: string, savedKey?: string): Promise<void> {
        const result = await action.run(
            () => api.post<TAccount>(path, {}),
            savedKey,
            accountRefusalLookup(account.marketplace)
        );
        if (result?.ok) onChanged(result.data);
    }

    async function remove(): Promise<void> {
        const result: TApiResult<unknown> | null = await action.run(
            () => api.delete(`/api/v1/accounts/${account.id}`),
            undefined,
            accountRefusalLookup(account.marketplace)
        );
        if (result?.ok) onDeleted(account.id);
    }

    /**
     * The last refusal in words: the marketplace's own text of its code, else the shared one, else the general one that sends to
     * the journal, with the marketplace's answer status when the refusal carries one.
     */
    function refusalText(refusal: NonNullable<TAccount['lastRefusal']>): string {
        const key = accountTextKey('refusal', refusal.code, account.marketplace, lang) ?? 'accounts.refusal.unknown';
        return t(key, lang, refusal.status === undefined ? {} : { status: refusal.status });
    }

    /** The marketplace gives each order its own deadline, so the owner's hours to hand an order over mean nothing for it. */
    const givesDeadline = $derived(marketplace?.givesDeadline === true);

    const facts = $derived.by(() => {
        const items: { key: string; label: string; value: string; help: string }[] = [];
        const fact = (key: string, value: string) =>
            items.push({
                key,
                value,
                label: t(`accounts.fact.${key}`, lang),
                help: t(`accounts.fact.${key}.help`, lang)
            });
        if (account.sellerName) fact('seller', account.sellerName);
        // The fields of the key that are not secret, in full, each named and explained by the texts of the form that asks for it.
        for (const shown of account.keyFields ?? []) {
            items.push({
                key: `key_${shown.name}`,
                value: shown.value,
                label: t(`marketplace.${account.marketplace}.key.${shown.name}.label`, lang),
                help: t(`marketplace.${account.marketplace}.key.${shown.name}.help`, lang)
            });
        }
        fact('key', t('accounts.fact.key.value', lang, { last4: account.keyLast4 }));
        // Whether a key may write matters only to a marketplace the exchange writes to; a marketplace that is only read says
        // nothing of it, so the card claims nothing.
        if (exchangeSupported(marketplace))
            fact(
                'access',
                t(account.readOnly ? 'accounts.fact.access.readOnly' : 'accounts.fact.access.readWrite', lang)
            );
        if (account.keyExpiresAt) fact('expires', formatDate(account.keyExpiresAt, lang));
        fact(
            'lastSuccess',
            account.lastSuccessAt ? formatDateTime(account.lastSuccessAt, lang) : t('accounts.fact.never', lang)
        );
        if (account.lastRefusal) {
            fact(
                'lastRefusal',
                t('accounts.fact.lastRefusal.value', lang, {
                    text: refusalText(account.lastRefusal),
                    date: formatDateTime(account.lastRefusal.at, lang)
                })
            );
        }
        if (account.expressWarehouses) fact('expressWarehouses', String(account.expressWarehouses));
        if (!givesDeadline) {
            fact('deadlineHours', t('accounts.fact.deadlineHours.value', lang, { hours: account.deadlineHours }));
        }
        return items;
    });

    const testNote = $derived(accountTextKey('note', 'test', account.marketplace, lang));
    const sellerDeliversNote = $derived(
        account.sellerDeliversWarehouses ? accountTextKey('note', 'sellerDelivers', account.marketplace, lang) : null
    );
    const missingNote = (category: string) => {
        const key = accountTextKey('note', `missing.${category}`, account.marketplace, lang);
        return key ? t(key, lang) : t('accounts.note.missingCategory', lang, { category });
    };
</script>

<article
    class="rounded border border-ink-200 bg-white p-4"
    data-test="account-card"
    data-account-id={account.id}
    data-state={account.state}
>
    <Stack>
        <div class="flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0">
                <h2 class="text-lg font-medium break-words text-ink-900" data-test="account-name">{account.name}</h2>
                <MarketplaceMark {lang} code={account.marketplace} mark={marketplace?.mark} dataTest="account-mark" />
            </div>
            <AccountStateBadge
                {lang}
                state={account.state}
                keyExpiresAt={account.keyExpiresAt}
                testId="account-state"
            />
        </div>
        <FactList items={facts} dataTest="account-facts" />
        {#if account.shops && account.shops.length > 0}
            <ShopList {lang} shops={account.shops} testId="account-shops" />
        {/if}
        {#if account.notifications}
            <NotificationAddressCard
                {lang}
                code={account.marketplace}
                notifications={account.notifications}
                testId="account-notifications"
            />
        {/if}
        {#if account.test && testNote}
            <NoteBox tone="info" dataTest="account-note-test">{t(testNote, lang)}</NoteBox>
        {/if}
        {#if sellerDeliversNote}
            <NoteBox tone="info" dataTest="account-note-seller-delivers">
                {tPlural(sellerDeliversNote, account.sellerDeliversWarehouses ?? 0, lang)}
            </NoteBox>
        {/if}
        {#each account.missingCategories as category (category)}
            <NoteBox tone="warning" dataTest="account-note-missing-{category}">{missingNote(category)}</NoteBox>
        {/each}
        {#if marketplace}
            <DisclosureForm
                {lang}
                label={t('accounts.replaceKey', lang)}
                testId="account-replace"
                openerTestId="account-replace-open"
            >
                <KeyForm {lang} {marketplace} {account} id="account-{account.id}-replace" onSaved={onChanged} />
            </DisclosureForm>
        {/if}
        <DisclosureForm
            {lang}
            label={t('accounts.change', lang)}
            testId="account-change"
            openerTestId="account-change-open"
        >
            <AccountChangeForm {lang} {account} {givesDeadline} id="account-{account.id}-change" onSaved={onChanged} />
        </DisclosureForm>
        <div>
            <div class="flex flex-wrap items-center gap-2">
                <ButtonWithHelp
                    variant="secondary"
                    pending={action.pending}
                    dataTest="account-check"
                    help={t('accounts.check.help', lang)}
                    helpLabel={t('accounts.check', lang)}
                    helpTestId="account-check-help"
                    onclick={() => press(`/api/v1/accounts/${account.id}/check`, 'accounts.checked')}
                >
                    {t('accounts.check', lang)}
                </ButtonWithHelp>
                {#if account.state === 'disabled'}
                    <ButtonWithHelp
                        variant="secondary"
                        dataTest="account-enable"
                        help={t('accounts.enable.help', lang)}
                        helpLabel={t('accounts.enable', lang)}
                        helpTestId="account-enable-help"
                        onclick={() => press(`/api/v1/accounts/${account.id}/enable`)}
                    >
                        {t('accounts.enable', lang)}
                    </ButtonWithHelp>
                {:else}
                    <ButtonWithHelp
                        variant="secondary"
                        dataTest="account-disable"
                        help={t('accounts.disable.help', lang)}
                        helpLabel={t('accounts.disable', lang)}
                        helpTestId="account-disable-help"
                        onclick={() => press(`/api/v1/accounts/${account.id}/disable`)}
                    >
                        {t('accounts.disable', lang)}
                    </ButtonWithHelp>
                {/if}
                {#if !account.hasOrders}
                    <ButtonWithHelp
                        variant="danger-outline"
                        dataTest="account-delete"
                        help={t('accounts.delete.help', lang)}
                        helpLabel={t('accounts.delete', lang)}
                        helpTestId="account-delete-help"
                        onclick={() => (asking = true)}
                    >
                        {t('accounts.delete', lang)}
                    </ButtonWithHelp>
                {/if}
            </div>
            <FormAnswer saved={action.saved} error={action.error} dataTest="account-answer" />
        </div>
    </Stack>
</article>

<ConfirmationModal
    bind:open={asking}
    title={t('accounts.deleteWindow.title', lang, { name: account.name })}
    message={t('accounts.deleteWindow.message', lang)}
    confirmText={t('accounts.deleteWindow.confirm', lang)}
    cancelText={t('app.common.cancel', lang)}
    dataTest="account-delete-window"
    onConfirm={() => {
        void remove();
    }}
/>

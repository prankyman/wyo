<script lang="ts">
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { t } from '$lib/i18n/i18n';
    import { tPlural } from '$lib/i18n/plural';
    import type { TLocale } from '$lib/i18n/types';
    import type { TAccountState } from './types';

    /**
     * The state of a marketplace account (`api/wyo.yaml`, `AccountState`): a badge in the tone of the state and a «?» that says
     * what the state means and what to do. A key that ends soon says in how many days, from the day the key ends.
     */
    interface IProps {
        lang: TLocale;
        state: TAccountState;
        /** When the key ends: a key that ends soon counts its days from it. */
        keyExpiresAt?: string;
        size?: 'sm' | 'md';
        /** The badge is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { lang, state, keyExpiresAt, size = 'md', testId }: IProps = $props();

    const TONE: Readonly<Record<TAccountState, TTone>> = {
        working: 'ok',
        key_expiring: 'warn',
        key_expired: 'stop',
        key_refused: 'stop',
        no_connection: 'stop',
        disabled: 'off'
    };
    const DAY_MS = 86_400_000;

    const label = $derived.by(() => {
        if (state !== 'key_expiring' || keyExpiresAt === undefined) return t(`accounts.state.${state}`, lang);
        const days = Math.max(0, Math.ceil((new Date(keyExpiresAt).getTime() - Date.now()) / DAY_MS));
        return tPlural('accounts.state.key_expiring', days, lang);
    });
</script>

<StatusBadgeWithHelp {label} tone={TONE[state]} help={t(`accounts.state.${state}.help`, lang)} {size} {testId} />

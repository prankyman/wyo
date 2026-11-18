<script lang="ts">
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TJournalOutcome } from './types';

    /**
     * How one request to a marketplace ended (`api/wyo.yaml`, `JournalOutcome`), in one of six words: done, limit met, refused,
     * server error, no connection, key not accepted. A badge in the tone of the outcome with a «?» that says what happened and
     * what the wyo server does about it.
     */
    interface IProps {
        lang: TLocale;
        outcome: TJournalOutcome;
        /** The badge is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { lang, outcome, testId }: IProps = $props();

    const TONE: Readonly<Record<TJournalOutcome, TTone>> = {
        done: 'ok',
        limited: 'warn',
        refused: 'stop',
        server_error: 'stop',
        no_connection: 'warn',
        key_refused: 'stop'
    };
</script>

<StatusBadgeWithHelp
    label={t(`journal.outcome.${outcome}`, lang)}
    tone={TONE[outcome]}
    help={t(`journal.outcome.${outcome}.help`, lang)}
    {testId}
/>

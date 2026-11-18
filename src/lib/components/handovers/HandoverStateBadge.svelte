<script lang="ts">
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { THandoverState } from './types';

    /** The state of a hand-over (`api/wyo.yaml`, `HandoverState`): a badge in the tone of the state with a «?» that says what it means. */
    interface IProps {
        lang: TLocale;
        state: THandoverState;
        size?: 'sm' | 'md';
        /** The badge is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { lang, state, size = 'sm', testId }: IProps = $props();

    const TONE: Readonly<Record<THandoverState, TTone>> = {
        working: 'wait',
        waiting: 'warn',
        problem: 'stop',
        documents_sent: 'ok',
        withdrawn: 'off',
        completed: 'ok'
    };
</script>

<StatusBadgeWithHelp
    label={t(`handovers.state.${state}`, lang)}
    tone={TONE[state]}
    help={t(`handovers.state.${state}.help`, lang)}
    {size}
    {testId}
/>

<script lang="ts">
    import { TONE_TEXT_CLASS } from '$lib/components/ui/status-tones';
    import { formatDateTime } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { deadlineMatters, deadlineTone } from '$lib/orders/deadline';
    import type { TOrderState } from './types';

    /**
     * The time by which an FBS order must be handed to the marketplace: its creation time plus the account's hours. It turns to
     * the attention tone 6 hours before it and to the stop tone after it, and says so in words too («скоро срок», «срок прошёл»),
     * because a state is never shown by colour alone. An order already handed over or cancelled shows a quiet dash, as an FBO
     * order with no deadline does: its deadline no longer matters.
     */
    interface IProps {
        lang: TLocale;
        deadlineAt?: string;
        state: TOrderState;
        testId?: string;
    }

    let { lang, deadlineAt, state, testId }: IProps = $props();

    const tone = $derived(deadlineTone(deadlineAt, state, new Date()));
    const shown = $derived(deadlineAt !== undefined && deadlineMatters(state) ? deadlineAt : undefined);
    const CLASS = {
        none: TONE_TEXT_CLASS.off,
        normal: 'text-ink-800',
        attention: `${TONE_TEXT_CLASS.warn} font-medium`,
        overdue: `${TONE_TEXT_CLASS.stop} font-medium`
    } as const;
</script>

<span class="tabular-nums {CLASS[tone]}" data-tone={tone} data-test={testId}>
    {shown === undefined ? '—' : formatDateTime(shown, lang)}
    {#if tone === 'attention'}
        · {t('orders.deadline.soon', lang)}
    {:else if tone === 'overdue'}
        · {t('orders.deadline.overdue', lang)}
    {/if}
</span>

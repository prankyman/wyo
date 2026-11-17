<script lang="ts">
    import HelpTooltip from './HelpTooltip.svelte';
    import StatusBadge from './StatusBadge.svelte';
    import { toneClass, type TTone } from './status-tones';

    /**
     * A status badge and the «?» that says what the status means: the badge in the colour of its tone
     * (`status-tones.ts`), the «?» right beside it. The kind badges (a badge of one kind of status) draw
     * through it when their status has a help text.
     */
    interface IProps {
        label: string;
        tone: TTone;
        /** What the status means; without it there is no «?». */
        help?: string;
        size?: 'sm' | 'md';
        /** The badge is `<testId>`, its «?» `<testId>-help`. */
        testId?: string;
    }

    let { label, tone, help, size = 'sm', testId }: IProps = $props();
</script>

<span class="inline-flex items-center gap-1">
    <StatusBadge {label} colorClass={toneClass(tone)} {size} dataTest={testId} />
    {#if help}
        <HelpTooltip tooltip={help} ariaLabel={help} testId={testId ? `${testId}-help` : 'status-help'} />
    {/if}
</span>

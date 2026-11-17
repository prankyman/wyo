<script lang="ts">
    import type { Snippet } from 'svelte';
    import type { HTMLAttributes } from 'svelte/elements';
    import { cn } from '$lib/utils/cn';

    /**
     * The coloured note of the interface: one sentence or a few lines in a tinted box — a warning
     * (amber, the theme's `warn`), a refusal or a danger (red, the theme's `stop`), a success or a confirmation (green, the theme's `ok`), a plain piece of information (the brand colour, the theme's `brand`), or a
     * quiet state that is none of those (grey, the theme's `ink`, `neutral`: a run that waits for its hour, a line that is neither right nor wrong).
     * Drawn by hand it gets accidental differences in padding, shade and role; the look
     * of each tone is decided here once (`note-box-policy.test.ts` keeps it the only copy). The words come from the
     * caller, already translated.
     *
     * A refusal is an alert — a screen reader says it at once; a success is a polite status; a warning or a piece of
     * information that stands on the page from the start has no live role, because nothing just happened.
     */
    interface IProps extends Omit<HTMLAttributes<HTMLDivElement>, 'class' | 'role' | 'children'> {
        tone: 'warning' | 'error' | 'success' | 'info' | 'neutral';
        /**
         * The live role; by tone when left out: `alert` for an error, `status` for a success, none otherwise. `note` marks a
         * box whose words are an aside to the main content (an offer's oversize notice) without announcing anything.
         */
        role?: 'alert' | 'status' | 'note' | 'none';
        /** `sm` (the default) or `xs` text. */
        size?: 'sm' | 'xs';
        dataTest?: string;
        dataTour?: string;
        /** Classes of the box's place on the page: its margins, its grid cell; a padding here replaces the box's own. */
        class?: string;
        children: Snippet;
    }

    /** Every other attribute (`data-confirmed`, `aria-live`) goes to the box, so a spec can address its state. */
    let {
        tone,
        role: givenRole,
        size = 'sm',
        dataTest,
        dataTour,
        class: className,
        children,
        ...rest
    }: IProps = $props();

    const TONE_CLASS: Record<IProps['tone'], string> = {
        warning: 'border-warn-200 bg-warn-50 text-warn-800',
        error: 'border-stop-200 bg-stop-50 text-stop-800',
        success: 'border-ok-200 bg-ok-50 text-ok-800',
        info: 'border-brand-200 bg-brand-50 text-brand-800',
        neutral: 'border-ink-200 bg-ink-50 text-ink-700'
    };
    const DEFAULT_ROLE: Record<IProps['tone'], 'alert' | 'status' | 'none'> = {
        warning: 'none',
        error: 'alert',
        success: 'status',
        info: 'none',
        neutral: 'none'
    };

    const role = $derived(givenRole ?? DEFAULT_ROLE[tone]);
</script>

<div
    {...rest}
    role={role === 'none' ? undefined : role}
    class={cn('rounded border px-4 py-2', size === 'xs' ? 'text-xs' : 'text-sm', TONE_CLASS[tone], className)}
    data-test={dataTest}
    data-tour={dataTour}
    data-tone={tone}
>
    {@render children()}
</div>

<script lang="ts">
    import { cn } from '$lib/utils/cn';
    import HelpTooltip from './HelpTooltip.svelte';
    import type { IHelpLink } from './types';

    /**
     * The small heading over a block with its «?»: a card's section title, a label-sized caption over a
     * group of facts. Written by hand, the caption's classes
     * (`text-xs tracking-widest text-ink-500 uppercase`) get copied from file to file; the three looks are decided here
     * once. The «?» stands beside the heading, not inside it, so a screen reader reads the heading's words alone.
     */
    interface IProps {
        text: string;
        /** The text of the «?»: paragraphs separated by blank lines. Without it the heading stands alone, in its look. */
        help?: string;
        /** Links under the «?»'s text, each opening in a new tab. */
        helpLinks?: IHelpLink[];
        /** The accessible name of the «?»; the heading's text when left out. */
        ariaLabel?: string;
        level?: 'h2' | 'h3' | 'h4' | 'h5';
        /**
         * `label`: the small grey capitals of a caption (the default); `section`: the heading of a card's section, as
         * `CollapsibleSection` draws its title; `title`: a window's or a page part's title in larger type.
         */
        look?: 'label' | 'section' | 'title';
        /** `data-test` prefix of the «?»: `<testId>-button` and `<testId>-tooltip`. */
        testId?: string;
        /** `data-test` of the heading element. */
        headingTestId?: string;
        /** The heading's `id`, for a section's `aria-labelledby`. */
        id?: string;
        helpPosition?: 'left' | 'center' | 'right';
        dataTour?: string;
        /** Classes of the row's place on the page. */
        class?: string;
    }

    let {
        text,
        help,
        helpLinks,
        ariaLabel,
        level = 'h2',
        look = 'label',
        testId,
        headingTestId,
        id,
        helpPosition = 'center',
        dataTour,
        class: className
    }: IProps = $props();

    const LOOK_CLASS: Record<NonNullable<IProps['look']>, string> = {
        label: 'text-xs tracking-widest text-ink-500 uppercase',
        section: 'text-sm font-medium text-ink-900',
        title: 'text-lg text-ink-900'
    };
</script>

<div class={cn('flex items-center gap-1', className)} data-tour={dataTour}>
    <svelte:element this={level} {id} class={LOOK_CLASS[look]} data-test={headingTestId}>{text}</svelte:element>
    {#if help}
        <HelpTooltip tooltip={help} links={helpLinks} ariaLabel={ariaLabel ?? text} {testId} position={helpPosition} />
    {/if}
</div>

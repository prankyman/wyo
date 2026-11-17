<script lang="ts">
    import type { Snippet } from 'svelte';
    import { cn } from '$lib/utils/cn';
    import BackLink from './BackLink.svelte';
    import HelpTooltip from './HelpTooltip.svelte';

    /**
     * The heading of a working page: the title and the page's ONE
     * «?» beside it (what the page is, why it is needed, its scenarios), with a
     * `BackLink` above it on a detail screen, the buttons of the page at the right, and a grey line under the title
     * when the page has one. Written by hand on every page it drifts; the look of the title is decided here once, and
     * `page-help-policy.test.ts` checks the one-«?» rule through this component. No introduction text is drawn under
     * the heading: that rule puts everything into the «?».
     */
    interface IProps {
        title: string;
        /** The text of the «?»: paragraphs separated by blank lines. */
        help: string;
        /** The accessible name of the «?»; the title when left out. */
        ariaLabel?: string;
        /** `data-test` of the title element. */
        headingTestId?: string;
        /** `data-test` prefix of the «?»: `<helpTestId>-button` and `<helpTestId>-tooltip`. */
        helpTestId: string;
        helpPosition?: 'left' | 'center' | 'right';
        /** The «← назад» link above the title of a detail screen. */
        back?: { href: string; label: string; dataTest: string; dataTour?: string; arrow?: boolean };
        /** The grey line under the title: a number, a status, a date. */
        subline?: Snippet;
        /** The buttons at the right of the title. */
        actions?: Snippet;
        /** A guided tour's anchor on the title. */
        tour?: string;
        /** A guided tour's anchor on the row that holds the title and its «?». */
        rowTour?: string;
        /** `h1` (the default) on a page; `h2` for a page drawn inside another page's heading. */
        level?: 'h1' | 'h2';
        /** `data-test` of the whole heading block. */
        dataTest?: string;
        /** Classes of the block's place on the page (its width). Never a margin: the page's `Stack` spaces it. */
        class?: string;
    }

    let {
        title,
        help,
        ariaLabel,
        headingTestId,
        helpTestId,
        helpPosition = 'center',
        back,
        subline,
        actions,
        tour,
        rowTour,
        level = 'h1',
        dataTest,
        class: className
    }: IProps = $props();
</script>

<div class={cn(className)} data-test={dataTest}>
    {#if back}
        <BackLink
            href={back.href}
            label={back.label}
            dataTest={back.dataTest}
            dataTour={back.dataTour}
            arrow={back.arrow}
        />
    {/if}
    <div class={cn('flex flex-wrap items-center justify-between gap-3', back && 'mt-2')}>
        <div class="flex min-w-0 items-center gap-1" data-tour={rowTour}>
            <svelte:element
                this={level}
                class="min-w-0 text-2xl break-words"
                data-test={headingTestId}
                data-tour={tour}
            >
                {title}
            </svelte:element>
            <HelpTooltip tooltip={help} ariaLabel={ariaLabel ?? title} testId={helpTestId} position={helpPosition} />
        </div>
        {#if actions}
            <div class="flex flex-wrap items-center gap-2">
                {@render actions()}
            </div>
        {/if}
    </div>
    {#if subline}
        <div class="mt-1 text-sm text-ink-500">
            {@render subline()}
        </div>
    {/if}
</div>

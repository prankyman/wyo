<script lang="ts">
    import SpinnerGapIcon from 'phosphor-svelte/lib/SpinnerGapIcon';
    import { cn } from '$lib/utils/cn';
    import { isNavigatingTo } from './page-change.svelte';
    import type { IStretchedLinkProps } from './types';

    /**
     * The link that opens a record, whose hit area covers the whole `ClickableCard` or
     * `ClickableRow` it sits in: its `::after` is stretched over the nearest `.clickable-surface`
     * (`src/app.css`). Its text is the record's name or number, so it is also what a screen reader
     * and a keyboard user reach — one link per record, with an accessible name.
     *
     * While SvelteKit is on its way to `href` it shows a spinner, says `aria-busy`, and a second
     * press is swallowed in the capture phase, before SvelteKit's own link handling sees it. Once the
     * page has been given back after `LOCK_MAX_MS` (`pageChange.lockExpired`) the change is taken as
     * dropped, and the link is a plain link again.
     */
    let { href, dataTest, ariaLabel, class: className, hreflang, children }: IStretchedLinkProps = $props();

    const opening = $derived(isNavigatingTo(href));

    function swallowWhileOpening(event: MouseEvent): void {
        if (!opening) return;
        event.preventDefault();
        event.stopImmediatePropagation();
    }
</script>

<a
    {href}
    {hreflang}
    data-stretched-link
    data-test={dataTest}
    aria-label={ariaLabel}
    aria-busy={opening || undefined}
    aria-disabled={opening || undefined}
    onclickcapture={swallowWhileOpening}
    class={cn(
        'inline-flex items-center gap-2 py-1 -my-1 font-medium touch-manipulation [-webkit-tap-highlight-color:transparent] focus-visible:outline-none aria-busy:cursor-wait',
        className
    )}
>
    {@render children()}
    {#if opening}
        <SpinnerGapIcon
            class="size-4 shrink-0 motion-safe:animate-spin"
            aria-hidden="true"
            data-test="stretched-link-spinner"
        />
    {/if}
</a>

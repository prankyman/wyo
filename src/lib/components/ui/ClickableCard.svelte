<script lang="ts">
    import { cn } from '$lib/utils/cn';
    import { CLICKABLE_CARD } from './press';
    import StretchedLink from './StretchedLink.svelte';
    import type { IClickableCardProps } from './types';

    /**
     * A card that opens its record wherever it is pressed (constitution, press feedback and single submission).
     *
     * `title` is the text of the one link on the card — the record's number or name — and that
     * link's hit area is stretched over the whole card (`StretchedLink`). Everything in
     * `children` is drawn on top of it: plain text passes the press through to the link, while
     * buttons, links, checkboxes, labels and selects are lifted above it (`.clickable-surface` in
     * `src/app.css`) and keep working on their own.
     *
     * Pressed, the card shrinks a little and darkens; while its record is opening it dims, the
     * link shows a spinner and a second press is swallowed. The keyboard reaches the link, and its
     * focus ring is drawn around the whole card.
     */
    let {
        as = 'div',
        href,
        dataTest,
        linkDataTest,
        ariaLabel,
        class: className,
        linkClass,
        hreflang,
        title,
        children,
        ...rest
    }: IClickableCardProps = $props();
</script>

<svelte:element this={as} class={cn(CLICKABLE_CARD, className)} data-test={dataTest} {...rest}>
    <StretchedLink {href} {hreflang} {ariaLabel} dataTest={linkDataTest} class={linkClass}>
        {@render title()}
    </StretchedLink>
    {#if children}
        {@render children()}
    {/if}
</svelte:element>

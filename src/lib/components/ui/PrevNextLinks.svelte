<script lang="ts">
    import { LINK_TARGET } from './press';

    /**
     * «Назад» and «Дальше» of a list that does not count its rows: the previous page while there is one, the
     * next while the list says it has more. `Pagination` is the pager of a list that knows its total. Nothing is drawn on
     * a list of one page. The row is `<dataTest>-pages`, the links `<dataTest>-prev` and `<dataTest>-next`.
     */
    interface IProps {
        /** The page shown, from 1. */
        page: number;
        /** There is a page after this one. */
        hasMore: boolean;
        /** The address of a page. */
        hrefFor: (page: number) => string;
        prevLabel: string;
        nextLabel: string;
        dataTest: string;
    }

    let { page, hasMore, hrefFor, prevLabel, nextLabel, dataTest }: IProps = $props();

    const linkClass = `${LINK_TARGET} text-sm text-ink-900 underline underline-offset-2`;
</script>

{#if page > 1 || hasMore}
    <p class="flex gap-5" data-test="{dataTest}-pages">
        {#if page > 1}
            <a class={linkClass} href={hrefFor(page - 1)} data-test="{dataTest}-prev">{prevLabel}</a>
        {/if}
        {#if hasMore}
            <a class={linkClass} href={hrefFor(page + 1)} data-test="{dataTest}-next">{nextLabel}</a>
        {/if}
    </p>
{/if}

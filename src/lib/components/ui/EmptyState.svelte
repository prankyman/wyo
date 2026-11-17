<script lang="ts">
    import type { Snippet } from 'svelte';
    import type { HTMLAttributes } from 'svelte/elements';
    import { cn } from '$lib/utils/cn';

    /**
     * «Nothing here yet»: the dashed grey box a list draws while it has no rows, or the same sentence as
     * a plain grey line where a box would be too much (a small block inside a card). Drawn by hand it drifts; the look is decided here once (`empty-state-policy.test.ts` keeps it the only copy). The sentence comes
     * from the caller, already translated; `children` is what the person may do about it — a link to the page that
     * adds the first row.
     */
    interface IProps extends Omit<HTMLAttributes<HTMLDivElement>, 'class' | 'children'> {
        message: string;
        /**
         * `box` (the default): the dashed box of a list with no rows; `inline`: a small dashed line standing where a thing
         * used to be (a deleted file in place of its link, a block of a card with nothing in it); `plain`: the grey line.
         */
        variant?: 'box' | 'inline' | 'plain';
        /** A lower box (`py-6` against `py-8`) inside a card or a narrow column. */
        compact?: boolean;
        /** Left off the printed page (`print:hidden`, which works from inside any page, unlike a page's own scoped `.no-print` rule). */
        noPrint?: boolean;
        dataTest?: string;
        dataTour?: string;
        /** Classes of the box's place on the page: its margins, its grid cell. */
        class?: string;
        children?: Snippet;
    }

    let {
        message,
        variant = 'box',
        compact = false,
        noPrint = false,
        dataTest,
        dataTour,
        class: className,
        children,
        ...rest
    }: IProps = $props();
</script>

<!-- Every other attribute (`data-empty-kind`) goes to the box, so a spec can tell the kinds of nothing apart. -->
<div
    {...rest}
    class={cn(
        'text-sm text-ink-500',
        variant !== 'plain' && 'rounded border border-dashed border-ink-300',
        variant === 'box' && 'px-4 text-center',
        variant === 'box' && (compact ? 'py-6' : 'py-8'),
        variant === 'inline' && 'px-3 py-2 break-words',
        noPrint && 'print:hidden',
        className
    )}
    data-test={dataTest}
    data-tour={dataTour}
>
    <!-- One block: a sentence alone leaves no stray space after it, so a test may match the whole text. -->
    {#if children}
        <p>{message}</p>
        <div class="mt-3">
            {@render children()}
        </div>
    {:else}
        <p>{message}</p>
    {/if}
</div>

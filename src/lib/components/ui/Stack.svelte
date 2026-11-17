<script lang="ts">
    import type { Snippet } from 'svelte';
    import type { HTMLAttributes } from 'svelte/elements';
    import { cn } from '$lib/utils/cn';

    /**
     * The one column of the interface: every
     * page and every stack of blocks inside a page — the sections of a form, the groups of a settings page, the blocks of a
     * section's body — is this flex column, and its gap of 16 px (`gap-4`) is the only space between two neighbouring blocks.
     * A block never brings an outer margin of its own and no stack spaces its blocks with `space-y-*`; an element that may
     * draw nothing is put inside only when it has content, because an empty flex item still takes a gap.
     * `stack-policy.test.ts` keeps the rule in the source and the screen sweeps measure it in the browser through
     * `data-stack` (`e2e/helpers/stack-gaps.ts`). A form wraps a Stack; the gap is not a prop.
     */
    interface IProps extends Omit<HTMLAttributes<HTMLElement>, 'class' | 'children' | 'id'> {
        /** The element: `div` by default, `section` for a page's root. */
        as?: 'div' | 'section';
        id?: string;
        dataTest?: string;
        dataTour?: string;
        /** Classes of the column's width or its place in a grid — never a gap or a margin. */
        class?: string;
        children: Snippet;
    }

    /** Every other attribute (`data-state`, `onsubmit`) goes to the column, so a spec can address its state. */
    let { as = 'div', id, dataTest, dataTour, class: className, children, ...rest }: IProps = $props();
</script>

<svelte:element
    this={as}
    {...rest}
    {id}
    class={cn('flex flex-col gap-4', className)}
    data-stack
    data-test={dataTest}
    data-tour={dataTour}
>
    {@render children()}
</svelte:element>

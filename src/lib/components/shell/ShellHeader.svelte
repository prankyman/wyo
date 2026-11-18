<script lang="ts">
    import type { Snippet } from 'svelte';

    /**
     * The bar of the header: at the left, whatever the shell passes as `left` (the program's name); at the right the header's
     * buttons in ONE row that never wraps, so they stay at the top right at every screen width, a phone's included — it is the
     * left side that gives way (`min-w-0`), never the buttons. The `<header>` itself, with its colour and its pinned position,
     * is drawn by the shell.
     */
    interface IProps {
        /** The left side: the program's name and what belongs to it. */
        left: Snippet;
        /** The header's buttons, in the order they are drawn from the left. */
        children: Snippet;
    }

    let { left, children }: IProps = $props();
</script>

<div class="flex h-16 flex-nowrap items-center justify-between gap-x-4 px-4" data-test="shell-header-bar">
    <div class="flex min-w-0 items-center gap-x-4" data-test="shell-header-left">
        {@render left()}
    </div>
    <div class="flex shrink-0 flex-nowrap items-center gap-x-2 md:gap-x-4" data-test="shell-header-buttons">
        {@render children()}
    </div>
</div>

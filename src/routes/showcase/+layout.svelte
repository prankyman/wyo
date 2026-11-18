<script lang="ts">
    import type { Snippet } from 'svelte';

    /**
     * The showcase page has no shell around it (no top bar, no menu: it exists only for development and tests), so this layout gives
     * it what the root layout no longer does: the main part of the document and the 16-pixel gutter every page keeps at its sides.
     */
    let { children }: { children: Snippet } = $props();
</script>

<main class="px-4 py-4">
    {@render children()}
</main>

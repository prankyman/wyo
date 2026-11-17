<script lang="ts">
    import '../app.css';
    import type { Snippet } from 'svelte';
    import PageChangeGuard from '$lib/components/ui/PageChangeGuard.svelte';
    import { getLocale } from '$lib/i18n/i18n';

    let { children }: { children: Snippet } = $props();
</script>

<!-- The page frame: one centred column with the side gutter. Plan 2 replaces it with the top bar and the menu. -->
<PageChangeGuard lang={getLocale()}>
    <main class="mx-auto w-full max-w-5xl px-4 py-4">
        {@render children()}
    </main>
</PageChangeGuard>

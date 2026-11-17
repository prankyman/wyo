<script lang="ts">
    import type { Snippet } from 'svelte';
    import { onMount } from 'svelte';

    let { children }: { children: Snippet } = $props();

    let portal = $state<HTMLDivElement | null>(null);
    let mounted = $state(false);

    onMount(() => {
        if (portal) {
            document.body.appendChild(portal);
            mounted = true;
        }

        return () => {
            // eslint-disable-next-line svelte/no-dom-manipulating -- this node was moved to <body> by hand, so Svelte cannot remove it
            portal?.remove();
        };
    });
</script>

<div bind:this={portal} data-test="portal-root" style:display={mounted ? 'contents' : 'none'}>
    {@render children()}
</div>

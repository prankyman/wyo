<script lang="ts">
    import IconTooltip from '$lib/components/ui/IconTooltip.svelte';

    /** One icon link with its tooltip, the way a header draws it. */
    let { onlyBelowMd = false }: { onlyBelowMd?: boolean } = $props();
</script>

<IconTooltip name="Sign out" description="Ends the sign-in on this device." {onlyBelowMd} testId="logout-tip">
    <a href="/sign-in" aria-label="Sign out" data-test="logout">×</a>
</IconTooltip>

<script lang="ts">
    import ShellHeader from '$lib/components/shell/ShellHeader.svelte';

    /** The header bar with a left side and three buttons, the way the shell fills it. */
</script>

<ShellHeader>
    {#snippet left()}<span data-test="harness-left">Logo</span>{/snippet}
    <span data-test="harness-button-1">1</span>
    <span data-test="harness-button-2">2</span>
    <span data-test="harness-button-3">3</span>
</ShellHeader>

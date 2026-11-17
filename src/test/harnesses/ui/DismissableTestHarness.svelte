<script lang="ts">
    import { dismissable } from '$lib/components/ui/dismissable';

    interface IProps {
        onDismiss: () => void;
    }

    let { onDismiss }: IProps = $props();

    let panel = $state<HTMLDivElement | undefined>();
    let trigger = $state<HTMLButtonElement | undefined>();

    // The way the helper is meant to be used: driven from an `$effect`, whose cleanup is the
    // teardown the helper returns.
    $effect(() => {
        if (!panel) return;
        return dismissable(panel, { onDismiss, trigger: () => trigger });
    });
</script>

<button type="button" bind:this={trigger} data-test="trigger">open</button>
<div bind:this={panel} data-test="panel">
    <button type="button" data-test="inside">inside</button>
</div>
<button type="button" data-test="outside">outside</button>

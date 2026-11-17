<script lang="ts">
    import Field from '$lib/components/ui/Field.svelte';

    /** One field with a box (or a tick box for the inline layout), the way a form draws it. */
    interface IProps {
        layout?: 'stacked' | 'inline' | 'row';
        withFor?: boolean;
        help?: string;
        error?: string | null;
        class?: string;
        labelHidden?: boolean;
        disabled?: boolean;
        /** A badge drawn on the label's row (`aside`). */
        badge?: string;
    }

    let {
        layout = 'stacked',
        withFor = true,
        help,
        error = null,
        class: className = '',
        labelHidden = false,
        disabled = false,
        badge
    }: IProps = $props();
</script>

<Field
    label="Tax number"
    {help}
    {error}
    {layout}
    {labelHidden}
    {disabled}
    for={withFor ? 'inn' : undefined}
    testId="client-inn"
    dataTest="client-inn-field"
    class={className}
>
    {#snippet aside()}{#if badge}<span data-test="client-inn-badge">{badge}</span>{/if}{/snippet}
    {#if layout === 'inline'}
        <input id="inn" type="checkbox" data-test="client-inn" />
    {:else}
        <input id="inn" data-test="client-inn" />
    {/if}
</Field>

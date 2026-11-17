<script lang="ts">
    import HelpTooltip from './HelpTooltip.svelte';

    interface IProps {
        /** The field's name, as the person reads it. */
        text: string;
        /** The text behind the «?» beside the name; without it the row has no button. */
        help?: string;
        /** `<testId>-label` is the row, `<testId>-help` the button's test id. */
        testId?: string;
    }

    let { text, help, testId = 'field' }: IProps = $props();
</script>

<!--
    The label row of a form field. It is as tall as its text, with or without a «?»: the «?» is the size of its icon, so the boxes of one grid row start at the same height. The row has no
    minimum height of its own: the «?» button is compact.
-->
<span class="flex items-center" data-test="{testId}-label">
    {text}
    {#if help}
        <HelpTooltip tooltip={help} ariaLabel={help} testId="{testId}-help" />
    {/if}
</span>

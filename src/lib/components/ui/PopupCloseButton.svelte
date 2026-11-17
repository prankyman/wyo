<script lang="ts">
    import XIcon from 'phosphor-svelte/lib/XIcon';
    import { getLocale, t } from '$lib/i18n/i18n';
    import Button from './Button.svelte';

    interface IProps {
        onclick: () => void;
        testId: string;
    }

    let { onclick, testId }: IProps = $props();
</script>

<!--
    The white cross in the top right corner of a help popup (`HelpTooltip`, `AttentionIcon`). On a phone the popup can
    cover the whole screen and leave no «outside» to tap, so the popup needs a way to close itself. It is a 44 × 44 px
    `Button` (the tap size every button meets) floated to the right, so the text wraps around it and never runs under it. It is deliberately not `sticky`: a cross
    that stayed in view while a long text scrolled inside the popup would sit on top of the lines passing under it. White
    on the popup's dark or amber background.
-->
<Button
    variant="ghost-inverse"
    {onclick}
    ariaLabel={t('common.close', getLocale())}
    class="float-right -mt-1 -mr-2 mb-1 ml-2"
    dataTest={testId}
>
    {#snippet icon()}<XIcon class="size-6" aria-hidden="true" />{/snippet}
</Button>

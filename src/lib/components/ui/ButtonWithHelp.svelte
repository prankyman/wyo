<script lang="ts">
    import Button from './Button.svelte';
    import HelpTooltip from './HelpTooltip.svelte';
    import type { IButtonProps } from './types';

    /**
     * A button with its «?» beside it: what pressing the button does, told in a
     * scenario, standing immediately to the right of the button on the same line. «Новая запись», «Закончить проверку»
     * and «Печать» are drawn with it.
     * Every other property is the `Button`'s own. With `fullWidth` the button takes the place the «?» leaves.
     */
    interface IProps extends IButtonProps {
        /** What the «?» says, already translated: paragraphs separated by blank lines. */
        help: string;
        /** `data-test` of the «?» (`<helpTestId>-button` is its button, `<helpTestId>-tooltip` its text). */
        helpTestId: string;
        /** The «?»'s accessible name: the button's words, already translated. */
        helpLabel: string;
        /** Classes of the row that holds the button and its «?» (its place in a grid). */
        rowClass?: string;
    }

    let { help, helpTestId, helpLabel, rowClass = '', fullWidth = false, ...button }: IProps = $props();
</script>

<span class="flex items-center gap-1 {rowClass}">
    <Button {...button} {fullWidth} class="{fullWidth ? 'min-w-0 flex-1' : ''} {button.class ?? ''}" />
    <HelpTooltip tooltip={help} ariaLabel={helpLabel} testId={helpTestId} />
</span>

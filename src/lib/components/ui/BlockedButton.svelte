<script lang="ts">
    import type { Snippet } from 'svelte';
    import Button from './Button.svelte';
    import ButtonWithHelp from './ButtonWithHelp.svelte';
    import NoteBox from './NoteBox.svelte';
    import type { IButtonProps } from './types';

    /**
     * A button that stays in its place, greyed out, with the reason under it while it cannot be pressed. «Взять» is one: «Сначала завершите шаг
     * № … — у него выше приоритет». With `help` it carries its «?» beside it (`ButtonWithHelp`); without, it is a plain button.
     * The reason is a `NoteBox` (warning, a polite status) — `reason` as text, or `reasonContent` for several paragraphs.
     */
    interface IProps extends IButtonProps {
        /** Why the button cannot be pressed now, already translated; null or empty: it can. */
        reason?: string | null;
        /** The reason as markup (several paragraphs); wins over `reason` when given. Its presence blocks the button. */
        reasonContent?: Snippet;
        /** `data-test` of the reason's box. */
        reasonTestId?: string;
        /** `data-tour` of the reason's box. */
        reasonTour?: string;
        /** Classes of the reason's box (its layout). */
        reasonClass?: string;
        /** What the «?» says; left out, the button has no «?». */
        help?: string;
        helpTestId?: string;
        helpLabel?: string;
        /** Classes of the whole block (its place in a grid or a row). */
        blockClass?: string;
    }

    let {
        reason = null,
        reasonContent,
        reasonTestId,
        reasonTour,
        reasonClass = '',
        help,
        helpTestId = '',
        helpLabel = '',
        blockClass = '',
        disabled = false,
        ...button
    }: IProps = $props();

    const blocked = $derived(reasonContent !== undefined || (reason !== null && reason !== ''));
</script>

<div class="flex flex-col gap-2 {blockClass}">
    {#if help !== undefined}
        <ButtonWithHelp {...button} disabled={disabled || blocked} {help} {helpTestId} {helpLabel} />
    {:else}
        <Button {...button} disabled={disabled || blocked} />
    {/if}
    {#if blocked}
        <NoteBox tone="warning" role="status" dataTest={reasonTestId} dataTour={reasonTour} class={reasonClass}>
            {#if reasonContent}{@render reasonContent()}{:else}<p>{reason}</p>{/if}
        </NoteBox>
    {/if}
</div>

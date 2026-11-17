<script lang="ts">
    import type { Snippet } from 'svelte';
    import { getLocale, t } from '$lib/i18n/i18n';
    import Button from './Button.svelte';
    import ModalWindow from './ModalWindow.svelte';
    import WindowButtons from './WindowButtons.svelte';

    /**
     * The question before a press that cannot be taken back: a title, a sentence, a red button that does it and a grey one
     * that keeps things as they are. Built on `ModalWindow`, which owns the backdrop, the focus trap, the
     * inert page, the scroll lock and Escape; this component adds only the question and its two answers.
     */
    let {
        open = $bindable(false),
        title = '',
        message = '',
        confirmText = t('app.common.confirm', getLocale()),
        // In the page's language: a dialog that names no label of its buttons shows the interface's own words.
        cancelText = t('app.common.cancel', getLocale()),
        onConfirm = () => {},
        onCancel = () => {},
        confirmType = 'button',
        cancelType = 'button',
        confirmForm = '',
        cancelForm = '',
        confirmDisabled = false,
        dataTest = '',
        closeOnEscape = true,
        sending = undefined,
        children
    }: {
        open?: boolean;
        title?: string;
        message?: string;
        confirmText?: string;
        cancelText?: string;
        onConfirm?: () => void | false;
        onCancel?: () => void | false;
        confirmType?: 'button' | 'submit';
        cancelType?: 'button' | 'submit';
        confirmForm?: string;
        cancelForm?: string;
        confirmDisabled?: boolean;
        dataTest?: string;
        closeOnEscape?: boolean;
        /**
         * The `submitting` of the superform that `confirmForm` names. A form that no longer reloads the page
         * leaves nothing to close the dialog, so while it is sending the confirm button shows the press state, and when the
         * answer arrives (`sending` goes from true back to false) the dialog closes; the answer itself is drawn under the
         * button that opened the dialog. Left out, the dialog behaves as before.
         */
        sending?: boolean;
        children?: Snippet;
    } = $props();

    let wasSending = false;
    $effect(() => {
        const now = sending === true;
        if (wasSending && !now && open) open = false;
        wasSending = now;
    });

    function cancel(): void {
        if (onCancel() !== false) open = false;
    }

    function confirm(): void {
        if (onConfirm() !== false) open = false;
    }
</script>

<ModalWindow bind:open {title} dataTest={dataTest || undefined} {closeOnEscape} onClose={() => void onCancel()}>
    <p class="text-sm text-ink-600">{message}</p>
    {#if children}
        <div class="mt-6">
            {@render children()}
        </div>
    {/if}
    {#snippet footer()}
        <!--
            `Button`s: pressed feedback, and a confirm that submits a guarded form through `form="…"` shows it is working —
            and cannot be pressed again — until the form's answer is in (GuardedForm).
        -->
        <WindowButtons>
            {#if cancelType === 'submit' && cancelForm}
                <Button
                    type="submit"
                    form={cancelForm}
                    variant="secondary"
                    dataTest={dataTest ? `${dataTest}-cancel` : undefined}
                >
                    {cancelText}
                </Button>
            {:else}
                <Button variant="secondary" onclick={cancel} dataTest={dataTest ? `${dataTest}-cancel` : undefined}>
                    {cancelText}
                </Button>
            {/if}

            {#if confirmType === 'submit' && confirmForm}
                <Button
                    type="submit"
                    form={confirmForm}
                    variant="danger"
                    disabled={confirmDisabled}
                    pending={sending === true}
                    dataTest={dataTest ? `${dataTest}-confirm` : undefined}
                >
                    {confirmText}
                </Button>
            {:else}
                <Button
                    variant="danger"
                    onclick={confirm}
                    disabled={confirmDisabled}
                    dataTest={dataTest ? `${dataTest}-confirm` : undefined}
                >
                    {confirmText}
                </Button>
            {/if}
        </WindowButtons>
    {/snippet}
</ModalWindow>

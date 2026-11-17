<script lang="ts">
    interface IProps {
        /** «Сохранено», already worded; null when the form has not answered with a success. */
        saved?: string | null;
        /** The refusal, already worded; null when there is none. Drawn instead of `saved` when both are given. */
        error?: string | null;
        /** The `data-test` prefix: the lines are `<prefix>-saved` and `<prefix>-error`, e.g. `settings-saved`. */
        dataTest: string;
        /** A tour's anchor on the refusal line, e.g. `settings-error`. */
        errorTour?: string;
        /**
         * The line stands as a block of its page's column (a page's one answer line for its row buttons), not under a button:
         * the column's gap spaces it, so it keeps no space of its own above it. Under a button it stands 8 px below.
         */
        standalone?: boolean;
    }

    let { saved = null, error = null, dataTest, errorTour, standalone = false }: IProps = $props();
    const spacing = $derived(standalone ? '' : 'mt-2 ');
</script>

<!--
    The answer of one form, drawn under its own save button. A form that answers on its own page no
    longer reloads it, so the page stays where the person is looking and a banner at its top would be out of sight.
    «Сохранено» is a polite status; a refusal is an alert, so a screen reader reads it at once. The form clears its
    answer the moment it is sent again (the form's message, `src/lib/forms/super-form.ts`).
-->
{#if error}
    <p
        role="alert"
        class="{spacing}text-xs break-words text-stop-800"
        data-test="{dataTest}-error"
        data-tour={errorTour}
    >
        {error}
    </p>
{:else if saved}
    <p role="status" class="{spacing}text-xs text-ok-800" data-test="{dataTest}-saved">{saved}</p>
{/if}

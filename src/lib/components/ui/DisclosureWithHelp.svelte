<script lang="ts">
    import type { Snippet } from 'svelte';
    import type { TLocale } from '$lib/i18n/types';
    import DisclosureForm from './DisclosureForm.svelte';
    import HelpTooltip from './HelpTooltip.svelte';

    /**
     * A decision that opens a form, with its «?» beside the opening button: «Подтвердить», «Отклонить», «Выключить
     * подключение». Closed, it is the opening button and the «?» that says what pressing it does; open, it
     * is the form (`DisclosureForm`) with whatever the page puts in it — the question and the form that sends the answer.
     */
    interface IProps {
        /** The opening button's words, already translated; also the «?»'s accessible name. */
        label: string;
        lang: TLocale;
        /** What the «?» says, already translated. */
        help: string;
        /** `data-test` of the «?» (`<helpTestId>-button` is its button). */
        helpTestId: string;
        /** `data-test` of the opening button. */
        openerTestId: string;
        /** `data-test` of the open form's box. */
        testId: string;
        /** A tour's anchor on the button and its «?» together. */
        tour?: string;
        children: Snippet;
    }

    let { label, lang, help, helpTestId, openerTestId, testId, tour, children }: IProps = $props();
</script>

<span class="inline-flex items-center gap-1" data-tour={tour}>
    <DisclosureForm {label} {lang} {openerTestId} {testId}>
        {@render children()}
    </DisclosureForm>
    <HelpTooltip tooltip={help} ariaLabel={label} testId={helpTestId} />
</span>

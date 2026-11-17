<script lang="ts">
    import type { TFormAnswer } from '$lib/forms/answer';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import Button from './Button.svelte';
    import FormAnswer from './FormAnswer.svelte';
    import FormButtons from './FormButtons.svelte';
    import type { IButtonProps } from './types';

    /**
     * A form's submit button and, under it, the refusal the form answered with: the row every step of a
     * multi-step form ends with. The refusal is worded here from the answer's message key and its parameters; a success is not drawn
     * (the step moves on instead). Used inside a `GuardedForm`.
     */
    interface IProps {
        lang: TLocale;
        /** The button's words, already translated. */
        label: string;
        /** `data-test` of the button. */
        dataTest: string;
        /** The prefix of the refusal's `data-test` (`<prefix>-error`); the button's own by default. */
        answerTest?: string;
        /** The form's answer (`answerOf($message)`). */
        answer: TFormAnswer | undefined;
        variant?: IButtonProps['variant'];
    }

    let { lang, label, dataTest, answerTest, answer, variant }: IProps = $props();
</script>

<FormButtons>
    <Button type="submit" {variant} {dataTest}>{label}</Button>
    {#snippet below()}
        <FormAnswer
            error={answer?.error ? t(answer.error, lang, answer.params) : null}
            dataTest={answerTest ?? dataTest}
        />
    {/snippet}
</FormButtons>

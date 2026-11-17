<script lang="ts">
    import { z } from 'zod';
    import { tError } from '$lib/i18n/form-error';
    import { answerOf } from '$lib/forms/answer';
    import { spaForm, type TSuperFormOptions } from '$lib/forms/super-form';

    /**
     * One form in the single-page mode, for the tests of `src/lib/forms/super-form.ts`: a text box `name` (two letters at
     * least, the schema's message is a key of `messages/`), a send button, and the form's answer. `options` goes to `spaForm`
     * as it is (`send`, `sample`, `answerGroup`...); `onMade` hands the made form to the test.
     */
    interface IProps {
        id?: string;
        options?: TSuperFormOptions<{ name: string }>;
        onMade?: (form: ReturnType<typeof spaForm<typeof schema>>) => void;
    }

    const schema = z.object({ name: z.string().min(2, 'errors.invalid') });

    let { id = 'spa-form', options = {}, onMade }: IProps = $props();

    // svelte-ignore state_referenced_locally
    const made = spaForm(schema, { name: '' }, { id, resetForm: false, ...options });
    // svelte-ignore state_referenced_locally
    onMade?.(made);
    const { form, errors, message, enhance, submitting } = made;
</script>

<form method="POST" use:enhance data-test={id}>
    <input name="name" data-test="{id}-name" bind:value={$form.name} />
    <span data-test="{id}-name-error">{tError($errors.name, 'ru')}</span>
    <button data-test="{id}-send" disabled={$submitting}>send</button>
    <output data-test="{id}-answer">{answerOf($message)?.saved ?? answerOf($message)?.error ?? ''}</output>
    <span data-test="{id}-form-error">{tError($errors._errors, 'ru')}</span>
</form>

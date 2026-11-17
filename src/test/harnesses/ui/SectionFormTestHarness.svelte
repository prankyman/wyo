<script lang="ts">
    import Field from '$lib/components/ui/Field.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import type { ISectionSuperForm } from '$lib/forms/section-form';

    /**
     * A section of an edit screen around one box (`SectionForm.svelte.test.ts`). The superform is a stand-in the test drives:
     * typing into the box taints it, the test sets the answer through `message`.
     */
    let {
        form,
        create = false,
        canWrite = true,
        class: className,
        innError = null
    }: {
        form: ISectionSuperForm & { typed(value: string): void };
        create?: boolean;
        canWrite?: boolean;
        /** The classes of the section's outer box, as a screen gives them. */
        class?: string;
        /** A refusal of the server drawn under the second box, as `Field` draws one. */
        innError?: string | null;
    } = $props();
</script>

<SectionForm
    lang="ru"
    id="client-section-legal"
    title="Юридические данные"
    {form}
    {create}
    {canWrite}
    class={className}
>
    <input data-test="legal-name" name="legalName" oninput={(event) => form.typed(event.currentTarget.value)} />
    <Field label="ИНН" for="inn" testId="inn-field" error={innError}>
        <input id="inn" name="inn" data-test="inn" />
    </Field>
</SectionForm>

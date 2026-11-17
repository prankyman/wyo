<script lang="ts">
    import type { TLocale } from '$lib/i18n/types';
    import { superForm } from '$lib/forms/super-form';
    import { type SuperForm } from 'sveltekit-superforms';
    import Textarea from '$lib/components/ui/Textarea.svelte';

    interface IProps {
        initialData: Record<string, unknown>;
        name: string;
        lang: TLocale;
        rows?: number;
        maxlength?: number;
        placeholder?: string;
        disabled?: boolean;
        class?: string;
        testId?: string;
        initialErrors?: Record<string, string[]>;
    }

    let {
        initialData,
        name,
        lang,
        rows,
        maxlength,
        placeholder,
        disabled,
        class: className,
        testId,
        initialErrors
    }: IProps = $props();

    // svelte-ignore state_referenced_locally
    const testForm = superForm(initialData, { validationMethod: 'oninput' });
    const { errors } = testForm;
    // svelte-ignore state_referenced_locally
    if (initialErrors) {
        errors.set(initialErrors);
    }
</script>

<Textarea
    form={testForm as unknown as SuperForm<Record<string, unknown>>}
    {name}
    {lang}
    {rows}
    {maxlength}
    {placeholder}
    {disabled}
    class={className}
    {testId}
/>

<script lang="ts">
    import type { HTMLTextareaAttributes } from 'svelte/elements';
    import { writable } from 'svelte/store';
    import type { TLocale } from '$lib/i18n/types';
    import { t } from '$lib/i18n/i18n';
    import { cn } from '$lib/utils/cn';
    import type { SuperForm } from 'sveltekit-superforms';
    import { INVALID_CLASS, TEXTAREA_CLASS } from './field-classes';

    /**
     * Every text area of the interface, in one of two modes:
     *
     * - with `form` — a field of a superform: it binds `$form[name]`, draws the field's refusal and the length counter
     *   under the box, and checks itself when the cursor leaves it;
     * - without `form` — a plain box: `value` is bound by the caller, `invalid` gives the red border, and `Field` around
     *   it draws the label and the refusal, as for `TextBox`.
     *
     * The look is `TEXTAREA_CLASS`, decided once; the caller passes only what differs through `class`. Every native
     * attribute (`placeholder`, `required`, `readonly`, `aria-*`, `data-*`, the events) reaches the `<textarea>`.
     */
    interface IProps extends Omit<HTMLTextareaAttributes, 'value' | 'class' | 'form'> {
        /** The superform the field belongs to; without it the box is plain. */
        form?: SuperForm<Record<string, unknown>>;
        name?: string;
        /** The language of the refusal (the superform mode). */
        lang?: TLocale;
        /** The text in the box (the plain mode); bind it to read what was typed. */
        value?: string | null;
        rows?: number;
        maxlength?: number;
        /** The box holds a refused value: a red border and `aria-invalid` (the plain mode). */
        invalid?: boolean;
        class?: string;
        /** The box's `data-test`; in the superform mode also the prefix of its refusal and counter (the field's name by default). */
        testId?: string;
        dataTest?: string;
        /** The box element, for a screen that moves the cursor into it. */
        element?: HTMLTextAreaElement | null;
        /** The `id` of the form the box belongs to when it is drawn outside it (a dialog's reason box) — the plain mode. */
        formId?: string;
        onfocusout?: (event: FocusEvent) => void;
    }

    let {
        form: superFormInstance,
        name = '',
        lang,
        value = $bindable(),
        rows = 3,
        maxlength,
        invalid = false,
        class: className = '',
        testId,
        dataTest,
        element = $bindable(null),
        formId,
        onfocusout,
        ...rest
    }: IProps = $props();

    // The plain mode has no superform; an empty store stands in for its data, so the markup below reads one shape.
    // svelte-ignore state_referenced_locally
    const form = superFormInstance?.form ?? writable<Record<string, unknown>>({});
    // svelte-ignore state_referenced_locally
    const errors = superFormInstance?.errors ?? writable<Record<string, unknown>>({});
    // svelte-ignore state_referenced_locally
    const bound = superFormInstance !== undefined;

    /**
     * The field checks itself when focus leaves it (constitution, "Forms"). Guarded on `validators`, because `validate()`
     * throws on a form that has no schema.
     */
    function handleFocusOut(event: FocusEvent): void {
        if (superFormInstance?.options?.validators) {
            void superFormInstance.validate(name, { update: 'errors' });
        }
        onfocusout?.(event);
    }

    const resolvedTestId = $derived(testId ?? dataTest ?? (name || undefined));
    const fieldError = $derived(($errors as Record<string, unknown>)[name] as string | string[] | undefined);
    const currentLength = $derived(($form[name] as string | undefined)?.length ?? 0);
    const lookClass = $derived(cn(TEXTAREA_CLASS, 'max-h-250 resize-y', invalid && INVALID_CLASS, className));
</script>

{#if bound}
    <div class="flex flex-col gap-1">
        <textarea
            bind:this={element}
            {name}
            bind:value={$form[name]}
            {rows}
            onfocusout={handleFocusOut}
            class={lookClass}
            data-test={resolvedTestId}
            {...rest}></textarea>
        {#if maxlength || fieldError}
            <div class="flex items-center justify-between">
                {#if fieldError}
                    <span data-test="{resolvedTestId}-error" class="text-xs text-stop-700"
                        >{t(Array.isArray(fieldError) ? fieldError[0] : fieldError, lang)}</span
                    >
                {:else}
                    <span></span>
                {/if}
                {#if maxlength}
                    <span data-test="{resolvedTestId}-counter" class="text-xs text-ink-400"
                        ><span
                            data-test="{resolvedTestId}-counter-count"
                            class:text-stop-600={currentLength > maxlength}>{currentLength}</span
                        >/{maxlength}</span
                    >
                {/if}
            </div>
        {/if}
    </div>
{:else}
    <textarea
        bind:this={element}
        bind:value
        name={name || undefined}
        form={formId}
        {rows}
        {maxlength}
        {onfocusout}
        class={lookClass}
        aria-invalid={invalid || undefined}
        data-test={resolvedTestId}
        {...rest}></textarea>
{/if}

<script lang="ts" generics="TValue extends string | number = string">
    import type { Snippet } from 'svelte';
    import type { HTMLSelectAttributes } from 'svelte/elements';
    import { cn } from '$lib/utils/cn';
    import { INVALID_CLASS, SELECT_CLASS, SMALL_CLASS } from './field-classes';
    import type { ISelectOption } from './types';

    /**
     * Every drop-down of the interface. The look is `SELECT_CLASS`, decided
     * once; the caller passes only what differs through `class`. The choices are either `options` or the caller's own
     * `<option>` tags (`children`), with `blank` — an empty first choice with its words — before them. Every native
     * attribute (`name`, `required`, `aria-*`, `data-*` — `data-choices` of `select-choices-policy.test.ts`
     * among them —, the events) reaches the `<select>` as it is.
     *
     * It draws only the box; `Field` around it draws the label, the «?» and the refusal.
     */
    interface IProps extends Omit<HTMLSelectAttributes, 'value' | 'size' | 'class' | 'children' | 'multiple'> {
        /** The chosen value; `''` while the blank choice is shown. Bind it to read the choice. */
        value?: TValue | '' | null | undefined;
        options?: readonly ISelectOption<TValue>[];
        /** The words of an empty first choice (value `''`); without it there is none. */
        blank?: string;
        /** The empty first choice cannot be picked again once something is chosen. */
        blankDisabled?: boolean;
        /** `sm` — the denser box of a table's filter row. */
        size?: 'md' | 'sm';
        /** The box holds a refused value: a red border and `aria-invalid`. */
        invalid?: boolean;
        /** Classes merged over the look: the box's width, its letters. */
        class?: string;
        dataTest?: string;
        /** The select element, for a screen that moves the cursor into it. */
        element?: HTMLSelectElement | null;
        children?: Snippet;
    }

    let {
        value = $bindable(),
        options,
        blank,
        blankDisabled = false,
        size = 'md',
        invalid = false,
        class: className = '',
        dataTest,
        element = $bindable(null),
        children,
        ...rest
    }: IProps = $props();
</script>

<select
    bind:this={element}
    bind:value
    class={cn(SELECT_CLASS, size === 'sm' && SMALL_CLASS, invalid && INVALID_CLASS, className)}
    aria-invalid={invalid || undefined}
    data-test={dataTest}
    {...rest}
>
    {#if blank !== undefined}
        <option value="" disabled={blankDisabled}>{blank}</option>
    {/if}
    {#if options}
        {#each options as option, index (index)}
            <option value={option.value} disabled={option.disabled}>{option.label}</option>
        {/each}
    {/if}
    {@render children?.()}
</select>

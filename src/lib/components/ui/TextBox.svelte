<script lang="ts">
    import type { HTMLInputAttributes } from 'svelte/elements';
    import { cn } from '$lib/utils/cn';
    import { INPUT_CLASS, INVALID_CLASS, SMALL_CLASS } from './field-classes';

    /**
     * Every one-line text box of the interface: text, e-mail, phone, address,
     * search, password, date and time. The look is `INPUT_CLASS`, decided once; the caller passes only what differs through
     * `class`. Every native attribute (`name`, `placeholder`, `inputmode`, `autocomplete`, `maxlength`, `required`,
     * `readonly`, `aria-*`, `data-*`, the events) reaches the `<input>` as it is. A number is `NumberField`, a tick box,
     * a radio, a file and a hidden value stay raw tags: they have no shared box to decide.
     *
     * It draws only the box; `Field` around it draws the label, the «?» and the refusal.
     */
    interface IProps extends Omit<HTMLInputAttributes, 'type' | 'size' | 'value' | 'class'> {
        type?: 'text' | 'email' | 'tel' | 'url' | 'search' | 'password' | 'date' | 'time' | 'datetime-local' | 'month';
        /** The text in the box; bind it to read what was typed. */
        value?: string | null;
        /** `sm` — the denser box of a table's filter row. */
        size?: 'md' | 'sm';
        /** The box holds a refused value: a red border and `aria-invalid`. */
        invalid?: boolean;
        /** Classes merged over the look: the box's width, its letters. */
        class?: string;
        dataTest?: string;
        /** The box element, for a screen that moves the cursor into it. */
        element?: HTMLInputElement | null;
    }

    let {
        type = 'text',
        value = $bindable(),
        size = 'md',
        invalid = false,
        class: className = '',
        dataTest,
        element = $bindable(null),
        ...rest
    }: IProps = $props();
</script>

<input
    bind:this={element}
    bind:value
    {type}
    class={cn(INPUT_CLASS, size === 'sm' && SMALL_CLASS, invalid && INVALID_CLASS, className)}
    aria-invalid={invalid || undefined}
    data-test={dataTest}
    {...rest}
/>

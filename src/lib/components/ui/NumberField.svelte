<script lang="ts">
    import type { HTMLInputAttributes } from 'svelte/elements';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { cn } from '$lib/utils/cn';
    import { inputModeOf } from '$lib/validation/number-input';
    import Button from './Button.svelte';
    import IconTooltip from './IconTooltip.svelte';
    import { acceptsTyped, flipSign, normalizePasted } from './number-field';

    /**
     * Every number a person types. A text box, never
     * `type="number"`: a number box empties text it cannot read («58,5» on a Russian keyboard reaches the server blank),
     * changes its value under the mouse wheel and the arrow keys, and says nothing about a wrong entry
     * (https://html.spec.whatwg.org/multipage/input.html#number-state-(type=number)). The phone shows its digit keyboard
     * by the field's places; a field that accepts a negative gets «±» beside its box, because those keyboards may have no
     * minus. What it lets in, and why, is in `number-field.ts`; the value is read by `readNumberText` on both sides.
     *
     * It draws only the box (and «±»); `Field` around it draws the label, the «?» and the refusal.
     */
    interface IProps extends Omit<HTMLInputAttributes, 'type' | 'inputmode' | 'value' | 'min' | 'max' | 'step'> {
        /** The text in the box, as typed; a form whose box starts empty may hold `undefined` until something is typed. */
        value?: string | undefined;
        /** Places after the separator the field keeps: 0 is a whole number. */
        decimals?: number;
        /** The field accepts a negative value: «±» is drawn beside the box. */
        signed?: boolean;
        /** The language of «±»'s name and tooltip. */
        lang: TLocale;
        /** The box's `id`, the one `Field`'s `for` names. */
        id?: string;
        name?: string;
        dataTest?: string;
        disabled?: boolean;
        /** Classes of the box itself (its width, its text size); the look is this component's. */
        class?: string;
        /** The box element, for a screen that moves the cursor into it. */
        element?: HTMLInputElement | null;
    }

    let {
        value = $bindable(),
        decimals = 0,
        signed = false,
        lang,
        id,
        name,
        dataTest,
        disabled = false,
        class: className = '',
        element = $bindable(null),
        ...rest
    }: IProps = $props();

    const rule = $derived({ decimals, signed });

    /** Typing lets in only what can be part of a number; a paste is tidied and kept, so the refusal can name a mistake. */
    function onBeforeInput(event: InputEvent): void {
        const box = event.currentTarget as HTMLInputElement;
        const start = box.selectionStart ?? box.value.length;
        const end = box.selectionEnd ?? start;
        if (event.inputType === 'insertFromPaste' || event.inputType === 'insertFromDrop') {
            const pasted = event.data ?? event.dataTransfer?.getData('text/plain') ?? '';
            event.preventDefault();
            insert(box, normalizePasted(pasted, rule), start, end);
            return;
        }
        if (!event.inputType.startsWith('insert') || event.data === null) return;
        if (!acceptsTyped(box.value, event.data, start, end, rule)) event.preventDefault();
    }

    /** Puts `text` in place of the selection, as the browser would have, and tells the form the value changed. */
    function insert(box: HTMLInputElement, text: string, start: number, end: number): void {
        box.value = box.value.slice(0, start) + text + box.value.slice(end);
        const caret = start + text.length;
        box.setSelectionRange(caret, caret);
        box.dispatchEvent(new Event('input', { bubbles: true }));
    }

    /** «±»: the minus goes on or comes off, and the cursor stays where it was in the digits. */
    function toggleSign(): void {
        const box = element;
        if (!box) return;
        const caret = box.selectionStart ?? box.value.length;
        const before = box.value.length;
        const flipped = flipSign(box.value);
        box.value = flipped;
        box.dispatchEvent(new Event('input', { bubbles: true }));
        box.focus();
        const moved = Math.max(0, Math.min(flipped.length, caret + (flipped.length - before)));
        box.setSelectionRange(moved, moved);
    }

    const signName = $derived(t('common.numberField.sign', lang));
</script>

<div class="flex min-w-0 items-stretch gap-1">
    <input
        bind:this={element}
        bind:value
        type="text"
        inputmode={inputModeOf(decimals)}
        autocomplete="off"
        {id}
        {name}
        {disabled}
        data-test={dataTest}
        data-number-field={signed ? 'signed' : 'unsigned'}
        class={cn(
            'min-h-11 w-full min-w-0 rounded border border-ink-300 bg-white px-3 text-sm disabled:opacity-50',
            className
        )}
        onbeforeinput={onBeforeInput}
        {...rest}
    />
    {#if signed}
        <IconTooltip name={signName} side="above" testId={dataTest ? `${dataTest}-sign-tip` : undefined}>
            <!-- The press must not take the cursor out of the box: the minus is put where the person was typing. -->
            <Button
                variant="secondary"
                class="min-w-11 px-0 text-base normal-case"
                ariaLabel={signName}
                {disabled}
                dataTest={dataTest ? `${dataTest}-sign` : undefined}
                onpointerdown={(event: PointerEvent) => event.preventDefault()}
                onclick={toggleSign}
            >
                ±
            </Button>
        </IconTooltip>
    {/if}
</div>

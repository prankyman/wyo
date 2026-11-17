import NumberFieldTestHarness from '$test/harnesses/ui/NumberFieldTestHarness.svelte';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { acceptsTyped, flipSign, normalizePasted } from './number-field';

/**
 * The one number field. The policy test (`number-field-policy.test.ts`) makes
 * every number box of the interface this component, so no screen tests its keyboard, its filter or its «±» again.
 */

/** Types one character the way a browser does: `beforeinput` first, and the character only if nobody refused it. */
async function type(box: HTMLInputElement, character: string): Promise<void> {
    const event = new InputEvent('beforeinput', {
        inputType: 'insertText',
        data: character,
        cancelable: true,
        bubbles: true
    });
    box.dispatchEvent(event);
    if (event.defaultPrevented) return;
    const start = box.selectionStart ?? box.value.length;
    box.value = box.value.slice(0, start) + character + box.value.slice(box.selectionEnd ?? start);
    box.setSelectionRange(start + 1, start + 1);
    await fireEvent.input(box);
}

describe('NumberField', () => {
    it('is a text box with the digit keyboard of its places, never a number box with arrows', () => {
        render(NumberFieldTestHarness, { props: { decimals: 0 } });
        const box = screen.getByTestId('shift-input');
        expect(box).toHaveAttribute('type', 'text');
        expect(box).toHaveAttribute('inputmode', 'numeric');
        expect(box).not.toHaveAttribute('step');
        expect(screen.getByLabelText('Shift')).toBe(box);
        expect(screen.queryByTestId('shift-input-sign')).toBeNull();
    });

    it('takes digits, one separator and a leading minus on a signed field, and nothing else', async () => {
        render(NumberFieldTestHarness, { props: { decimals: 1, signed: true } });
        const box = screen.getByTestId('shift-input') as HTMLInputElement;
        expect(box).toHaveAttribute('inputmode', 'decimal');
        for (const character of ['-', '2', 'e', ',', '.', '5', '-', '+']) await type(box, character);
        expect(box.value).toBe('-2,5');
        expect(screen.getByTestId('shift-value')).toHaveTextContent('-2,5');
    });

    it('tidies a paste and keeps what it cannot read, so the refusal under the box names it', async () => {
        render(NumberFieldTestHarness, { props: { decimals: 1, signed: true, min: -5, max: 5 } });
        const box = screen.getByTestId('shift-input') as HTMLInputElement;
        const paste = new InputEvent('beforeinput', {
            inputType: 'insertFromPaste',
            data: '\u2212 1,25',
            cancelable: true,
            bubbles: true
        });
        box.dispatchEvent(paste);
        expect(paste.defaultPrevented).toBe(true);
        expect(box.value).toBe('-1,25');
        await fireEvent.blur(box);
        expect(screen.getByTestId('shift-error')).toHaveTextContent('tooManyDecimals');
        expect(screen.getByTestId('shift-field')).toHaveAttribute('data-field-error');
    });

    it('«±» puts the minus on and takes it off, and leaves the cursor in the box where it was', async () => {
        render(NumberFieldTestHarness, { props: { decimals: 1, signed: true, value: '2,5' } });
        const box = screen.getByTestId('shift-input') as HTMLInputElement;
        const sign = screen.getByTestId('shift-input-sign');
        expect(sign).toHaveAttribute('aria-label', 'Сменить знак: плюс или минус');
        box.focus();
        box.setSelectionRange(1, 1);

        const press = new PointerEvent('pointerdown', { cancelable: true, bubbles: true });
        sign.dispatchEvent(press);
        expect(press.defaultPrevented).toBe(true);
        await fireEvent.click(sign);
        expect(box.value).toBe('-2,5');
        expect(screen.getByTestId('shift-value')).toHaveTextContent('-2,5');
        expect(document.activeElement).toBe(box);
        expect(box.selectionStart).toBe(2);

        await fireEvent.click(sign);
        expect(box.value).toBe('2,5');
        expect(box.selectionStart).toBe(1);
    });
});

describe('number-field typing rules', () => {
    const whole = { decimals: 0, signed: false };
    const signed = { decimals: 1, signed: true };

    it.each([
        ['', '5', 0, 0, whole, true],
        ['', ',', 0, 0, whole, false],
        ['', '-', 0, 0, whole, false],
        ['12', '-', 0, 0, signed, true],
        ['12', '-', 1, 1, signed, false],
        ['-12', '-', 0, 0, signed, false],
        ['-12', '3', 0, 0, signed, false],
        ['1,2', ',', 3, 3, signed, false],
        ['1,2', ',', 1, 2, signed, true],
        ['12', '34', 2, 2, whole, true],
        ['12', 'a', 2, 2, whole, false]
    ])(
        'typing into «%s» the text «%s» at %i to %i (%o) is let in: %s',
        (current, typed, start, end, rule, accepted) => {
            expect(acceptsTyped(current, typed, start, end, rule)).toBe(accepted);
        }
    );

    it('tidies a paste by the field’s sign and flips a sign', () => {
        expect(normalizePasted('+ 1\u00a0234', whole)).toBe('1234');
        expect(normalizePasted('\u20133', signed)).toBe('-3');
        expect(normalizePasted('\u22123', whole)).toBe('\u22123');
        expect(flipSign('3')).toBe('-3');
        expect(flipSign('-3')).toBe('3');
        expect(flipSign('')).toBe('-');
    });
});

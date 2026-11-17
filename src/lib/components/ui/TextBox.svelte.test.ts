import FormControlTestHarness from '$test/harnesses/ui/FormControlTestHarness.svelte';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import TextBox from './TextBox.svelte';
import { INPUT_CLASS } from './field-classes';

/**
 * The one text box. `form-control-policy.test.ts` makes every one-line box of the interface this component, so
 * no screen tests its look, its binding or its refused state again.
 */
describe('TextBox', () => {
    it('draws the interface’s box look and passes every native attribute through', () => {
        render(TextBox, {
            props: {
                type: 'email',
                name: 'email',
                placeholder: 'Почта',
                autocomplete: 'email',
                inputmode: 'email',
                maxlength: 120,
                required: true,
                'aria-describedby': 'hint',
                'data-tour': 'email-box',
                dataTest: 'email'
            }
        });
        const box = screen.getByTestId('email');

        expect(box).toHaveAttribute('type', 'email');
        expect(box).toHaveAttribute('name', 'email');
        expect(box).toHaveAttribute('placeholder', 'Почта');
        expect(box).toHaveAttribute('autocomplete', 'email');
        expect(box).toHaveAttribute('inputmode', 'email');
        expect(box).toHaveAttribute('maxlength', '120');
        expect(box).toBeRequired();
        expect(box).toHaveAttribute('aria-describedby', 'hint');
        expect(box).toHaveAttribute('data-tour', 'email-box');
        expect(box).not.toHaveAttribute('aria-invalid');
        expect(box.className).toBe(INPUT_CLASS);
    });

    it('is a text box by default and lets a raw data-test name it', () => {
        render(TextBox, { props: { 'data-test': 'raw-name' } as Record<string, string> });

        expect(screen.getByTestId('raw-name')).toHaveAttribute('type', 'text');
    });

    it('binds what is typed and hands its element to the caller', async () => {
        const onFocus = vi.fn();
        render(FormControlTestHarness, { props: { control: 'text', initial: 'Москва', onFocus } });
        const box = screen.getByTestId('box');

        expect(box).toHaveValue('Москва');
        await fireEvent.input(box, { target: { value: 'Казань' } });
        expect(screen.getByTestId('bound')).toHaveTextContent('Казань');
        await fireEvent.click(screen.getByTestId('focus'));
        expect(onFocus).toHaveBeenCalledWith(box);
    });

    it('marks a refused value, draws the small box and merges the caller’s classes over the look', () => {
        render(FormControlTestHarness, {
            props: { control: 'text', invalid: true, size: 'sm', class: 'w-auto text-base' }
        });
        const box = screen.getByTestId('box');

        expect(box).toHaveAttribute('aria-invalid', 'true');
        expect(box).toHaveClass('border-stop-600', 'px-2', 'w-auto', 'text-base');
        expect(box).not.toHaveClass('w-full', 'px-3', 'text-sm', 'text-xs', 'border-ink-300');
    });
});

import FormControlTestHarness from '$test/harnesses/ui/FormControlTestHarness.svelte';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { SELECT_CLASS } from './field-classes';

/**
 * The one drop-down. `form-control-policy.test.ts` makes every `<select>` of the interface this component, so
 * no screen tests its look, its blank choice or its binding again.
 */
const OPTIONS = [
    { value: 'fbs', label: 'FBS' },
    { value: 'fbo', label: 'FBO', disabled: true }
];

describe('SelectBox', () => {
    it('draws the blank choice first, then the given options, and passes the native attributes through', () => {
        render(FormControlTestHarness, { props: { control: 'select', options: OPTIONS, blank: '— выберите —' } });
        const box = screen.getByTestId('box') as HTMLSelectElement;
        const choices = within(box).getAllByRole('option') as HTMLOptionElement[];

        expect(choices.map((choice) => [choice.value, choice.textContent, choice.disabled])).toEqual([
            ['', '— выберите —', false],
            ['fbs', 'FBS', false],
            ['fbo', 'FBO', true]
        ]);
        expect(box).toHaveAttribute('name', 'kind');
        expect(box).toHaveAttribute('data-choices', 'fixed');
        expect(box).not.toHaveAttribute('aria-invalid');
        expect(box.className).toBe(SELECT_CLASS);
    });

    it('takes the caller’s own option tags and binds the choice', async () => {
        render(FormControlTestHarness, { props: { control: 'select', initial: 'own', withChildren: true } });
        const box = screen.getByTestId('box') as HTMLSelectElement;

        expect(within(box).getAllByRole('option')).toHaveLength(1);
        expect(box).toHaveValue('own');
        await fireEvent.change(box, { target: { value: 'own' } });
        expect(screen.getByTestId('bound')).toHaveTextContent('own');
    });

    it('reports a new choice through the binding', async () => {
        render(FormControlTestHarness, { props: { control: 'select', initial: '', options: OPTIONS, blank: '—' } });
        const box = screen.getByTestId('box') as HTMLSelectElement;

        box.value = 'fbs';
        await fireEvent.change(box);
        expect(screen.getByTestId('bound')).toHaveTextContent('fbs');
    });

    it('marks a refused choice, draws the small box and merges the caller’s classes over the look', () => {
        render(FormControlTestHarness, {
            props: { control: 'select', options: OPTIONS, invalid: true, size: 'sm', class: 'sm:w-64' }
        });
        const box = screen.getByTestId('box');

        expect(box).toHaveAttribute('aria-invalid', 'true');
        expect(box).toHaveClass('border-stop-600', 'px-2', 'text-xs', 'sm:w-64');
        expect(box).not.toHaveClass('px-3', 'text-sm');
    });
});

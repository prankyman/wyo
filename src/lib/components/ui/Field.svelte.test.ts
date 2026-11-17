import FieldTestHarness from '$test/harnesses/ui/FieldTestHarness.svelte';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

/**
 * The one field of the interface: the label in normal letters, its «?», the
 * box the caller passes and the refusal under it. This is the only test of how a field's label looks: the policy test
 * (`field-label-policy.test.ts`) makes every labelled field use this component, so no screen tests its labels again.
 */

describe('Field', () => {
    it('draws the label in normal letters above the box it points at, with the caller’s classes on the field', () => {
        render(FieldTestHarness, { props: { class: 'w-32 md:col-span-2' } });

        const label = screen.getByTestId('client-inn-label');
        expect(label.tagName).toBe('LABEL');
        expect(label).toHaveAttribute('for', 'inn');
        expect(label).toHaveTextContent('Tax number');
        expect(label).toHaveClass('text-xs', 'text-ink-600');
        expect(label.className).not.toMatch(/uppercase|tracking-/);
        expect(screen.getByLabelText('Tax number')).toBe(screen.getByTestId('client-inn'));

        const field = screen.getByTestId('client-inn-field');
        expect(field).toHaveClass('flex', 'flex-col', 'gap-1.5', 'w-32', 'md:col-span-2');
        expect(field).not.toHaveAttribute('data-field-error');
        expect(screen.queryByTestId('client-inn-help-button')).toBeNull();
        expect(screen.queryByTestId('client-inn-error')).toBeNull();
    });

    it('holds the box inside the label when there is no id, the «?» drawn beside the words but naming nothing', () => {
        render(FieldTestHarness, { props: { withFor: false, help: 'Ten or twelve digits.' } });

        // A label names the first control inside it; the «?» comes after the box, so the box keeps the label's name.
        const box = screen.getByTestId('client-inn');
        expect(box.closest('label')).not.toBeNull();
        expect(screen.getByLabelText(/Tax number/)).toBe(box);
        expect(screen.getByTestId('client-inn-help-button')).toHaveAttribute('aria-label', 'Ten or twelve digits.');
    });

    it('opens the help of the «?» beside the label', async () => {
        render(FieldTestHarness, { props: { help: 'Ten or twelve digits.' } });

        await fireEvent.click(screen.getByTestId('client-inn-help-button'));

        expect(screen.getByTestId('client-inn-help-tooltip')).toHaveTextContent('Ten or twelve digits.');
        expect(screen.getByTestId('client-inn-label')).toContainElement(screen.getByTestId('client-inn-help-button'));
    });

    it('draws a refusal under the box and marks the field for the section around it', () => {
        render(FieldTestHarness, { props: { error: 'The tax number has 10 or 12 digits.' } });

        expect(screen.getByTestId('client-inn-error')).toHaveTextContent('The tax number has 10 or 12 digits.');
        expect(screen.getByTestId('client-inn-error')).toHaveAttribute('role', 'alert');
        expect(screen.getByTestId('client-inn-field')).toHaveAttribute('data-field-error');
    });

    it('puts a tick box first and its words beside it in a 44 px row (the inline layout)', () => {
        render(FieldTestHarness, { props: { layout: 'inline', help: 'Shown on the act.' } });

        const label = screen.getByTestId('client-inn-label');
        expect(label).toHaveClass('min-h-11', 'text-sm');
        expect(label.firstElementChild).toBe(screen.getByTestId('client-inn'));
        expect(screen.getByLabelText('Tax number')).toBe(screen.getByTestId('client-inn'));
        // The «?» stands beside the label, outside it, so a press on it never ticks the box.
        expect(label).not.toContainElement(screen.getByTestId('client-inn-help-button'));
    });

    it.each([
        ['row', 'flex-wrap'],
        ['stacked', 'flex-col']
    ] as const)('lays the %s field out with %s', (layout, layoutClass) => {
        render(FieldTestHarness, { props: { layout } });

        expect(screen.getByTestId('client-inn-field')).toHaveClass(layoutClass);
    });

    it('draws the marks of `aside` on the label’s row, outside the label, so the box stays right under them', () => {
        render(FieldTestHarness, { props: { badge: 'Default' } });

        const label = screen.getByTestId('client-inn-label');
        const badge = screen.getByTestId('client-inn-badge');
        expect(label).not.toContainElement(badge);
        expect(badge.parentElement).toBe(label.parentElement);
        expect(label.parentElement?.nextElementSibling).toBe(screen.getByTestId('client-inn'));
    });

    it('gives the tap area of the inline layout at least 44 px of width as well', () => {
        render(FieldTestHarness, { props: { layout: 'inline' } });

        expect(screen.getByTestId('client-inn-label')).toHaveClass('min-h-11', 'min-w-11');
    });

    it('greys the label of a disabled box, and hides the words from sight but not from a screen reader', () => {
        render(FieldTestHarness, { props: { disabled: true, labelHidden: true } });

        expect(screen.getByTestId('client-inn-label')).toHaveClass('opacity-50', 'sr-only');
        expect(screen.getByLabelText('Tax number')).toBe(screen.getByTestId('client-inn'));
    });
});

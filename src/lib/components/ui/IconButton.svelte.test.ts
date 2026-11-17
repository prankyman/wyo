import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import IconButton from './IconButton.svelte';

// Consolidated from two files that both tested this component — IconButton.test.ts and
// IconButton.svelte.test.ts — 18 tests between them, most of them duplicates rendering the
// component again to assert one more class. Merged per the test-economy rules in
// .agents/skills/sveltekit-testing/SKILL.md §1: one behaviour per test, one render per
// test, and `it.each` for the variant enumeration.

describe('IconButton', () => {
    it('renders a 44 px button carrying the aria-label, and shows its name and sentence on hover (IconTooltip)', async () => {
        render(IconButton, {
            tooltip: 'Edit',
            description: 'Opens the item for editing.',
            ariaLabel: 'Edit item',
            'data-test': 'edit'
        });

        const button = screen.getByRole('button', { name: 'Edit item' });
        expect(button).toHaveAttribute('type', 'button');
        expect(button).toHaveClass('min-h-11', 'min-w-11');
        // Pressed feedback and a keyboard focus ring (constitution, press feedback and single submission).
        expect(button).toHaveClass(
            'motion-safe:pressable:active:scale-[0.97]',
            'touch-manipulation',
            'focus-visible:outline-2'
        );
        expect(screen.queryByRole('tooltip')).toBeNull();

        await fireEvent.mouseEnter(screen.getByTestId('edit-tip'));

        const tooltip = screen.getByRole('tooltip');
        expect(tooltip).toHaveTextContent('Edit');
        expect(tooltip).toHaveTextContent('Opens the item for editing.');
    });

    it('calls onclick when clicked', async () => {
        const onclick = vi.fn();
        render(IconButton, { tooltip: 'Edit', ariaLabel: 'Edit item', onclick });

        await fireEvent.click(screen.getByRole('button', { name: 'Edit item' }));

        expect(onclick).toHaveBeenCalledTimes(1);
    });

    it('applies the disabled attribute and opacity class when disabled', () => {
        render(IconButton, { tooltip: 'Delete', ariaLabel: 'Delete item', disabled: true });

        const button = screen.getByRole('button', { name: 'Delete item' });
        expect(button).toBeDisabled();
        expect(button.className).toContain('opacity-50');
    });

    // `as const` keeps `variant` a literal union rather than widening to `string`, which the
    // component's own prop type rejects.
    it.each([
        ['delete', 'text-stop-600'],
        ['archive', 'text-warn-600'],
        ['unarchive', 'text-ok-600']
    ] as const)('colours the %s variant with %s', (variant, expectedClass) => {
        render(IconButton, { tooltip: variant, ariaLabel: `${variant} item`, variant });

        expect(screen.getByRole('button', { name: `${variant} item` }).className).toContain(expectedClass);
    });

    it('renders a submit button and forwards a custom class when asked', () => {
        render(IconButton, {
            tooltip: 'Submit',
            ariaLabel: 'Submit form',
            type: 'submit',
            class: 'custom-class'
        });

        const button = screen.getByRole('button', { name: 'Submit form' });
        expect(button).toHaveAttribute('type', 'submit');
        expect(button.className).toContain('custom-class');
    });

    it('renders as an anchor when href is given', () => {
        render(IconButton, { tooltip: 'Edit', ariaLabel: 'Edit item', href: '/products/1/edit' });

        const link = screen.getByRole('link', { name: 'Edit item' });
        expect(link).toHaveAttribute('href', '/products/1/edit');
        expect(link).not.toHaveAttribute('aria-disabled');
    });

    it('as a disabled link has no address and says it is disabled, so it shows no hover or pressed look', () => {
        render(IconButton, {
            tooltip: 'Edit',
            ariaLabel: 'Edit item',
            href: '/products/1/edit',
            disabled: true
        });

        // Bootstrap's rule for a disabled `<a>` button: `aria-disabled="true"` and no `href`
        // (https://getbootstrap.com/docs/5.3/components/buttons/#disabled-state). `aria-disabled`
        // is also what the `pressable:` guard (src/app.css) reads.
        const link = screen.getByLabelText('Edit item');
        expect(link.tagName).toBe('A');
        expect(link).not.toHaveAttribute('href');
        expect(link).not.toHaveAttribute('disabled');
        expect(link).toHaveAttribute('aria-disabled', 'true');
        expect(link).toHaveClass('pressable:hover:text-ink-900');
        expect(link).not.toHaveClass('hover:text-ink-900');
    });

    it('forwards the data-test attribute to the rendered button', () => {
        render(IconButton, { tooltip: 'Edit', ariaLabel: 'Edit item', 'data-test': 'my-icon-button' });

        expect(screen.getByTestId('my-icon-button')).toBeInTheDocument();
    });
});

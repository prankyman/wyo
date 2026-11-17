import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import RemovableChip from './RemovableChip.svelte';

/**
 * The chip that takes a choice off (a picker's row, a filter in use): one control at least 44 px tall whose accessible name
 * says what a tap does — a button for a screen that changes its own state, a link for a choice that lives in the address.
 */

describe('RemovableChip', () => {
    it('is one button, at least 44 px tall, named by what a tap does; a tap calls onRemove', async () => {
        const onRemove = vi.fn();
        render(RemovableChip, {
            props: { label: 'A-01', ariaLabel: 'Remove A-01', onRemove, dataTest: 'chip' }
        });

        const chip = screen.getByTestId('chip');
        expect(chip.tagName).toBe('BUTTON');
        expect(chip).toHaveAttribute('type', 'button');
        expect(chip).toHaveClass('min-h-11');
        expect(screen.getByRole('button', { name: 'Remove A-01' })).toBe(chip);
        expect(chip).toHaveTextContent('A-01');

        await fireEvent.click(chip);
        expect(onRemove).toHaveBeenCalledTimes(1);
    });

    it('shows a quieter second part after the label, only when there is one', () => {
        const { unmount } = render(RemovableChip, {
            props: { label: 'A-01', sublabel: 'Zone A', ariaLabel: 'Remove A-01 Zone A', dataTest: 'chip' }
        });
        expect(screen.getByTestId('chip')).toHaveTextContent('A-01 Zone A');
        unmount();

        render(RemovableChip, { props: { label: 'A-01', ariaLabel: 'Remove A-01', dataTest: 'chip' } });
        expect(screen.getByTestId('chip').querySelector('.text-ink-400:not(svg)')).toBeNull();
    });

    it('is a link, 44 px tall, when the choice lives in the address; the cross is hidden from a screen reader', () => {
        render(RemovableChip, {
            props: {
                label: 'Status: Active',
                ariaLabel: 'Remove Status: Active',
                href: '/en/items',
                dataTest: 'chip'
            }
        });

        const chip = screen.getByRole('link', { name: 'Remove Status: Active' });
        expect(chip).toHaveAttribute('href', '/en/items');
        expect(chip).toHaveClass('min-h-11');
        expect(chip.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    });
});

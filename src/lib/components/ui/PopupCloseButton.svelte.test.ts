import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import PopupCloseButton from './PopupCloseButton.svelte';

describe('PopupCloseButton', () => {
    it('is a labelled button with a cross that calls onclick', async () => {
        const onclick = vi.fn();
        const { container } = render(PopupCloseButton, { props: { onclick, testId: 'popup-close' } });
        const button = screen.getByTestId('popup-close');

        expect(button).toHaveAttribute('type', 'button');
        expect(button).toHaveAttribute('aria-label');
        expect(container.querySelector('svg')).toBeInTheDocument();

        await fireEvent.click(button);
        expect(onclick).toHaveBeenCalledOnce();
    });

    it('floats to the right so the text wraps around it, and is white on the popup colour', () => {
        render(PopupCloseButton, { props: { onclick: vi.fn(), testId: 'popup-close' } });
        const button = screen.getByTestId('popup-close');

        expect(button).toHaveClass('float-right', 'min-h-11', 'min-w-11', 'text-white');
        expect(button).not.toHaveClass('sticky', 'absolute', 'fixed');
    });
});

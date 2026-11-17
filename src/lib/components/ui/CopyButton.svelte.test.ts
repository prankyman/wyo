import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import CopyButton from './CopyButton.svelte';

/** The copy button with its answer: the places that draw it test only what they copy. */
afterEach(() => {
    vi.unstubAllGlobals();
});

const props = {
    text: 'https://example.org/shared/link',
    label: 'Копировать',
    copiedLabel: 'Скопировано',
    failedLabel: 'Скопируйте вручную',
    dataTest: 'link-copy'
};

describe('CopyButton', () => {
    it('copies its text and says so in green', async () => {
        const writeText = vi.fn(async () => undefined);
        vi.stubGlobal('navigator', { clipboard: { writeText } });
        render(CopyButton, { props });

        expect(screen.getByTestId('link-copy-state')).toBeEmptyDOMElement();
        await fireEvent.click(screen.getByTestId('link-copy'));

        expect(writeText).toHaveBeenCalledWith('https://example.org/shared/link');
        expect(await screen.findByText('Скопировано')).toHaveClass('text-ok-800');
        expect(screen.getByTestId('link-copy')).toHaveTextContent('Копировать');
    });

    it('asks to copy by hand when the browser refuses the clipboard', async () => {
        vi.stubGlobal('navigator', {
            clipboard: { writeText: vi.fn(async () => Promise.reject(new Error('denied'))) }
        });
        render(CopyButton, { props });

        await fireEvent.click(screen.getByTestId('link-copy'));

        expect(await screen.findByText('Скопируйте вручную')).toHaveClass('text-warn-800');
    });

    // In a row of a list the copy button is an icon beside the red delete icon, named by its tooltip.
    it('draws only a copy icon named by its words, and after a copy says «Скопировано» to a screen reader', async () => {
        const writeText = vi.fn(async () => undefined);
        vi.stubGlobal('navigator', { clipboard: { writeText } });
        render(CopyButton, { props: { ...props, look: 'icon' } });

        const button = screen.getByTestId('link-copy');
        expect(button).toHaveAttribute('aria-label', 'Копировать');
        expect(button).not.toHaveTextContent('Копировать');
        expect(button.querySelector('svg')).not.toBeNull();

        await fireEvent.click(button);

        expect(writeText).toHaveBeenCalledWith('https://example.org/shared/link');
        const state = screen.getByTestId('link-copy-state');
        expect(state).toHaveTextContent('Скопировано');
        expect(state).toHaveClass('sr-only');
    });
});

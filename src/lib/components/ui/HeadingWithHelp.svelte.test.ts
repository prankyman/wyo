import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import HeadingWithHelp from './HeadingWithHelp.svelte';

describe('HeadingWithHelp', () => {
    it('draws a caption in small grey capitals with its «?» right beside it in one row, and the «?» opens its text', async () => {
        const { container } = render(HeadingWithHelp, {
            props: {
                text: 'Stops',
                help: 'The addresses the driver visits.',
                testId: 'trip-stops-help',
                headingTestId: 'trip-stops-heading'
            }
        });

        const row = container.firstElementChild as HTMLElement;
        expect(row).toHaveClass('flex', 'items-center', 'gap-1');
        const heading = screen.getByTestId('trip-stops-heading');
        expect(heading.tagName).toBe('H2');
        expect(heading).toHaveClass('text-xs', 'tracking-widest', 'text-ink-500', 'uppercase');
        expect(heading).toHaveTextContent('Stops');
        // The «?» is the heading's next sibling in the same row — never on a line of its own under it.
        expect(heading.nextElementSibling).toContainElement(screen.getByTestId('trip-stops-help-button'));
        expect(screen.getByTestId('trip-stops-help-button')).toHaveAttribute('aria-label', 'Stops');

        await fireEvent.click(screen.getByTestId('trip-stops-help-button'));
        expect(screen.getByTestId('trip-stops-help-tooltip')).toHaveTextContent('The addresses the driver visits.');
    });

    it('stands alone in its look, with its id, when a block has no «?» in this state', () => {
        const { container } = render(HeadingWithHelp, {
            props: { text: 'Version 3', look: 'label', level: 'h4', id: 'version-3', headingTestId: 'version-heading' }
        });

        expect(screen.getByTestId('version-heading')).toHaveAttribute('id', 'version-3');
        expect(container.querySelector('[data-help-icon]')).toBeNull();
    });

    it('draws the heading of a card’s section in its own look', () => {
        render(HeadingWithHelp, {
            props: {
                text: 'Packaging',
                help: 'What the step does.',
                look: 'section',
                testId: 'step-help',
                headingTestId: 'step-heading'
            }
        });

        expect(screen.getByTestId('step-heading')).toHaveClass('text-sm', 'font-medium', 'text-ink-900');
        expect(screen.getByTestId('step-heading')).not.toHaveClass('uppercase', 'font-display');
    });

    it('draws a section title at the level and with the name, tour anchor and classes it is given', () => {
        const { container } = render(HeadingWithHelp, {
            props: {
                text: 'Параметры отправки',
                help: 'What the form does.',
                ariaLabel: 'About the shipping options section',
                level: 'h3',
                look: 'title',
                testId: 'batch-options-help',
                headingTestId: 'batch-options-heading',
                dataTour: 'batch-options',
                class: 'mb-2'
            }
        });

        const row = container.firstElementChild as HTMLElement;
        expect(row).toHaveClass('mb-2');
        expect(row).toHaveAttribute('data-tour', 'batch-options');
        const heading = screen.getByTestId('batch-options-heading');
        expect(heading.tagName).toBe('H3');
        expect(heading).toHaveClass('text-lg', 'text-ink-900');
        expect(heading).not.toHaveClass('uppercase');
        expect(screen.getByTestId('batch-options-help-button')).toHaveAttribute(
            'aria-label',
            'About the shipping options section'
        );
    });
});

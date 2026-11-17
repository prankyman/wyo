import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import AttentionIcon from './AttentionIcon.svelte';

describe('AttentionIcon', () => {
    it('renders the warning triangle at the size it is given', () => {
        const { container } = render(AttentionIcon, { props: { size: 'w-8 h-8', tooltip: 'Something is wrong' } });

        const button = screen.getByTestId('attention-icon-button');
        expect(button).toHaveAttribute('type', 'button');
        expect(container.querySelector('svg')?.getAttribute('class')?.split(/\s+/)).toEqual(
            expect.arrayContaining(['w-8', 'h-8'])
        );
    });

    it('is a tap target of at least 44 x 44 pixels, whatever the triangle’s size', () => {
        render(AttentionIcon, { props: { tooltip: 'Something is wrong' } });

        const classes = screen.getByTestId('attention-icon-button').className.split(/\s+/);
        expect(classes).toEqual(expect.arrayContaining(['min-h-11', 'min-w-11']));
    });

    it('keeps the explanation hidden until it is asked for', () => {
        render(AttentionIcon, { props: { tooltip: 'Prices below minimum' } });

        expect(document.body.querySelector('[data-test="attention-icon-tooltip"]')).not.toBeInTheDocument();
    });

    it('does not open on hover', async () => {
        render(AttentionIcon, { props: { tooltip: 'Prices below minimum', testId: 'hover-case' } });

        await fireEvent.mouseEnter(screen.getByTestId('hover-case-button'));

        expect(document.body.querySelector('[data-test="hover-case-tooltip"]')).not.toBeInTheDocument();
    });

    // The reason this component was rewritten: a CSS group-hover tooltip is unreachable
    // on a touch device, so the warning showed with no way to read what it meant.
    it('opens the explanation on click, which is the only way to reach it on touch', async () => {
        render(AttentionIcon, { props: { tooltip: 'Prices below minimum', testId: 'tap-case' } });
        const button = screen.getByTestId('tap-case-button');

        await fireEvent.click(button);
        expect(screen.getByTestId('tap-case-tooltip')).toHaveTextContent('Prices below minimum');

        await fireEvent.click(button);
        expect(document.body.querySelector('[data-test="tap-case-tooltip"]')).not.toBeInTheDocument();
    });

    it('closes on Escape, on a click outside it, and on its cross, and puts the focus back on the icon', async () => {
        render(AttentionIcon, { props: { tooltip: 'Prices below minimum', testId: 'close-case' } });
        const button = screen.getByTestId('close-case-button');
        const tooltip = () => document.body.querySelector('[data-test="close-case-tooltip"]');

        await fireEvent.click(button);
        await fireEvent.keyDown(document, { key: 'Escape' });
        expect(tooltip()).not.toBeInTheDocument();
        expect(button).toHaveFocus();

        await fireEvent.click(button);
        await fireEvent.click(document.body);
        expect(tooltip()).not.toBeInTheDocument();

        await fireEvent.click(button);
        await fireEvent.click(screen.getByText('Prices below minimum'));
        expect(tooltip()).toBeInTheDocument();
        await fireEvent.click(screen.getByTestId('close-case-close'));
        expect(tooltip()).not.toBeInTheDocument();
        expect(button).toHaveFocus();
    });

    it('lists every issue when a product has more than one', async () => {
        render(AttentionIcon, {
            props: { issues: ['No variants', '2 variants below minimum'], testId: 'multi-case' }
        });

        await fireEvent.click(screen.getByTestId('multi-case-button'));
        const tooltip = screen.getByTestId('multi-case-tooltip');
        expect(tooltip.querySelectorAll('li')).toHaveLength(2);
        expect(tooltip).toHaveTextContent('No variants');
        expect(tooltip).toHaveTextContent('2 variants below minimum');
    });

    it('stays silent when there is nothing to explain', async () => {
        render(AttentionIcon, { props: { testId: 'empty-case' } });

        await fireEvent.click(screen.getByTestId('empty-case-button'));
        expect(document.body.querySelector('[data-test="empty-case-tooltip"]')).not.toBeInTheDocument();
    });

    it('portals the tooltip out of its row, so a stacking context cannot clip it', async () => {
        const { container } = render(AttentionIcon, { props: { tooltip: 'Warning', testId: 'portal-case' } });

        await fireEvent.click(screen.getByTestId('portal-case-button'));
        const tooltip = screen.getByTestId('portal-case-tooltip');
        expect(container).not.toContainElement(tooltip);
        expect(tooltip.className).toContain('z-10000');
    });
});

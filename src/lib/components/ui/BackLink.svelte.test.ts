import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import BackLink from './BackLink.svelte';

/**
 * The one «← назад» link of every detail screen: the same grey uppercase look as the
 * 41 hand-written copies it replaces, in a box of at least 44 × 44 px with no negative margin. The box is
 * measured in a real browser by the phone sweep; here the markup is proven — which element carries which
 * attribute — because jsdom lays nothing out.
 */

const tokens = (element: HTMLElement): string[] => (element.getAttribute('class') ?? '').split(/\s+/).filter(Boolean);

describe('BackLink', () => {
    it('is a link to its address that reads «← label» and keeps its data-test and data-tour', () => {
        render(BackLink, {
            props: { href: '/records', label: 'Clients', dataTest: 'client-back', dataTour: 'client-back-tour' }
        });

        const link = screen.getByTestId('client-back');
        expect(link.tagName).toBe('A');
        expect(link).toHaveAttribute('href', '/records');
        expect(link).toHaveAttribute('data-tour', 'client-back-tour');
        expect(link).toHaveTextContent(/^← Clients$/);
        expect(screen.getByRole('link', { name: '← Clients' })).toBe(link);
    });

    it('has no data-tour attribute unless it is given one', () => {
        render(BackLink, { props: { href: '/records', label: 'Clients', dataTest: 'client-back' } });

        expect(screen.getByTestId('client-back')).not.toHaveAttribute('data-tour');
    });

    it('draws the label alone when the message already begins with its own arrow', () => {
        render(BackLink, {
            props: { href: '/archive', label: '← Returns', dataTest: 'returns-back', arrow: false }
        });

        expect(screen.getByTestId('returns-back')).toHaveTextContent(/^← Returns$/);
    });

    it('draws a box of at least 44 × 44 px in the normal layout, with no negative margin and no padding trick', () => {
        render(BackLink, { props: { href: '/records', label: 'Clients', dataTest: 'client-back' } });

        const classes = tokens(screen.getByTestId('client-back'));
        expect(classes).toEqual(expect.arrayContaining(['inline-flex', 'min-h-11', 'min-w-11', 'items-center']));
        // The old links were 16 px of text made 40 px by `py-3 -my-3`; a negative margin lets the box sit over
        // its neighbours and take their taps.
        expect(classes.filter((token) => /(^|:)-m/.test(token))).toEqual([]);
        expect(classes).not.toContain('py-3');
    });

    it('keeps the look of the link it replaces: small, spaced, grey, uppercase, darker under the pointer', () => {
        render(BackLink, { props: { href: '/records', label: 'Clients', dataTest: 'client-back' } });

        expect(screen.getByTestId('client-back')).toHaveClass(
            'text-xs',
            'tracking-wider',
            'text-ink-500',
            'uppercase',
            'pressable:hover:text-ink-900'
        );
    });

    it('shows that it was pressed and a keyboard focus, like every pressable control', () => {
        render(BackLink, { props: { href: '/records', label: 'Clients', dataTest: 'client-back' } });

        expect(screen.getByTestId('client-back')).toHaveClass(
            'touch-manipulation',
            'motion-safe:pressable:active:scale-[0.97]',
            'focus-visible:outline-2'
        );
    });

    it('lets a screen add a class and pass any other link attribute through', async () => {
        const onclick = vi.fn((event: MouseEvent) => event.preventDefault());
        render(BackLink, {
            props: {
                href: '/records',
                label: 'Clients',
                dataTest: 'client-back',
                class: 'ml-auto',
                onclick,
                'aria-label': 'Back to all clients',
                'data-sveltekit-reload': ''
            }
        });

        const link = screen.getByTestId('client-back');
        expect(link).toHaveClass('ml-auto', 'min-h-11');
        expect(link).toHaveAttribute('aria-label', 'Back to all clients');
        expect(link).toHaveAttribute('data-sveltekit-reload');

        await fireEvent.click(link);
        expect(onclick).toHaveBeenCalledTimes(1);
    });
});

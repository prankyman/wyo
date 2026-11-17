import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import WarningBanner from './WarningBanner.svelte';

describe('WarningBanner', () => {
    it('states the problem and its explanation', () => {
        render(WarningBanner, {
            props: { title: 'Нужен ключ', message: 'Значение задаётся у ключа.', dataTest: 'banner' }
        });

        const banner = screen.getByTestId('banner');
        expect(banner).toHaveTextContent('Нужен ключ');
        expect(banner).toHaveTextContent('Значение задаётся у ключа.');
    });

    // The colours are `NoteBox`'s (tested there); the banner only picks the tone of its variant.
    it.each([
        ['warning', undefined, 'warning'],
        ['danger', 'danger', 'error']
    ] as const)('draws the %s form as a note of its tone, with no live role', (_name, variant, tone) => {
        render(WarningBanner, { props: { title: 't', message: 'm', dataTest: 'banner', variant } });

        const banner = screen.getByTestId('banner');
        expect(banner).toHaveAttribute('data-tone', tone);
        expect(banner).not.toHaveAttribute('role');
        expect(screen.getByText('t').tagName).toBe('H3');
        expect(screen.getByText('t')).toHaveClass('font-medium');
    });

    it('runs the callback when its action is a button', async () => {
        const onAction = vi.fn();
        render(WarningBanner, {
            props: { title: 't', message: 'm', actionLabel: 'Добавить', onAction, dataTest: 'banner' }
        });

        await fireEvent.click(screen.getByTestId('banner-action'));
        expect(onAction).toHaveBeenCalledOnce();
    });

    it('renders a link when its action is a destination', () => {
        render(WarningBanner, {
            props: { title: 't', message: 'm', actionLabel: 'Открыть', actionHref: '/x', dataTest: 'banner' }
        });

        expect(screen.getByTestId('banner-action')).toHaveAttribute('href', '/x');
    });

    // A label with nowhere to go is a control that does nothing — the create page passes
    // no action precisely because there is nothing to act on until the product is saved.
    it('renders no action when there is nothing to do about it yet', () => {
        render(WarningBanner, { props: { title: 't', message: 'm', actionLabel: 'Добавить', dataTest: 'banner' } });

        expect(screen.queryByTestId('banner-action')).not.toBeInTheDocument();
    });
});

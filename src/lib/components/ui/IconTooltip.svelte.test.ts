import IconTooltipTestHarness from '$test/harnesses/ui/IconTooltipTestHarness.svelte';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

/**
 * The tooltip of a header icon: its name and what it does, on hover and on keyboard focus, never on a tap;
 * gone on Escape and when the mouse leaves; not shown from 768 px up when the header draws words there.
 */

const matchMedia = (matches: boolean) =>
    vi.stubGlobal(
        'matchMedia',
        vi.fn(() => ({ matches }) as MediaQueryList)
    );

afterEach(() => vi.unstubAllGlobals());

describe('IconTooltip', () => {
    it('shows the name and the sentence on hover, keeps them while the mouse moves onto them, and hides them after', async () => {
        render(IconTooltipTestHarness);

        await fireEvent.mouseEnter(screen.getByTestId('logout-tip'));
        const tip = screen.getByRole('tooltip');
        expect(tip).toHaveTextContent('Sign out');
        expect(tip).toHaveTextContent('Ends the sign-in on this device.');

        await fireEvent.mouseLeave(screen.getByTestId('logout-tip'));
        await fireEvent.mouseEnter(tip);
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        await fireEvent.mouseLeave(tip);
        await waitFor(() => expect(screen.queryByRole('tooltip')).toBeNull());
    });

    it('shows for a keyboard focus but not for the focus a tap leaves, and goes on Escape', async () => {
        render(IconTooltipTestHarness);
        const link = screen.getByTestId('logout');
        const visible = vi.spyOn(link, 'matches');

        visible.mockReturnValue(false);
        await fireEvent.focusIn(link);
        expect(screen.queryByRole('tooltip')).toBeNull();

        visible.mockReturnValue(true);
        await fireEvent.focusIn(link);
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(screen.queryByRole('tooltip')).toBeNull();
    });

    it.each([
        [true, false],
        [false, true]
    ])('for a header icon, with the words drawn (768 px or wider) = %s, shows the tooltip: %s', async (wide, shown) => {
        matchMedia(wide);
        render(IconTooltipTestHarness, { props: { onlyBelowMd: true } });

        await fireEvent.mouseEnter(screen.getByTestId('logout-tip'));

        expect(screen.queryByRole('tooltip') !== null).toBe(shown);
    });
});

import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import Harness from '$test/harnesses/ui/DismissableTestHarness.svelte';

describe('dismissable', () => {
    it('closes on a click outside the panel, but not on one inside it or on its own trigger', async () => {
        const onDismiss = vi.fn();
        render(Harness, { props: { onDismiss } });

        await fireEvent.click(screen.getByTestId('inside'));
        // The trigger toggles the panel itself; dismissing here as well would fight it.
        await fireEvent.click(screen.getByTestId('trigger'));
        expect(onDismiss).not.toHaveBeenCalled();

        await fireEvent.click(screen.getByTestId('outside'));
        expect(onDismiss).toHaveBeenCalledTimes(1);
    });

    it('closes on Escape and hands focus back to the trigger, ignoring every other key', async () => {
        const onDismiss = vi.fn();
        render(Harness, { props: { onDismiss } });

        await fireEvent.keyDown(document, { key: 'Enter' });
        expect(onDismiss).not.toHaveBeenCalled();

        await fireEvent.keyDown(document, { key: 'Escape' });
        expect(onDismiss).toHaveBeenCalledTimes(1);
        expect(document.activeElement).toBe(screen.getByTestId('trigger'));
    });

    it('stops listening once the panel is torn down', async () => {
        const onDismiss = vi.fn();
        const { unmount } = render(Harness, { props: { onDismiss } });

        unmount();
        await fireEvent.click(document.body);
        await fireEvent.keyDown(document, { key: 'Escape' });

        expect(onDismiss).not.toHaveBeenCalled();
    });
});

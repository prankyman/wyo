import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import CounterPill from './CounterPill.svelte';

/**
 * A menu counter that explains itself: a button whose sentence is its accessible name and its pop-up. The pop-up
 * opens on the mouse and on keyboard focus, and on a phone with a tap; it closes on Escape and on a tap elsewhere.
 */

const SENTENCE = 'Three things wait for you';
const PROPS = { count: 3, sentences: [SENTENCE], testId: 'nav-settings-count' };

const counter = () => screen.getByTestId('nav-settings-count');

/** A tap as the browser delivers it: the finger goes down, then the click follows. */
async function tap(element: HTMLElement, pointerType = 'touch') {
    await fireEvent.pointerDown(element, { pointerType });
    await fireEvent.click(element);
}

describe('CounterPill', () => {
    it('is a button named by its sentence, and draws the number only', () => {
        render(CounterPill, { props: PROPS });

        expect(counter().tagName).toBe('BUTTON');
        expect(counter()).toHaveAttribute('aria-label', SENTENCE);
        expect(counter()).toHaveTextContent('3');
        expect(screen.queryByRole('tooltip')).toBeNull();
    });

    it('draws the number white on red, for every counter', () => {
        render(CounterPill, { props: PROPS });

        const number = counter().querySelector('span');
        expect(number).toHaveClass('bg-stop-600', 'text-white');
        expect(number).toHaveAttribute('aria-hidden', 'true');
        expect(number?.className).not.toContain('warn');
    });

    it('keeps the number red on the phone’s compact strip too', () => {
        render(CounterPill, { props: { ...PROPS, count: 12, compact: true } });

        expect(counter().querySelector('span')).toHaveClass('bg-stop-600', 'text-white', 'text-[0.625rem]');
        expect(counter()).toHaveTextContent('12');
    });

    it('opens its sentence while the mouse is on it, keeps it while the mouse moves onto it, and closes it after', async () => {
        render(CounterPill, { props: PROPS });

        await fireEvent.pointerEnter(counter(), { pointerType: 'mouse' });
        const tip = screen.getByRole('tooltip');
        expect(tip).toHaveTextContent(SENTENCE);

        await fireEvent.pointerLeave(counter(), { pointerType: 'mouse' });
        await fireEvent.mouseEnter(tip);
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        await fireEvent.mouseLeave(tip);
        await waitFor(() => expect(screen.queryByRole('tooltip')).toBeNull());
    });

    it('opens for a keyboard focus but not for the focus a tap leaves, and closes on Escape', async () => {
        render(CounterPill, { props: PROPS });
        const visible = vi.spyOn(counter(), 'matches');

        visible.mockReturnValue(false);
        await fireEvent.focusIn(counter());
        expect(screen.queryByRole('tooltip')).toBeNull();

        visible.mockReturnValue(true);
        await fireEvent.focusIn(counter());
        expect(screen.getByRole('tooltip')).toHaveTextContent(SENTENCE);

        await fireEvent.keyDown(document, { key: 'Escape' });
        expect(screen.queryByRole('tooltip')).toBeNull();
    });

    it('opens on a touch tap, ignores the mouse events a touch sends first, and closes on a second tap', async () => {
        render(CounterPill, { props: PROPS });

        await fireEvent.pointerEnter(counter(), { pointerType: 'touch' });
        expect(screen.queryByRole('tooltip')).toBeNull();

        await tap(counter());
        expect(screen.getByRole('tooltip')).toHaveTextContent(SENTENCE);

        await tap(counter());
        expect(screen.queryByRole('tooltip')).toBeNull();
    });

    it('closes an open sentence on a tap elsewhere and on Escape', async () => {
        render(CounterPill, { props: PROPS });

        await tap(counter());
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        await fireEvent.click(document.body);
        expect(screen.queryByRole('tooltip')).toBeNull();

        await tap(counter());
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        await fireEvent.keyDown(document, { key: 'Escape' });
        expect(screen.queryByRole('tooltip')).toBeNull();
    });

    it('keeps a mouse click from closing the sentence the hover opened', async () => {
        render(CounterPill, { props: PROPS });

        await fireEvent.pointerEnter(counter(), { pointerType: 'mouse' });
        await tap(counter(), 'mouse');

        expect(screen.getByRole('tooltip')).toBeInTheDocument();
    });

    it('cancels the click it gets, so the link or group heading it stands in has no default action to run', async () => {
        render(CounterPill, { props: PROPS });
        await fireEvent.pointerDown(counter(), { pointerType: 'touch' });

        const notCancelled = await fireEvent.click(counter());

        expect(notCancelled).toBe(false);
    });

    it('says one sentence per line for a group, and is named by all of them', async () => {
        render(CounterPill, {
            props: { count: 5, sentences: [SENTENCE, '2 more things wait'], testId: 'nav-settings-count' }
        });

        expect(counter()).toHaveAttribute('aria-label', `${SENTENCE}; 2 more things wait`);
        await tap(counter());
        const lines = Array.from(screen.getByRole('tooltip').querySelectorAll('p')).map((line) => line.textContent);
        expect(lines).toEqual([SENTENCE, '2 more things wait']);
    });

    it.each([
        [false, 'size-11', null],
        [true, 'size-6', '']
    ])('with compact = %s the tap area is %s and the sweep mark is %j', (compact, size, mark) => {
        render(CounterPill, { props: { ...PROPS, compact } });

        expect(counter()).toHaveClass(size);
        expect(counter().getAttribute('data-counter')).toBe(mark);
    });
});

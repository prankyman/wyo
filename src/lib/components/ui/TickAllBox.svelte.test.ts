import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import TickAllBox from './TickAllBox.svelte';
import { tickAllState } from './tick-all';

// The tick box in a list's heading, in place of the buttons «Отметить все показанные» and «Снять отметки».
describe('tickAllState', () => {
    it.each([
        [0, 0, 'none'],
        [0, 3, 'none'],
        [1, 3, 'some'],
        [3, 3, 'all']
    ] as const)('names %i ticked of %i «%s»', (ticked, total, state) => {
        expect(tickAllState(ticked, total)).toBe(state);
    });
});

describe('TickAllBox', () => {
    const box = () => screen.getByTestId('rows-tick-all') as HTMLInputElement;

    it('is empty and ticks every row when nothing is ticked, its name «Отметить все показанные» on hover', async () => {
        const onTick = vi.fn();
        render(TickAllBox, { lang: 'ru', status: 'none', onTick, dataTest: 'rows-tick-all' });

        expect(box()).not.toBeChecked();
        expect(box().indeterminate).toBe(false);
        expect(box()).toHaveAccessibleName('Отметить все показанные');
        await fireEvent.mouseEnter(screen.getByTestId('rows-tick-all-tip'));
        expect(screen.getByRole('tooltip')).toHaveTextContent('Отметить все показанные');

        await fireEvent.click(box());
        expect(onTick).toHaveBeenCalledWith(true);
    });

    it('shows a dash while some rows are ticked, and a press ticks all of them', async () => {
        const onTick = vi.fn();
        render(TickAllBox, { lang: 'ru', status: 'some', onTick, dataTest: 'rows-tick-all' });

        expect(box().indeterminate).toBe(true);
        expect(box()).toHaveAccessibleName('Отметить все показанные');
        await fireEvent.click(box());
        expect(onTick).toHaveBeenCalledWith(true);
        // The page has not answered yet: the box still says what is ticked, not what the browser's press made of it.
        expect(box().indeterminate).toBe(true);
        expect(box()).not.toBeChecked();
    });

    it('is ticked while every row is, says «Снять отметки» and a press unticks them', async () => {
        const onTick = vi.fn();
        const { rerender } = render(TickAllBox, { lang: 'ru', status: 'none', onTick, dataTest: 'rows-tick-all' });
        await rerender({ status: 'all' });

        expect(box()).toBeChecked();
        expect(box().indeterminate).toBe(false);
        expect(box()).toHaveAccessibleName('Снять отметки');
        await fireEvent.mouseEnter(screen.getByTestId('rows-tick-all-tip'));
        expect(screen.getByRole('tooltip')).toHaveTextContent('Снять отметки');

        await fireEvent.click(box());
        expect(onTick).toHaveBeenCalledWith(false);
    });

    it('takes the words of its page and the look of its rows, and cannot be pressed with no rows to tick', () => {
        render(TickAllBox, {
            lang: 'ru',
            status: 'none',
            onTick: vi.fn(),
            dataTest: 'rows-tick-all',
            tickLabel: 'Отметить все на странице',
            disabled: true,
            inputClass: 'size-5'
        });

        expect(box()).toHaveAccessibleName('Отметить все на странице');
        expect(box()).toBeDisabled();
        expect(box()).toHaveClass('size-5');
        expect(box().closest('label')).toHaveClass('justify-center');
    });

    it('stands at the start of its tap area over rows whose boxes do', () => {
        render(TickAllBox, { lang: 'ru', status: 'none', onTick: vi.fn(), dataTest: 'rows-tick-all', align: 'start' });
        expect(box().closest('label')).toHaveClass('justify-start');
    });
});

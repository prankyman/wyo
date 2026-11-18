import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { messages } from '$lib/i18n/messages';
import ShowcaseLists from './ShowcaseLists.svelte';

// The section only draws the list components of the library — `DataList` with `ColumnHeading` and `ColumnLegend`, `TickAllBox`,
// `DateRangeFields` and `FactList`; each is tested once in its own file. This test holds that every block is drawn, in each state
// the block promises to show.
const text = (key: string): string => messages.ru[key];

describe('the lists section of the showcase', () => {
    it('draws a block for every group of list components, each with its own heading', () => {
        const { container } = render(ShowcaseLists);

        expect(screen.getByTestId('showcase-lists')).toBeInTheDocument();
        for (const name of ['list', 'empty', 'ticks', 'dates', 'facts']) {
            expect(screen.getByTestId(`showcase-lists-${name}`).querySelector('h2'), name).not.toBeNull();
        }
        // The page's own title is the only first-level heading; the section adds none.
        expect(container.querySelectorAll('h1')).toHaveLength(0);
    });

    it('draws the list as a table and as cards, with the columns’ «?», the legend, a badge, a late and a closed row and tick boxes', () => {
        render(ShowcaseLists);

        const table = screen.getByTestId('showcase-lists-list-table');
        expect(within(table).getAllByTestId('showcase-lists-list-row')).toHaveLength(3);
        expect(
            within(screen.getByTestId('showcase-lists-list-cards')).getAllByTestId('showcase-lists-list-card')
        ).toHaveLength(3);
        for (const column of ['number', 'status', 'quantity']) {
            expect(within(table).getByTestId(`showcase-lists-list-${column}-help-button`), column).toBeInTheDocument();
        }
        expect(screen.getByTestId('showcase-lists-list-legend')).toHaveTextContent(
            text('showcase.lists.column.quantity')
        );
        expect(within(table).getByTestId('showcase-lists-list-badge')).toHaveTextContent(text('showcase.lists.badge'));
        expect(table.querySelector('[data-row-id="1002"]')).toHaveClass('text-stop-700');
        expect(table.querySelector('[data-row-id="1003"]')).toHaveClass('text-ink-400');
        expect(screen.getAllByTestId('showcase-lists-list-select')).toHaveLength(6);
        expect(screen.getByTestId('showcase-lists-list-tick-all')).toHaveAttribute('data-state', 'some');

        expect(screen.getByTestId('showcase-lists-empty-list-empty')).toHaveTextContent(
            text('showcase.lists.empty.message')
        );
    });

    it('draws the heading tick box in each state, the period plain, refused and dimmed, and the facts with and without a «?»', () => {
        render(ShowcaseLists);

        for (const state of ['none', 'some', 'all']) {
            expect(screen.getByTestId(`showcase-lists-tick-${state}`), state).toHaveAttribute('data-state', state);
        }
        expect(screen.getByTestId('showcase-lists-tick-disabled')).toBeDisabled();

        for (const look of ['plain', 'refused', 'disabled']) {
            expect(screen.getByTestId(`showcase-lists-dates-${look}-from`), look).toHaveAttribute('type', 'date');
            expect(screen.getByTestId(`showcase-lists-dates-${look}-to`), look).toHaveAttribute('type', 'date');
        }
        expect(screen.getByTestId('showcase-lists-dates')).toHaveTextContent(text('showcase.lists.dates.refusal'));
        expect(screen.getByTestId('showcase-lists-dates-disabled-from')).toBeDisabled();

        expect(screen.getByTestId('showcase-lists-fact-list-number')).toHaveTextContent('1001');
        expect(screen.getByTestId('showcase-lists-fact-list-number-help-button')).toBeInTheDocument();
        expect(screen.getByTestId('showcase-lists-fact-list-note')).toHaveTextContent(
            text('showcase.lists.facts.noteValue')
        );
        expect(screen.queryByTestId('showcase-lists-fact-list-note-help-button')).toBeNull();
    });
});

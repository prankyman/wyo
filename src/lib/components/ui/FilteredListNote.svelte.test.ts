import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import FilteredListNote from './FilteredListNote.svelte';

/** The line above a filtered list and the press that shows everything again. */
describe('FilteredListNote', () => {
    it('says what is shown and offers the press that lists everything, as a link to where the caller sends it', () => {
        render(FilteredListNote, {
            props: {
                text: 'Only items with the status Active are shown.',
                clearLabel: 'Show all items',
                clearHref: '/en/items',
                dataTest: 'note',
                clearDataTest: 'note-clear'
            }
        });

        expect(screen.getByTestId('note')).toHaveTextContent('Only items with the status Active are shown.');
        const clear = screen.getByTestId('note-clear');
        expect(clear).toHaveTextContent('Show all items');
        expect(clear).toHaveAttribute('href', '/en/items');
    });
});

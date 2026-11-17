import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import FieldLabel from './FieldLabel.svelte';

/**
 * The label row of a form field: as tall as its text, with or without a «?», so the boxes of two fields side by
 * side start at the same height. The measuring is the browser test `e2e/specs/form-box-alignment.spec.ts`.
 */

describe('FieldLabel', () => {
    it('draws the text in a row with no height of its own, and no button when there is no help', () => {
        render(FieldLabel, { props: { text: 'Size class', testId: 'size-class' } });

        const row = screen.getByTestId('size-class-label');
        expect(row).toHaveTextContent('Size class');
        expect(row).toHaveClass('flex', 'items-center');
        expect(row).not.toHaveClass('min-h-11');
        expect(screen.queryByTestId('size-class-help-button')).not.toBeInTheDocument();
    });

    it('draws the same row with the «?» beside the text, which opens the help', async () => {
        render(FieldLabel, {
            props: { text: 'Seconds per unit', help: 'How long one unit takes.', testId: 'seconds' }
        });

        const row = screen.getByTestId('seconds-label');
        expect(row).toHaveClass('flex', 'items-center');
        expect(row).not.toHaveClass('min-h-11');
        await fireEvent.click(screen.getByTestId('seconds-help-button'));
        expect(screen.getByTestId('seconds-help-tooltip')).toHaveTextContent('How long one unit takes.');
    });
});

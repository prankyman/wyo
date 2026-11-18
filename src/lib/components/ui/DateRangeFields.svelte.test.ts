import DateRangeFieldsTestHarness from '$test/harnesses/ui/DateRangeFieldsTestHarness.svelte';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';

/**
 * The «from – to» pair of dates: every period filter of the wyo interface is this component, so no screen tests
 * its boxes, names or refusals again.
 */
describe('DateRangeFields', () => {
    it('draws two labelled date boxes side by side in the caller’s row, with their names, limits and the first «?»', () => {
        render(DateRangeFieldsTestHarness, {
            props: {
                initialFrom: '2026-10-01',
                initialTo: '2026-10-31',
                required: true,
                min: '2026-01-01',
                max: '2026-12-31'
            }
        });
        const from = screen.getByTestId('period-from');
        const to = screen.getByTestId('period-to');

        expect(screen.getByTestId('row').children).toHaveLength(2);
        expect(document.querySelector('label[for="period-from"]')).toHaveTextContent('С');
        expect(document.querySelector('label[for="period-to"]')).toHaveTextContent('По');
        expect(from).toHaveAttribute('id', 'period-from');
        expect(to).toHaveAttribute('id', 'period-to');
        for (const [box, name, value] of [
            [from, 'from', '2026-10-01'],
            [to, 'to', '2026-10-31']
        ] as const) {
            expect(box).toHaveAttribute('type', 'date');
            expect(box).toHaveAttribute('name', name);
            expect(box).toHaveValue(value);
            expect(box).toBeRequired();
            expect(box).toHaveAttribute('min', '2026-01-01');
            expect(box).toHaveAttribute('max', '2026-12-31');
        }
        expect(screen.getAllByRole('button')).toHaveLength(1);
        expect(screen.getByTestId('row').children[0]).toHaveClass('w-40');
    });

    it('binds both days and takes the names a form needs', async () => {
        render(DateRangeFieldsTestHarness, { props: { fromName: 'dateFrom', toName: 'dateTo' } });

        await fireEvent.input(screen.getByTestId('period-from'), { target: { value: '2026-11-01' } });
        await fireEvent.input(screen.getByTestId('period-to'), { target: { value: '2026-11-07' } });
        expect(screen.getByTestId('bound')).toHaveTextContent('2026-11-01…2026-11-07');
        expect(screen.getByTestId('period-from')).toHaveAttribute('name', 'dateFrom');
        expect(screen.getByTestId('period-to')).toHaveAttribute('name', 'dateTo');
    });

    it('shows each day’s refusal under its own box and marks only that box', () => {
        render(DateRangeFieldsTestHarness, { props: { errors: { to: 'Конец раньше начала' } } });

        expect(screen.getByRole('alert')).toHaveTextContent('Конец раньше начала');
        expect(screen.getByTestId('period-to')).toHaveAttribute('aria-invalid', 'true');
        expect(screen.getByTestId('period-from')).not.toHaveAttribute('aria-invalid');
    });
});

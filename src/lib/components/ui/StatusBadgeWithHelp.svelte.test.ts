import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import StatusBadgeWithHelp from './StatusBadgeWithHelp.svelte';

/** A status badge with its «?». */
describe('StatusBadgeWithHelp', () => {
    it('draws the badge in its tone with the «?» beside it', () => {
        render(StatusBadgeWithHelp, {
            props: {
                label: 'Обрабатывается',
                tone: 'wait',
                help: 'Запись обрабатывается программой',
                size: 'md',
                testId: 'record-status'
            }
        });
        const badge = screen.getByTestId('record-status');

        expect(badge).toHaveTextContent('Обрабатывается');
        expect(badge).toHaveClass('bg-wait-50', 'text-wait-800', 'px-3');
        expect(screen.getByTestId('record-status-help-button')).toBeInTheDocument();
    });

    it('draws no «?» without a help text', () => {
        render(StatusBadgeWithHelp, { props: { label: 'Закрыт', tone: 'off', testId: 'closed-status' } });

        expect(screen.getByTestId('closed-status')).toHaveClass('bg-ink-100', 'text-ink-600');
        expect(screen.queryByRole('button')).toBeNull();
    });
});

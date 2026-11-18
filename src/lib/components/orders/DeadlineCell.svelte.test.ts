import { render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import DeadlineCell from './DeadlineCell.svelte';

beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-11-19T08:00:00'));
});
afterEach(() => vi.useRealTimers());

/** The deadline of an FBS order: its time, the attention tone 6 hours before it, the stop tone after it, said in words too. */
describe('DeadlineCell', () => {
    it.each([
        ['a day ahead', '2026-11-20T12:00:00', 'new', 'normal', 'text-ink-800', null],
        ['4 hours ahead', '2026-11-19T12:00:00', 'assembling', 'attention', 'text-warn-800', 'скоро срок'],
        ['an hour past', '2026-11-19T07:00:00', 'new', 'overdue', 'text-stop-800', 'срок прошёл']
    ] as const)('draws a deadline %s', (_case, deadlineAt, state, tone, toneClass, words) => {
        render(DeadlineCell, { props: { lang: 'ru', deadlineAt, state, testId: 'deadline' } });

        const cell = screen.getByTestId('deadline');
        expect(cell).toHaveAttribute('data-tone', tone);
        expect(cell).toHaveClass(toneClass);
        expect(cell).toHaveTextContent(deadlineAt.startsWith('2026-11-20') ? '20 нояб. 2026' : '19 нояб. 2026');
        if (words) expect(cell).toHaveTextContent(words);
        else expect(cell).not.toHaveTextContent('срок');
    });

    it.each([
        ['with no deadline, an FBO order', undefined, 'new'],
        ['already handed over, its deadline past', '2026-11-19T07:00:00', 'in_delivery'],
        ['cancelled, its deadline past', '2026-11-19T07:00:00', 'cancelled']
    ] as const)('draws a quiet dash for an order %s', (_case, deadlineAt, state) => {
        render(DeadlineCell, { props: { lang: 'ru', deadlineAt, state, testId: 'deadline' } });

        expect(screen.getByTestId('deadline')).toHaveTextContent(/^—$/);
        expect(screen.getByTestId('deadline')).toHaveAttribute('data-tone', 'none');
    });
});

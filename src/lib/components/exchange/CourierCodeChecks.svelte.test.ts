import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { courierCheckOf } from '$test/support/external-api';
import CourierCodeChecks from './CourierCodeChecks.svelte';
import type { TCourierCheck } from './types';

function draw(checks: TCourierCheck[]) {
    return render(CourierCodeChecks, { props: { lang: 'ru', checks } });
}

/** The codes a courier named for an order and what the marketplace answered to each: when, the answer, the tries left, its words. */
describe('CourierCodeChecks', () => {
    it('draws nothing while the courier has named no code, never an empty block', () => {
        const { container } = draw([]);

        expect(container.querySelectorAll('*')).toHaveLength(0);
        expect(container.textContent).toBe('');
    });

    it('says a code is still awaited, with the moment the courier named it, and nothing about tries or words', () => {
        draw([courierCheckOf()]);

        expect(screen.getByText('Проверки кода курьера')).toBeInTheDocument();
        expect(screen.getByTestId('courier-checks-help-button')).toBeInTheDocument();
        expect(screen.getByTestId('courier-checks-1-at')).toHaveTextContent('Курьер назвал код 18 нояб. 2026');
        expect(screen.getByTestId('courier-checks-1-result')).toHaveTextContent('ответа ждём');
        expect(screen.getByTestId('courier-checks-1-result')).toHaveClass('bg-wait-50');
        expect(screen.getByTestId('courier-checks-1-result-help-button')).toBeInTheDocument();
        for (const absent of ['answered', 'attempts', 'words']) {
            expect(screen.queryByTestId(`courier-checks-1-${absent}`)).toBeNull();
        }
    });

    it('draws an accepted code in the ok tone with the moment of the answer', () => {
        draw([courierCheckOf({ result: 'accepted', answeredAt: '2026-11-18T12:31:00' })]);

        expect(screen.getByTestId('courier-checks-1-result')).toHaveTextContent('код принят');
        expect(screen.getByTestId('courier-checks-1-result')).toHaveClass('bg-ok-50');
        expect(screen.getByTestId('courier-checks-1-answered')).toHaveTextContent('Маркетплейс ответил 18 нояб. 2026');
    });

    it.each([
        [1, 'Маркетплейс разрешает ещё 1 попытку.'],
        [2, 'Маркетплейс разрешает ещё 2 попытки.'],
        [5, 'Маркетплейс разрешает ещё 5 попыток.']
    ])('says how many tries a refused code leaves: %i', (attemptsLeft, text) => {
        draw([courierCheckOf({ result: 'refused', attemptsLeft, answeredAt: '2026-11-18T12:31:00' })]);

        expect(screen.getByTestId('courier-checks-1-result')).toHaveTextContent('код не принят');
        expect(screen.getByTestId('courier-checks-1-result')).toHaveClass('bg-stop-50');
        expect(screen.getByTestId('courier-checks-1-attempts')).toHaveTextContent(text);
    });

    it('says no more tries are left when the marketplace names none left, and names none when it did not say', () => {
        draw([
            courierCheckOf({ result: 'refused', attemptsLeft: 0 }),
            courierCheckOf({ at: '2026-11-18T12:40:00', result: 'refused' })
        ]);

        expect(screen.getByTestId('courier-checks-1-attempts')).toHaveTextContent('Попыток больше не осталось');
        expect(screen.queryByTestId('courier-checks-2-attempts')).toBeNull();
    });

    it('draws «нужен новый код» in the attention tone, and the marketplace’s own words as received, oldest check first', () => {
        draw([
            courierCheckOf({ result: 'refused', attemptsLeft: 1, words: 'Код не найден' }),
            courierCheckOf({
                at: '2026-11-18T12:45:00',
                result: 'new_code_needed',
                words: 'Код устарел, запросите новый'
            })
        ]);

        const rows = screen.getAllByTestId('courier-checks-row');
        expect(rows).toHaveLength(2);
        expect(rows[0]).toHaveTextContent('код не принят');
        expect(rows[1]).toHaveTextContent('нужен новый код');
        expect(screen.getByTestId('courier-checks-2-result')).toHaveClass('bg-warn-50');
        expect(screen.getByTestId('courier-checks-1-words')).toHaveTextContent('Слова маркетплейса: «Код не найден»');
        expect(screen.getByTestId('courier-checks-2-words')).toHaveTextContent(
            'Слова маркетплейса: «Код устарел, запросите новый»'
        );
    });
});

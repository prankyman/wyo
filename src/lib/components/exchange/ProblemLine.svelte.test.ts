import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { exchangeProblemOf } from '$test/support/external-api';
import ProblemLine from './ProblemLine.svelte';
import type { TProblem } from './types';

function draw(problem: TProblem): void {
    render(ProblemLine, { props: { lang: 'ru', problem, testId: 'problem' } });
}

/** A problem in wyo's sentence and, beside it, the marketplace's or the fulfilment centre's own words and code as received. */
describe('ProblemLine', () => {
    it('words a refusal of the marketplace with the step named, and shows its words and its code beside the sentence', () => {
        draw(exchangeProblemOf());

        expect(screen.getByTestId('problem')).toHaveTextContent(
            'Маркетплейс отказал на шаге «взятие заказа в сборку».'
        );
        expect(screen.getByTestId('problem-words')).toHaveTextContent(
            'Слова маркетплейса: «Не удалось добавить заказ в поставку» (код FailedToAddSupplyOrder)'
        );
        expect(screen.getByTestId('problem-help-button')).toBeInTheDocument();
    });

    it('attributes the words of a refusal of the fulfilment centre to the fulfilment centre', () => {
        draw(
            exchangeProblemOf({
                code: 'fc_refused',
                params: { step: 'labels' },
                marketplaceCode: undefined,
                words: 'Заказ закрыт'
            })
        );

        expect(screen.getByTestId('problem')).toHaveTextContent(
            'Фулфилмент-центр отказал на шаге «получение этикетки».'
        );
        expect(screen.getByTestId('problem-words')).toHaveTextContent('Слова фулфилмент-центра: «Заказ закрыт»');
        expect(screen.getByTestId('problem-words')).not.toHaveTextContent('код');
    });

    it.each([
        [
            { code: 'marking_code_missing' },
            'Маркетплейс требует код маркировки, а фулфилмент-центр не прислал ни одного.'
        ],
        [{ code: 'codes_refused' }, 'Маркетплейс не принял код маркировки.'],
        [
            { code: 'identifier_unsupported', params: { identifier: 'imei' } },
            'требует не только код маркировки, но и «Номер IMEI'
        ],
        [
            { code: 'label_changed', params: { orders: ['4455667788', '4455667799'] } },
            'изменилась у заказов: 4455667788, 4455667799'
        ],
        [{ code: 'shipping_point_needed' }, 'Не выбран пункт сдачи'],
        [{ code: 'shipping_point_gone' }, 'больше не предлагает'],
        [{ code: 'no_orders' }, 'не осталось заказов'],
        [{ code: 'settings_missing' }, 'Не заполнены настройки передачи'],
        [{ code: 'something_new' }, 'кодом «something_new», которого wyo не знает']
    ] as const)('words the problem %j', (given, text) => {
        draw({ ...given, at: '2026-11-18T12:00:00' } as TProblem);

        expect(screen.getByTestId('problem')).toHaveTextContent(text);
        expect(screen.queryByTestId('problem-words')).toBeNull();
    });
});

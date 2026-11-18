import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import HeldReasons from './HeldReasons.svelte';
import type { THeldReason } from './types';

function draw(reasons: THeldReason[]): HTMLElement[] {
    render(HeldReasons, { props: { lang: 'ru', reasons, dataTest: 'held' } });
    return screen.queryAllByTestId('held-reason');
}

/** The fulfilment centre's reasons for holding an order, each in a sentence of its own, with the values it named. */
describe('HeldReasons', () => {
    it.each([
        [
            { code: 'unknown_product', params: { barcode: '2040000000017' } },
            'Товар со штрихкодом 2040000000017 не найден'
        ],
        [{ code: 'unknown_product', params: { article: 'DRESS-RED-42' } }, 'Товар с артикулом DRESS-RED-42 не найден'],
        [
            { code: 'unknown_product', params: { marketplace_item_id: '987654' } },
            'Товар с номером 987654 на маркетплейсе не найден'
        ],
        [{ code: 'unknown_product' }, 'Товар не найден у фулфилмент-центра.'],
        [
            { code: 'line_conflict', params: { article: 'DRESS-RED-42', barcode: '2040000000017' } },
            'артикулом DRESS-RED-42 и штрихкодом 2040000000017'
        ],
        [{ code: 'unknown_warehouse', params: { warehouse: 'Лесная' } }, 'Склад «Лесная» не найден'],
        [{ code: 'warehouse_missing' }, 'В заказе не указан склад.'],
        [{ code: 'work_stopped' }, 'приостановлена'],
        [{ code: 'something_new' }, 'кодом «something_new», которого wyo не знает']
    ] as const)('words the reason %j', (reason, text) => {
        const lines = draw([reason as THeldReason]);

        expect(lines).toHaveLength(1);
        expect(lines[0]).toHaveTextContent(text);
    });

    it('draws one line for each reason, in the order the fulfilment centre gave them, and nothing for none', () => {
        const lines = draw([{ code: 'warehouse_missing' }, { code: 'work_stopped' }]);

        expect(lines.map((line) => line.textContent)).toEqual([
            'В заказе не указан склад.',
            'Работа этого продавца у фулфилмент-центра приостановлена.'
        ]);
    });
});

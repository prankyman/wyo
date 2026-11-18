import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { warehouseMatchesOf } from '$test/support/external-api';
import WarehouseMatchHarness from '$test/harnesses/external-api/WarehouseMatchHarness.svelte';

function draw(change: Partial<ReturnType<typeof warehouseMatchesOf>> = {}, errors: (string | undefined)[] = []): void {
    const matches = warehouseMatchesOf(change);
    render(WarehouseMatchHarness, {
        props: {
            rows: matches.rows,
            fcWarehouses: matches.fcWarehouses,
            start: matches.rows.map((row) => row.fcWarehouse ?? ''),
            errors
        }
    });
}

/** Each warehouse of the seller beside a choice of the fulfilment centre's warehouses; an unmatched row says its orders are not sent and why. */
describe('WarehouseMatchTable', () => {
    it('draws a row for each warehouse of the seller with the fulfilment centre’s warehouses to choose from, the matched one chosen', () => {
        draw();

        const rows = screen.getAllByTestId('warehouses-row');
        expect(rows).toHaveLength(2);
        expect(rows[0]).toHaveTextContent('Склад на Ленина');
        expect(rows[0]).toHaveTextContent('Магазин одежды');
        const box = within(rows[0]).getByTestId('warehouses-0-box');
        expect(
            within(box)
                .getAllByRole('option')
                .map((option) => option.textContent)
        ).toEqual(['Не передавать заказы этого склада', 'Москва, Лесная', 'Санкт-Петербург, Невский']);
        expect(box).toHaveValue('MSK-1');
        expect(screen.getByTestId('warehouses-1-box')).toHaveValue('');
    });

    it('says an unmatched warehouse’s orders are not sent and why, and stops saying it when a warehouse is chosen', async () => {
        draw();

        expect(screen.queryByTestId('warehouses-0-unmatched')).toBeNull();
        expect(screen.getByTestId('warehouses-1-unmatched')).toHaveTextContent(
            'Заказы этого склада не передаются фулфилмент-центру'
        );

        await fireEvent.change(screen.getByTestId('warehouses-1-box'), { target: { value: 'SPB-1' } });

        expect(screen.queryByTestId('warehouses-1-unmatched')).toBeNull();
    });

    it('says the fulfilment centre gave no warehouses when its list is empty', () => {
        draw({ fcWarehouses: [] });

        expect(screen.getByTestId('warehouses-no-fc')).toHaveTextContent('Фулфилмент-центр не назвал ни одного склада');
    });

    it('lays a refusal onto the row it is about', () => {
        draw({}, [undefined, 'Выбранного склада у фулфилмент-центра нет.']);

        expect(screen.getByTestId('warehouses-1-field')).toHaveTextContent(
            'Выбранного склада у фулфилмент-центра нет.'
        );
        expect(screen.getByTestId('warehouses-0-field')).not.toHaveTextContent('Выбранного склада');
    });
});

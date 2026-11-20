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

    it('warns in the row of a warehouse with automatic assembly, and in no other row, while still letting the owner match it', () => {
        draw({
            rows: warehouseMatchesOf().rows.map((row, index) => ({ ...row, autoAssembly: index === 1 }))
        });

        expect(screen.queryByTestId('warehouses-0-auto-assembly')).toBeNull();
        expect(screen.getByTestId('warehouses-1-auto-assembly')).toHaveTextContent(
            'На этом складе включена автоматическая сборка'
        );
        expect(screen.getByTestId('warehouses-1-box')).toBeEnabled();
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

    describe('the label size and Express', () => {
        const FC_WAREHOUSES = [
            {
                code: 'MSK-1',
                name: 'Москва, Лесная',
                labelSizes: [
                    { marketplace: 'mp-a', widthMm: 58, heightMm: 40 },
                    { marketplace: 'mp-b', widthMm: 40, heightMm: 30 }
                ]
            },
            { code: 'SPB-1', name: 'Санкт-Петербург, Невский' }
        ];

        function drawSized(): void {
            const matches = warehouseMatchesOf({ fcWarehouses: FC_WAREHOUSES });
            draw({
                fcWarehouses: FC_WAREHOUSES,
                rows: matches.rows.map((row, index) => ({ ...row, fcMarketplace: 'mp-a', express: index === 0 }))
            });
        }

        it('says which label size the matched warehouse names for the account’s marketplace, and the marketplace’s own size when it names none', async () => {
            drawSized();

            expect(screen.getByTestId('warehouses-0-label-size')).toHaveTextContent(
                'Размер этикетки: 58 × 40 мм — задаёт фулфилмент-центр'
            );
            expect(screen.getByTestId('warehouses-0-label-size-help-button')).toBeInTheDocument();
            expect(screen.queryByTestId('warehouses-1-label-size')).toBeNull();

            await fireEvent.change(screen.getByTestId('warehouses-1-box'), { target: { value: 'SPB-1' } });

            expect(screen.getByTestId('warehouses-1-label-size')).toHaveTextContent(
                'Размер этикетки: размер маркетплейса'
            );
        });

        it('follows the warehouse the owner chooses', async () => {
            drawSized();

            await fireEvent.change(screen.getByTestId('warehouses-0-box'), { target: { value: 'SPB-1' } });

            expect(screen.getByTestId('warehouses-0-label-size')).toHaveTextContent('размер маркетплейса');
            expect(screen.getByTestId('warehouses-0-label-size')).not.toHaveTextContent('58 × 40');
        });

        it('tells in the row of an Express warehouse that a courier waits for the code check and that notifications bring the code at once', () => {
            drawSized();

            expect(screen.getByTestId('warehouses-0-express')).toHaveTextContent('курьер');
            expect(screen.getByTestId('warehouses-0-express')).toHaveTextContent('до минуты');
            expect(screen.queryByTestId('warehouses-1-express')).toBeNull();
        });
    });
});

import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import WarehouseCell from './WarehouseCell.svelte';

/** The warehouse of an order in the list: its name, or — for an order the seller packs — the words that the warehouse is not known. */
describe('WarehouseCell', () => {
    it('draws the warehouse’s name when the order has one', () => {
        render(WarehouseCell, {
            props: { lang: 'ru', scheme: 'fbs', warehouse: 'Склад на Ленина', testId: 'warehouse' }
        });

        expect(screen.getByTestId('warehouse')).toHaveTextContent('Склад на Ленина');
        expect(screen.queryByTestId('warehouse-help-button')).toBeNull();
    });

    it.each(['fbs', 'express'] as const)(
        'says «Склад не определён» with a «?» for an %s order the marketplace gave no warehouse',
        (scheme) => {
            render(WarehouseCell, { props: { lang: 'ru', scheme, testId: 'warehouse' } });

            expect(screen.getByTestId('warehouse')).toHaveTextContent('Склад не определён');
            expect(screen.getByTestId('warehouse-help-button').getAttribute('aria-label')).toContain(
                'в магазине несколько складов'
            );
        }
    );

    it('shows the marketplace’s own text of the warehouse beside «Склад не определён», with a «?» that says why it is kept', () => {
        render(WarehouseCell, {
            props: { lang: 'ru', scheme: 'fbs', warehouseText: 'Склад Подольск-2', testId: 'warehouse' }
        });

        expect(screen.getByTestId('warehouse')).toHaveTextContent('Склад не определён');
        expect(screen.getByTestId('warehouse-text')).toHaveTextContent('Склад Подольск-2');
        expect(screen.getByTestId('warehouse-text-help-button').getAttribute('aria-label')).toContain(
            'собственное название склада'
        );
    });

    it('draws no such text when the order has a warehouse of its own or none was given', () => {
        const { unmount } = render(WarehouseCell, {
            props: {
                lang: 'ru',
                scheme: 'fbs',
                warehouse: 'Склад на Ленина',
                warehouseText: 'Подольск-2',
                testId: 'warehouse'
            }
        });
        expect(screen.queryByTestId('warehouse-text')).toBeNull();
        unmount();

        render(WarehouseCell, { props: { lang: 'ru', scheme: 'fbs', testId: 'warehouse' } });
        expect(screen.queryByTestId('warehouse-text')).toBeNull();
    });

    it.each(['fbo', 'fby'] as const)(
        'draws a quiet dash for an %s order, which has no warehouse of the seller',
        (scheme) => {
            render(WarehouseCell, { props: { lang: 'ru', scheme, testId: 'warehouse' } });

            expect(screen.getByTestId('warehouse')).toHaveTextContent('—');
            expect(screen.queryByTestId('warehouse-help-button')).toBeNull();
        }
    );
});

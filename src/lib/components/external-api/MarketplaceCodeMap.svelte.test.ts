import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { marketplaceMatchesOf } from '$test/support/external-api';
import MarketplaceMapHarness from '$test/harnesses/external-api/MarketplaceMapHarness.svelte';

function draw(
    start: string[],
    change: Partial<ReturnType<typeof marketplaceMatchesOf>> = {},
    errors: (string | undefined)[] = []
): void {
    const matches = marketplaceMatchesOf(change);
    render(MarketplaceMapHarness, {
        props: { rows: matches.rows, fcMarketplaces: matches.fcMarketplaces, start, errors }
    });
}

/** Each account beside the marketplace's code in the fulfilment centre's own list, the proposed code named, a key that may only read warned about. */
describe('MarketplaceCodeMap', () => {
    it('draws a row for each account with the fulfilment centre’s marketplaces to choose from, the chosen code selected', () => {
        draw(['mp-a', 'mp-b']);

        const rows = screen.getAllByTestId('codes-row');
        expect(rows).toHaveLength(2);
        expect(rows[0]).toHaveTextContent('Магазин одежды');
        const box = within(rows[0]).getByTestId('codes-0-box');
        expect(
            within(box)
                .getAllByRole('option')
                .map((option) => option.textContent)
        ).toEqual(['Не передавать заказы этого аккаунта', 'Маркетплейс А', 'Маркетплейс Б']);
        expect(box).toHaveValue('mp-a');
        expect(screen.getByTestId('codes-1-box')).toHaveValue('mp-b');
    });

    it('says which code wyo proposes while the proposed one is the chosen one and none is saved yet', async () => {
        draw(['mp-a', 'mp-b']);

        expect(screen.getByTestId('codes-0-proposed')).toHaveTextContent('wyo предлагает «Маркетплейс А»');
        expect(screen.queryByTestId('codes-1-proposed')).toBeNull();

        await fireEvent.change(screen.getByTestId('codes-0-box'), { target: { value: 'mp-b' } });

        expect(screen.queryByTestId('codes-0-proposed')).toBeNull();
    });

    it('says the orders of an account with no code are not sent and why', () => {
        draw(['', 'mp-b']);

        expect(screen.getByTestId('codes-0-unmatched')).toHaveTextContent(
            'Заказы этого аккаунта не передаются фулфилмент-центру'
        );
        expect(screen.queryByTestId('codes-1-unmatched')).toBeNull();
    });

    it('warns an account whose key may only read, with the remedy, and does not warn the others', () => {
        draw(['mp-a', 'mp-b']);

        expect(screen.queryByTestId('codes-0-read-only')).toBeNull();
        expect(screen.getByTestId('codes-1-read-only')).toHaveTextContent(
            'Ключ аккаунта «Второй магазин» только читает'
        );
        expect(screen.getByTestId('codes-1-read-only')).toHaveTextContent('замените его в разделе «Маркетплейсы»');
    });

    it('lays a refusal onto the row it is about', () => {
        draw(['mp-a', 'mp-b'], {}, [undefined, 'Такого маркетплейса у фулфилмент-центра нет.']);

        expect(screen.getByTestId('codes-1-field')).toHaveTextContent('Такого маркетплейса у фулфилмент-центра нет.');
    });
});

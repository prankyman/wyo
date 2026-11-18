import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { MARKETPLACE } from '$test/support/marketplaces';
import AdapterSettingsHarness from '$test/harnesses/external-api/AdapterSettingsHarness.svelte';
import type { TMarketplace } from '$lib/components/marketplaces/types';

function draw(
    start: Record<string, string>,
    marketplace: TMarketplace = MARKETPLACE,
    errors: Record<string, string | undefined> = {}
): void {
    render(AdapterSettingsHarness, { props: { marketplace, start, errors } });
}

const values = () => JSON.parse(screen.getByTestId('adapter-values').textContent ?? '{}') as Record<string, string>;

/** The hand-over settings an adapter declares, drawn from what it declares: a drop-down for a field with choices, a box for the rest. */
describe('AdapterSettingsForm', () => {
    it('draws a drop-down for each declared setting with choices, worded by the adapter’s own texts, and a text box for one without', () => {
        draw({ shipping_type: 'selfShipping', city: 'Москва', office_type: 'sc' });

        const shipping = screen.getByTestId('adapter-shipping_type-box');
        expect(
            within(shipping)
                .getAllByRole('option')
                .map((option) => option.textContent)
        ).toEqual(['Не выбрано', 'Доставка силами продавца', 'Доставка через транспортную компанию']);
        expect(shipping).toHaveValue('selfShipping');
        expect(screen.getByTestId('adapter-city-box')).toHaveValue('Москва');
        expect(screen.getByTestId('adapter-city-box').tagName).toBe('INPUT');
        expect(screen.getByTestId('adapter-office_type-field')).toHaveTextContent('Вид пункта сдачи');
        expect(screen.getByTestId('adapter-shipping_type-field')).toHaveTextContent('Как поставка попадёт');
    });

    it('writes what is chosen or typed into the values under the setting’s name', async () => {
        draw({});

        await fireEvent.change(screen.getByTestId('adapter-office_type-box'), { target: { value: 'pp' } });
        await fireEvent.input(screen.getByTestId('adapter-city-box'), { target: { value: 'Казань' } });

        expect(values()).toEqual({ office_type: 'pp', city: 'Казань' });
    });

    it('draws nothing for an adapter that declares no hand-over setting', () => {
        draw({}, { ...MARKETPLACE, handoverSettings: [] });

        expect(screen.queryByTestId('adapter-city-field')).toBeNull();
    });

    it('lays a refusal onto the setting it is about', () => {
        draw({}, MARKETPLACE, { city: 'Это поле нужно заполнить.' });

        expect(screen.getByTestId('adapter-city-field')).toHaveTextContent('Это поле нужно заполнить.');
    });
});

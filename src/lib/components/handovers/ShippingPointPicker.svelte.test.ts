import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { handoverDetailsOf } from '$test/support/external-api';
import { sendForm } from '$test/support/superform-answer';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn(), put: vi.fn() }
}));

import { api } from '$lib/api/api';
import ShippingPointPicker from './ShippingPointPicker.svelte';

const get = vi.mocked(api.get);
const put = vi.mocked(api.put);
const POINTS = {
    points: [
        { id: 'p1', name: 'Склад Коледино', address: 'Подольск, Троицкая, 10' },
        { id: 'p2', name: 'Склад Электросталь' }
    ],
    chosen: 'p2'
};

afterEach(() => {
    get.mockReset();
    put.mockReset();
});

function draw(onSaved = vi.fn()): void {
    render(ShippingPointPicker, { props: { lang: 'ru', handoverId: 31, onSaved } });
}

/** The points the marketplace offers for a hand-over, the remembered one marked and chosen, and the saving of the choice. */
describe('ShippingPointPicker', () => {
    it('reads the points of the hand-over, names each with its address, marks the remembered one and chooses it', async () => {
        get.mockResolvedValue({ ok: true, data: POINTS });
        draw();

        const box = await screen.findByTestId('shipping-point-box');
        expect(get).toHaveBeenCalledExactlyOnceWith('/api/v1/handovers/31/shipping-point');
        const options = within(box).getAllByRole('option');
        expect(options.map((option) => option.textContent)).toEqual([
            'Склад Коледино — Подольск, Троицкая, 10',
            'Склад Электросталь (запомнен)'
        ]);
        expect(box).toHaveValue('p2');
    });

    it('saves the chosen point for the account, tells the page the hand-over as it now stands and says so', async () => {
        get.mockResolvedValue({ ok: true, data: POINTS });
        const saved = handoverDetailsOf({ needsShippingPoint: false });
        put.mockResolvedValue({ ok: true, data: saved });
        const onSaved = vi.fn();
        draw(onSaved);
        await screen.findByTestId('shipping-point-box');

        await fireEvent.change(screen.getByTestId('shipping-point-box'), { target: { value: 'p1' } });
        await sendForm(screen.getByTestId('shipping-point'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(saved));
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/handovers/31/shipping-point', { point: 'p1' });
        expect(screen.getByTestId('shipping-point-answer-saved')).toHaveTextContent('Пункт запомнен');
    });

    it('words the marketplace’s refusal of a point it no longer offers under the button', async () => {
        get.mockResolvedValue({ ok: true, data: POINTS });
        put.mockResolvedValue({ ok: false, status: 422, refusal: { code: 'shipping_point_unknown' } });
        draw();
        await screen.findByTestId('shipping-point-box');

        await sendForm(screen.getByTestId('shipping-point'));

        await waitFor(() =>
            expect(screen.getByTestId('shipping-point-answer-error')).toHaveTextContent('больше не предлагает')
        );
    });

    it('says the points cannot be asked yet when the batch is not made, and asks again on «Повторить»', async () => {
        get.mockResolvedValueOnce({ ok: false, status: 409, refusal: { code: 'shipping_points_unavailable' } });
        draw();

        expect(await screen.findByTestId('shipping-point-refusal')).toHaveTextContent('Список пунктов пока недоступен');
        get.mockResolvedValue({ ok: true, data: POINTS });
        await fireEvent.click(screen.getByTestId('shipping-point-retry'));

        expect(await screen.findByTestId('shipping-point-box')).toBeInTheDocument();
        expect(screen.queryByTestId('shipping-point-refusal')).toBeNull();
    });

    it('says when the marketplace offers no point at all', async () => {
        get.mockResolvedValue({ ok: true, data: { points: [] } });
        draw();

        expect(await screen.findByTestId('shipping-point-empty')).toHaveTextContent('не предложил ни одного пункта');
        expect(screen.queryByTestId('shipping-point')).toBeNull();
    });
});

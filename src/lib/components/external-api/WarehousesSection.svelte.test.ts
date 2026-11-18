import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { warehouseMatchesOf } from '$test/support/external-api';
import { flushTicks, sendForm } from '$test/support/superform-answer';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { put: vi.fn() }
}));

import { api } from '$lib/api/api';
import WarehousesSection from './WarehousesSection.svelte';

const put = vi.mocked(api.put);

afterEach(() => {
    put.mockReset();
});

function draw(matches = warehouseMatchesOf(), onSaved = vi.fn()): typeof onSaved {
    render(WarehousesSection, { props: { lang: 'ru', matches, onSaved } });
    return onSaved;
}

async function openSection(): Promise<HTMLDetailsElement> {
    const section = screen.getByTestId('external-api-warehouses') as HTMLDetailsElement;
    section.open = true;
    await fireEvent(section, new Event('toggle'));
    await flushTicks();
    return section;
}

/** «Склады»: each warehouse of the seller beside the fulfilment centre's warehouse that stands for it, saved as one form. */
describe('WarehousesSection', () => {
    it('draws a folded section that starts with what it is for and holds the table of matches', async () => {
        draw();

        expect((screen.getByTestId('external-api-warehouses') as HTMLDetailsElement).open).toBe(false);
        await openSection();
        expect(screen.getByTestId('external-api-warehouses-intro')).toHaveTextContent('Склад фулфилмент-центра');
        expect(screen.getAllByTestId('warehouses-row')).toHaveLength(2);
        expect(screen.getByTestId('warehouses-0-box')).toHaveValue('MSK-1');
        expect(screen.queryByTestId('external-api-warehouses-save')).toBeNull();
    });

    it('saves the changed matches, a warehouse left unmatched sent without a warehouse, tells the page the answer and folds', async () => {
        const saved = warehouseMatchesOf();
        put.mockResolvedValue({ ok: true, data: saved });
        const onSaved = draw();
        const section = await openSection();

        await fireEvent.change(screen.getByTestId('warehouses-0-box'), { target: { value: '' } });
        await fireEvent.change(screen.getByTestId('warehouses-1-box'), { target: { value: 'SPB-1' } });
        await sendForm(screen.getByTestId('external-api-warehouses-form'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(saved));
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api/warehouses', {
            rows: [
                { accountId: 7, warehouseNumber: '1001' },
                { accountId: 7, warehouseNumber: '1002', fcWarehouse: 'SPB-1' }
            ]
        });
        expect(section.open).toBe(false);
        expect(screen.getByTestId('external-api-warehouses-saved')).toHaveTextContent('Сохранено.');
    });

    it('lays the wyo server’s refusal of one row onto that row’s box', async () => {
        put.mockResolvedValue({
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'rows.1.fcWarehouse', code: 'unknown_warehouse' }] }
        });
        draw();
        await openSection();
        await fireEvent.change(screen.getByTestId('warehouses-1-box'), { target: { value: 'SPB-1' } });

        await sendForm(screen.getByTestId('external-api-warehouses-form'));

        await waitFor(() => expect(screen.getByTestId('warehouses-1-field')).toHaveTextContent('Значение не подходит'));
        expect(screen.getByTestId('warehouses-0-field')).not.toHaveTextContent('Значение не подходит');
    });

    it('says there is nothing to match when the seller has no warehouse yet', async () => {
        draw(warehouseMatchesOf({ rows: [] }));
        await openSection();

        expect(screen.getByTestId('warehouses-empty')).toHaveTextContent(
            'wyo ещё не прочитала ни одного вашего склада'
        );
        expect(screen.queryByTestId('warehouses-row')).toBeNull();
    });
});

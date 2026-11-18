import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { marketplaceMatchesOf } from '$test/support/external-api';
import { flushTicks, sendForm } from '$test/support/superform-answer';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { put: vi.fn() }
}));

import { api } from '$lib/api/api';
import MarketplacesSection from './MarketplacesSection.svelte';

const put = vi.mocked(api.put);

afterEach(() => {
    put.mockReset();
});

function draw(matches = marketplaceMatchesOf(), onSaved = vi.fn()): typeof onSaved {
    render(MarketplacesSection, { props: { lang: 'ru', matches, onSaved } });
    return onSaved;
}

async function openSection(): Promise<HTMLDetailsElement> {
    const section = screen.getByTestId('external-api-marketplaces') as HTMLDetailsElement;
    section.open = true;
    await fireEvent(section, new Event('toggle'));
    await flushTicks();
    return section;
}

/** «Маркетплейсы»: each account beside the marketplace's code in the fulfilment centre's list, the proposed code already chosen, saved as one form. */
describe('MarketplacesSection', () => {
    it('draws a folded section that starts with what it is for, with the saved code and the proposed code already chosen', async () => {
        draw();

        expect((screen.getByTestId('external-api-marketplaces') as HTMLDetailsElement).open).toBe(false);
        await openSection();
        expect(screen.getByTestId('external-api-marketplaces-intro')).toHaveTextContent(
            'Выберите для каждого своего аккаунта'
        );
        expect(screen.getByTestId('codes-0-box')).toHaveValue('mp-a');
        expect(screen.getByTestId('codes-1-box')).toHaveValue('mp-b');
    });

    it('offers «Сохранить» at once when a proposed code is chosen that is not saved yet, so the owner can accept it with one press', async () => {
        draw();
        await openSection();

        expect(screen.getByTestId('external-api-marketplaces-save')).toBeInTheDocument();
    });

    it('offers no «Сохранить» while every code is saved and nothing is proposed', async () => {
        draw(
            marketplaceMatchesOf({
                rows: marketplaceMatchesOf().rows.map((row) => ({
                    ...row,
                    fcMarketplace: row.fcMarketplace ?? 'mp-a',
                    proposed: undefined
                }))
            })
        );
        await openSection();

        expect(screen.queryByTestId('external-api-marketplaces-save')).toBeNull();
    });

    it('saves the code of each account without touching its hand-over settings, tells the page the answer and folds', async () => {
        const saved = marketplaceMatchesOf();
        put.mockResolvedValue({ ok: true, data: saved });
        const onSaved = draw();
        const section = await openSection();

        await fireEvent.change(screen.getByTestId('codes-1-box'), { target: { value: '' } });
        await sendForm(screen.getByTestId('external-api-marketplaces-form'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(saved));
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api/marketplaces', {
            rows: [
                { accountId: 7, fcMarketplace: 'mp-a' },
                { accountId: 8, fcMarketplace: '' }
            ]
        });
        expect(section.open).toBe(false);
        expect(screen.getByTestId('external-api-marketplaces-saved')).toHaveTextContent('Сохранено.');
    });

    it('lays the wyo server’s refusal of one row onto that row’s box', async () => {
        put.mockResolvedValue({
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'rows.1.fcMarketplace', code: 'unknown_marketplace' }] }
        });
        draw();
        await openSection();

        await sendForm(screen.getByTestId('external-api-marketplaces-form'));

        await waitFor(() => expect(screen.getByTestId('codes-1-field')).toHaveTextContent('Значение не подходит'));
    });

    it('warns an account whose key may only read, and says there is nothing to match without an account', async () => {
        draw();
        await openSection();
        expect(screen.getByTestId('codes-1-read-only')).toBeInTheDocument();
    });

    it('says an account must be added first when there is none', async () => {
        draw(marketplaceMatchesOf({ rows: [] }));
        await openSection();

        expect(screen.getByTestId('codes-empty')).toHaveTextContent('Аккаунтов ещё нет');
    });
});

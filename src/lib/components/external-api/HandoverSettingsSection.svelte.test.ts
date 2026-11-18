import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { marketplaceMatchesOf } from '$test/support/external-api';
import { MARKETPLACE } from '$test/support/marketplaces';
import { flushTicks, sendForm } from '$test/support/superform-answer';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { put: vi.fn() }
}));

import { api } from '$lib/api/api';
import HandoverSettingsSection from './HandoverSettingsSection.svelte';

const put = vi.mocked(api.put);

afterEach(() => {
    put.mockReset();
});

function draw(matches = marketplaceMatchesOf(), marketplaces = [MARKETPLACE], onSaved = vi.fn()): typeof onSaved {
    render(HandoverSettingsSection, { props: { lang: 'ru', matches, marketplaces, onSaved } });
    return onSaved;
}

async function openSection(): Promise<HTMLDetailsElement> {
    const section = screen.getByTestId('external-api-handover') as HTMLDetailsElement;
    section.open = true;
    await fireEvent(section, new Event('toggle'));
    await flushTicks();
    return section;
}

/** «Передача заказов маркетплейсу»: the adapters’ hand-over settings of each account, drawn from what each adapter declares. */
describe('HandoverSettingsSection', () => {
    it('draws a folded section that starts with what it is for, and for each account the settings its adapter declares, filled as saved', async () => {
        draw();

        expect((screen.getByTestId('external-api-handover') as HTMLDetailsElement).open).toBe(false);
        await openSection();
        expect(screen.getByTestId('external-api-handover-intro')).toHaveTextContent(
            'wyo закрывает на маркетплейсе партию'
        );
        expect(screen.getAllByTestId('handover-account')).toHaveLength(2);
        expect(screen.getByTestId('handover-account-title-0')).toHaveTextContent('Аккаунт «Магазин одежды»');
        expect(screen.getByTestId('handover-0-city-box')).toHaveValue('Москва');
        expect(screen.getByTestId('handover-0-shipping_type-box')).toHaveValue('selfShipping');
        expect(screen.getByTestId('handover-1-city-box')).toHaveValue('');
    });

    it('saves the settings of every account, tells the page the answer and folds', async () => {
        const saved = marketplaceMatchesOf();
        put.mockResolvedValue({ ok: true, data: saved });
        const onSaved = draw();
        const section = await openSection();

        await fireEvent.input(screen.getByTestId('handover-1-city-box'), { target: { value: 'Казань' } });
        await fireEvent.change(screen.getByTestId('handover-1-office_type-box'), { target: { value: 'sw' } });
        await sendForm(screen.getByTestId('external-api-handover-form'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(saved));
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api/marketplaces', {
            rows: [
                {
                    accountId: 7,
                    handoverSettings: { shipping_type: 'selfShipping', city: 'Москва', office_type: 'sc' }
                },
                { accountId: 8, handoverSettings: { city: 'Казань', office_type: 'sw' } }
            ]
        });
        expect(section.open).toBe(false);
        expect(screen.getByTestId('external-api-handover-saved')).toHaveTextContent('Сохранено.');
    });

    it('lays the wyo server’s refusal of one setting of one account onto its box', async () => {
        put.mockResolvedValue({
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'rows.1.handoverSettings.city', code: 'required' }] }
        });
        draw();
        await openSection();
        await fireEvent.input(screen.getByTestId('handover-1-city-box'), { target: { value: 'К' } });

        await sendForm(screen.getByTestId('external-api-handover-form'));

        await waitFor(() =>
            expect(screen.getByTestId('handover-1-city-field')).toHaveTextContent('Это поле нужно заполнить.')
        );
        expect(screen.getByTestId('handover-0-city-field')).not.toHaveTextContent('Это поле нужно заполнить.');
    });

    it('says there is nothing to fill when no adapter declares a setting, and that an account must be added first when there is none', async () => {
        const { unmount } = render(HandoverSettingsSection, {
            props: {
                lang: 'ru',
                matches: marketplaceMatchesOf(),
                marketplaces: [{ ...MARKETPLACE, handoverSettings: [] }],
                onSaved: vi.fn()
            }
        });
        await openSection();
        expect(screen.getByTestId('handover-none')).toHaveTextContent('нет настроек передачи заказов');
        unmount();

        draw(marketplaceMatchesOf({ rows: [] }));
        await openSection();
        expect(screen.getByTestId('handover-none')).toHaveTextContent('Аккаунтов ещё нет');
    });
});

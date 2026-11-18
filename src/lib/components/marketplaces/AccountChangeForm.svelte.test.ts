import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountOf } from '$test/support/marketplaces';
import { sendForm } from '$test/support/superform-answer';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { patch: vi.fn() }
}));

import { api } from '$lib/api/api';
import AccountChangeForm from './AccountChangeForm.svelte';

const patch = vi.mocked(api.patch);

afterEach(() => patch.mockReset());

/** «Изменить» on an account's card: its name and its hours to hand an order over. */
describe('AccountChangeForm', () => {
    it('starts from the stored values, sends the hours as a number, keeps the values and tells the card', async () => {
        const changed = accountOf({ name: 'Одежда', deadlineHours: 36 });
        patch.mockResolvedValue({ ok: true, data: changed });
        const onSaved = vi.fn();
        render(AccountChangeForm, { props: { lang: 'ru', account: accountOf(), id: 'account-7-change', onSaved } });

        expect(screen.getByTestId('account-7-change-name')).toHaveValue('Магазин одежды');
        expect(screen.getByTestId('account-7-change-hours')).toHaveValue('48');
        expect(screen.getByTestId('account-7-change-hours-field')).toHaveTextContent('Срок передачи заказа, часов');
        await fireEvent.input(screen.getByTestId('account-7-change-name'), { target: { value: 'Одежда' } });
        await fireEvent.input(screen.getByTestId('account-7-change-hours'), { target: { value: '36' } });
        await sendForm(screen.getByTestId('account-7-change'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(changed));
        expect(patch).toHaveBeenCalledExactlyOnceWith('/api/v1/accounts/7', { name: 'Одежда', deadlineHours: 36 });
        expect(screen.getByTestId('account-7-change-hours')).toHaveValue('36');
    });

    it('refuses hours outside the bounds in the browser, before anything is sent', async () => {
        render(AccountChangeForm, {
            props: { lang: 'ru', account: accountOf(), id: 'account-7-change', onSaved: vi.fn() }
        });

        await fireEvent.input(screen.getByTestId('account-7-change-hours'), { target: { value: '0' } });
        await sendForm(screen.getByTestId('account-7-change'));

        await waitFor(() =>
            expect(screen.getByTestId('account-7-change-hours-field')).toHaveTextContent('Число вне допустимых границ')
        );
        expect(patch).not.toHaveBeenCalled();
    });

    it('asks only the name when the marketplace gives each order its own deadline, and sends only the name', async () => {
        const changed = accountOf({ name: 'Одежда' });
        patch.mockResolvedValue({ ok: true, data: changed });
        const onSaved = vi.fn();
        render(AccountChangeForm, {
            props: { lang: 'ru', account: accountOf(), givesDeadline: true, id: 'account-7-change', onSaved }
        });

        expect(screen.queryByTestId('account-7-change-hours')).toBeNull();
        await fireEvent.input(screen.getByTestId('account-7-change-name'), { target: { value: 'Одежда' } });
        await sendForm(screen.getByTestId('account-7-change'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(changed));
        expect(patch).toHaveBeenCalledExactlyOnceWith('/api/v1/accounts/7', { name: 'Одежда' });
    });
});

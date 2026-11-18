import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { eventsOf } from '$test/support/external-api';
import { flushTicks, sendForm } from '$test/support/superform-answer';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { put: vi.fn() }
}));

import { api } from '$lib/api/api';
import EventsModeForm from './EventsModeForm.svelte';
import type { TEventsSetting } from './types';

const put = vi.mocked(api.put);
const SECRET = 'whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw';

afterEach(() => {
    put.mockReset();
});

function draw(events: TEventsSetting = eventsOf(), onSaved = vi.fn()): void {
    render(EventsModeForm, { props: { lang: 'ru', events, onSaved } });
}

async function openSection(): Promise<HTMLDetailsElement> {
    const section = screen.getByTestId('external-api-events') as HTMLDetailsElement;
    section.open = true;
    await fireEvent(section, new Event('toggle'));
    await flushTicks();
    return section;
}

/** How the wyo server learns what the fulfilment centre did: asking once a minute, or notifications with the address to copy and the signing secret. */
describe('EventsModeForm', () => {
    it('keeps the section folded and, while the wyo server asks once a minute, shows the choice and no address and no secret', async () => {
        draw();

        expect((screen.getByTestId('external-api-events') as HTMLDetailsElement).open).toBe(false);
        await openSection();
        expect(screen.getByTestId('events-mode-box')).toHaveValue('poll');
        expect(screen.queryByTestId('events-hook-url')).toBeNull();
        expect(screen.queryByTestId('events-secret-box')).toBeNull();
    });

    it('shows, for notifications, the wyo server’s address to copy, the secret box as a hidden box, and when the last test notification came', async () => {
        draw(
            eventsOf({
                mode: 'notify',
                hookUrl: 'https://wyo.example.com/hooks/fulfilment/a1b2c3',
                secretSaved: true,
                lastPingAt: '2026-11-18T12:00:00'
            })
        );
        await openSection();

        expect(screen.getByTestId('events-hook-url')).toHaveTextContent(
            'https://wyo.example.com/hooks/fulfilment/a1b2c3'
        );
        expect(screen.getByTestId('events-hook-copy')).toBeInTheDocument();
        const secret = screen.getByTestId('events-secret-box');
        expect(secret).toHaveAttribute('type', 'password');
        expect(secret).toHaveAttribute('autocomplete', 'off');
        expect(secret).toHaveValue('');
        expect(screen.getByTestId('events-secret-saved')).toHaveTextContent('Секрет подписи сохранён');
        expect(screen.getByTestId('events-facts-lastPing')).toHaveTextContent('18 нояб. 2026');
    });

    it('says the test notification has not come yet, and that the address appears after the first save', async () => {
        draw(eventsOf({ mode: 'notify' }));
        await openSection();

        expect(screen.queryByTestId('events-hook-url')).toBeNull();
        expect(screen.getByTestId('events-hook-pending')).toHaveTextContent('появится после сохранения');
        expect(screen.getByTestId('events-facts-lastPing')).toHaveTextContent('ещё не приходило');
    });

    it('switches to notifications with the secret, sends the mode and the secret, tells the page the setting, empties the secret box and folds', async () => {
        const saved = eventsOf({
            mode: 'notify',
            hookUrl: 'https://wyo.example.com/hooks/fulfilment/a1b2c3',
            secretSaved: true
        });
        put.mockResolvedValue({ ok: true, data: saved });
        const onSaved = vi.fn();
        draw(eventsOf(), onSaved);
        const section = await openSection();

        await fireEvent.change(screen.getByTestId('events-mode-box'), { target: { value: 'notify' } });
        await fireEvent.input(screen.getByTestId('events-secret-box'), { target: { value: SECRET } });
        await sendForm(screen.getByTestId('external-api-events-form'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(saved));
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api/events', { mode: 'notify', secret: SECRET });
        expect(screen.getByTestId('events-secret-box')).toHaveValue('');
        expect(screen.getByTestId('events-hook-url')).toHaveTextContent('a1b2c3');
        expect(section.open).toBe(false);
        expect(screen.getByTestId('external-api-events-saved')).toHaveTextContent('Сохранено.');
    });

    it('needs the secret for notifications when none is saved, and stops the form in the browser', async () => {
        draw();
        await openSection();

        await fireEvent.change(screen.getByTestId('events-mode-box'), { target: { value: 'notify' } });
        await sendForm(screen.getByTestId('external-api-events-form'));

        await waitFor(() =>
            expect(screen.getByTestId('events-secret-field')).toHaveTextContent('Это поле нужно заполнить.')
        );
        expect(put).not.toHaveBeenCalled();
    });

    it('lays the wyo server’s refusal of the secret onto its box and keeps what was typed', async () => {
        put.mockResolvedValue({
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'secret', code: 'invalid_secret' }] }
        });
        draw();
        await openSection();
        await fireEvent.change(screen.getByTestId('events-mode-box'), { target: { value: 'notify' } });
        await fireEvent.input(screen.getByTestId('events-secret-box'), { target: { value: SECRET } });

        await sendForm(screen.getByTestId('external-api-events-form'));

        await waitFor(() => expect(screen.getByTestId('events-secret-field')).toHaveTextContent('Секрет не подходит'));
        expect(screen.getByTestId('events-secret-box')).toHaveValue(SECRET);
    });
});

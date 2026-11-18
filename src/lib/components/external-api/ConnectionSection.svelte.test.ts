import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { connectionOf, emptyConnection } from '$test/support/external-api';
import { flushTicks, sendForm } from '$test/support/superform-answer';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { put: vi.fn(), post: vi.fn(), delete: vi.fn() }
}));

import { api } from '$lib/api/api';
import ConnectionSection from './ConnectionSection.svelte';
import type { TExternalApi } from './types';

const put = vi.mocked(api.put);
const post = vi.mocked(api.post);
const del = vi.mocked(api.delete);
const KEY = 'fck_live_9f8e7d6c5b4a3210z9z2';

afterEach(() => {
    put.mockReset();
    post.mockReset();
    del.mockReset();
});

function draw(
    connection: TExternalApi = emptyConnection(),
    handlers = { onChanged: vi.fn(), onDisconnected: vi.fn() },
    allowPlainHttp = false
): typeof handlers {
    render(ConnectionSection, { props: { lang: 'ru', connection, allowPlainHttp, ...handlers } });
    return handlers;
}

async function openSection(): Promise<HTMLDetailsElement> {
    const section = screen.getByTestId('external-api-connection') as HTMLDetailsElement;
    section.open = true;
    await fireEvent(section, new Event('toggle'));
    await flushTicks();
    return section;
}

/** «Подключение»: the address and the key of the fulfilment centre's system, checked and saved together, what it answered, and how to check again or disconnect. */
describe('ConnectionSection', () => {
    it('for a setting not filled, draws the introduction, an address box and a key box that is hidden, and no check or disconnect', async () => {
        draw();
        await openSection();

        expect(screen.getByTestId('external-api-connection-intro')).toHaveTextContent('Подключение нужно');
        expect(screen.getByTestId('connection-address-box')).toHaveValue('');
        const key = screen.getByTestId('connection-key-box');
        expect(key).toHaveAttribute('type', 'password');
        expect(key).toHaveAttribute('autocomplete', 'off');
        expect(screen.queryByTestId('connection-result')).toBeNull();
        expect(screen.queryByTestId('connection-current')).toBeNull();
        expect(screen.queryByTestId('connection-check')).toBeNull();
        expect(screen.queryByTestId('connection-disconnect')).toBeNull();
    });

    it('for a filled setting, shows the saved address, the end of the key and the answer of the fulfilment centre, with «Проверить снова» and «Отключить»', async () => {
        draw(connectionOf());
        await openSection();

        expect(screen.getByTestId('connection-address-box')).toHaveValue('https://fc.example.com/api');
        expect(screen.getByTestId('connection-key-box')).toHaveValue('');
        expect(screen.getByTestId('connection-current')).toHaveTextContent('заканчивается на k9z2');
        expect(screen.getByTestId('connection-result-state')).toHaveTextContent('Работает');
        expect(screen.getByTestId('connection-check')).toHaveTextContent('Проверить снова');
        expect(screen.getByTestId('connection-disconnect')).toHaveTextContent('Отключить');
    });

    it('checks and saves the address and the key together, tells the page the answer, empties the key box and folds', async () => {
        const saved = connectionOf();
        put.mockResolvedValue({ ok: true, data: saved });
        const { onChanged } = draw();
        const section = await openSection();

        await fireEvent.input(screen.getByTestId('connection-address-box'), {
            target: { value: 'https://fc.example.com/api' }
        });
        await fireEvent.input(screen.getByTestId('connection-key-box'), { target: { value: KEY } });
        await sendForm(screen.getByTestId('external-api-connection-form'));

        await waitFor(() => expect(onChanged).toHaveBeenCalledExactlyOnceWith(saved));
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api', {
            address: 'https://fc.example.com/api',
            key: KEY
        });
        expect(screen.getByTestId('connection-key-box')).toHaveValue('');
        expect(screen.getByTestId('connection-address-box')).toHaveValue('https://fc.example.com/api');
        expect(section.open).toBe(false);
        expect(screen.getByTestId('external-api-connection-saved')).toHaveTextContent('Сохранено.');
    });

    it('stops an address that is not https in the browser, under the address box, and sends nothing', async () => {
        draw();
        await openSection();

        await fireEvent.input(screen.getByTestId('connection-address-box'), {
            target: { value: 'http://fc.example.com' }
        });
        await fireEvent.input(screen.getByTestId('connection-key-box'), { target: { value: KEY } });
        await sendForm(screen.getByTestId('external-api-connection-form'));

        await waitFor(() =>
            expect(screen.getByTestId('connection-address-field')).toHaveTextContent(
                'Адрес должен начинаться с «https://»'
            )
        );
        expect(put).not.toHaveBeenCalled();
    });

    it('takes a plain http address when told the wyo server is a build for development and tests, and sends it', async () => {
        put.mockResolvedValue({ ok: true, data: connectionOf() });
        const { onChanged } = draw(emptyConnection(), { onChanged: vi.fn(), onDisconnected: vi.fn() }, true);
        await openSection();

        await fireEvent.input(screen.getByTestId('connection-address-box'), {
            target: { value: 'http://127.0.0.1:8792' }
        });
        await fireEvent.input(screen.getByTestId('connection-key-box'), { target: { value: KEY } });
        await sendForm(screen.getByTestId('external-api-connection-form'));

        await waitFor(() => expect(onChanged).toHaveBeenCalledOnce());
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api', {
            address: 'http://127.0.0.1:8792',
            key: KEY
        });
    });

    it.each([
        ['fc_read_only_key', 'только читает'],
        ['fc_key_refused', 'не принимает ключ'],
        ['fc_api_off', 'выключил'],
        ['fc_forbidden', 'отказала в доступе'],
        ['fc_unavailable', 'не может ответить'],
        ['fc_no_connection', 'не смогла соединиться'],
        ['fc_unreadable', 'не умеет читать']
    ])('words the refusal %s under the buttons and keeps what was typed', async (code, text) => {
        put.mockResolvedValue({ ok: false, status: 422, refusal: { code } });
        const { onChanged } = draw();
        await openSection();
        await fireEvent.input(screen.getByTestId('connection-address-box'), {
            target: { value: 'https://fc.example.com/api' }
        });
        await fireEvent.input(screen.getByTestId('connection-key-box'), { target: { value: KEY } });

        await sendForm(screen.getByTestId('external-api-connection-form'));

        await waitFor(() => expect(screen.getByTestId('external-api-connection-error')).toHaveTextContent(text));
        expect(screen.getByTestId('connection-key-box')).toHaveValue(KEY);
        expect(onChanged).not.toHaveBeenCalled();
    });

    it('lays a refusal of the address by the wyo server onto the address box as well as under the buttons', async () => {
        put.mockResolvedValue({ ok: false, status: 422, refusal: { code: 'invalid_address' } });
        draw();
        await openSection();
        await fireEvent.input(screen.getByTestId('connection-address-box'), { target: { value: 'https://fc' } });
        await fireEvent.input(screen.getByTestId('connection-key-box'), { target: { value: KEY } });

        await sendForm(screen.getByTestId('external-api-connection-form'));

        await waitFor(() =>
            expect(screen.getByTestId('connection-address-field')).toHaveTextContent(
                'Адрес должен начинаться с «https://»'
            )
        );
    });

    it('checks the saved setting again, tells the page the new state and says so', async () => {
        const answer = connectionOf({ state: 'unavailable' });
        post.mockResolvedValue({ ok: true, data: answer });
        const { onChanged } = draw(connectionOf());
        await openSection();

        await fireEvent.click(screen.getByTestId('connection-check'));

        await waitFor(() => expect(onChanged).toHaveBeenCalledExactlyOnceWith(answer));
        expect(post).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api/check', {});
        expect(screen.getByTestId('connection-check-answer-saved')).toHaveTextContent('Проверка выполнена');
    });

    it('disconnects only after the owner confirms in a window that says what stays, and tells the page', async () => {
        del.mockResolvedValue({ ok: true, data: undefined });
        const { onDisconnected } = draw(connectionOf());
        await openSection();

        await fireEvent.click(screen.getByTestId('connection-disconnect'));
        expect(screen.getByTestId('connection-disconnect-window')).toHaveTextContent('Соответствия складов');
        expect(del).not.toHaveBeenCalled();
        await fireEvent.click(screen.getByTestId('connection-disconnect-window-confirm'));

        await waitFor(() => expect(onDisconnected).toHaveBeenCalledOnce());
        expect(del).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api');
    });
});

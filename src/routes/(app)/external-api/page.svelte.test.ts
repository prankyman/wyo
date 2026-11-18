import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
    connectionOf,
    emptyConnection,
    eventsOf,
    marketplaceMatchesOf,
    warehouseMatchesOf
} from '$test/support/external-api';
import { MARKETPLACE } from '$test/support/marketplaces';
import { flushTicks, sendForm } from '$test/support/superform-answer';
import type { TExternalApi } from '$lib/components/external-api/types';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn(), getMeta: vi.fn(), put: vi.fn(), post: vi.fn(), delete: vi.fn() }
}));

import { api, type TApiResult } from '$lib/api/api';
import Page from './+page.svelte';

const get = vi.mocked(api.get);
const getMeta = vi.mocked(api.getMeta);
const put = vi.mocked(api.put);
const del = vi.mocked(api.delete);

const REFUSAL: TApiResult<unknown> = { ok: false, status: 500, refusal: { code: 'internal' } };

/** The wyo server's answers to the page’s calls; `change` replaces one answer. */
function answers(
    connection: TExternalApi,
    change: Record<string, TApiResult<unknown>> = {},
    mode: 'release' | 'test' = 'release'
): void {
    getMeta.mockResolvedValue({ ok: true, data: { name: 'wyo', version: '1.2.3', mode } });
    const data: Record<string, TApiResult<unknown>> = {
        '/api/v1/external-api': { ok: true, data: connection },
        '/api/v1/external-api/warehouses': { ok: true, data: warehouseMatchesOf() },
        '/api/v1/external-api/marketplaces': { ok: true, data: marketplaceMatchesOf() },
        '/api/v1/external-api/events': { ok: true, data: eventsOf() },
        '/api/v1/marketplaces': { ok: true, data: { marketplaces: [MARKETPLACE] } },
        ...change
    };
    get.mockImplementation(async (path: string) => data[path] as never);
}

afterEach(() => {
    get.mockReset();
    getMeta.mockReset();
    put.mockReset();
    del.mockReset();
});

const SECTIONS = [
    'external-api-connection',
    'external-api-warehouses',
    'external-api-marketplaces',
    'external-api-events',
    'external-api-handover'
];

/** «Внешний API»: the introduction, the section «Подключение», and, once the setting is filled, the four sections that depend on it. */
describe('«Внешний API»', () => {
    it('draws its heading with one «?» and, before the setting is filled, the introduction and «Подключение» only', async () => {
        answers(emptyConnection());

        render(Page);

        expect(screen.getByTestId('external-api-heading')).toHaveTextContent('Внешний API');
        expect(screen.getAllByTestId('external-api-help-button')).toHaveLength(1);
        expect(screen.getByTestId('external-api-loading')).toBeInTheDocument();
        await screen.findByTestId('external-api-connection');
        const about = screen.getByTestId('external-api-about');
        expect(about).toHaveTextContent('Фулфилмент-центр — это компания со своим складом');
        expect(about).toHaveTextContent('брать новые заказы в сборку');
        expect(about).toHaveTextContent('не отменяет заказ на маркетплейсе');
        expect(about).toHaveTextContent('не отправляет фулфилмент-центру ключ маркетплейса и данные покупателя');
        for (const hidden of SECTIONS.slice(1)) expect(screen.queryByTestId(hidden)).toBeNull();
        expect(screen.getByTestId('external-api-empty-next')).toBeInTheDocument();
        expect(get).toHaveBeenCalledExactlyOnceWith('/api/v1/external-api');
    });

    it('draws the five folded sections once the setting is filled, reading each one’s data', async () => {
        answers(connectionOf());

        render(Page);

        await waitFor(() => expect(screen.getByTestId('external-api-handover')).toBeInTheDocument());
        for (const id of SECTIONS) expect((screen.getByTestId(id) as HTMLDetailsElement).open).toBe(false);
        expect(screen.queryByTestId('external-api-not-working')).toBeNull();
        expect(screen.queryByTestId('external-api-empty-next')).toBeNull();
        for (const path of ['warehouses', 'marketplaces', 'events']) {
            expect(get).toHaveBeenCalledWith(`/api/v1/external-api/${path}`);
        }
        expect(get).toHaveBeenCalledWith('/api/v1/marketplaces');
    });

    it.each([
        ['a released wyo server', 'release' as const, false],
        ['a build for development and tests', 'test' as const, true]
    ])(
        'takes a plain http address from %s only if it is a build for development and tests',
        async (_name, mode, sent) => {
            answers(emptyConnection(), {}, mode);
            put.mockResolvedValue({ ok: true, data: connectionOf() });
            render(Page);
            const section = (await screen.findByTestId('external-api-connection')) as HTMLDetailsElement;
            section.open = true;
            await fireEvent(section, new Event('toggle'));
            await waitFor(() => expect(getMeta).toHaveBeenCalledOnce());
            await flushTicks();

            await fireEvent.input(screen.getByTestId('connection-address-box'), {
                target: { value: 'http://127.0.0.1:8792' }
            });
            await fireEvent.input(screen.getByTestId('connection-key-box'), { target: { value: 'a-key' } });
            await sendForm(screen.getByTestId('external-api-connection-form'));

            if (sent) await waitFor(() => expect(put).toHaveBeenCalledOnce());
            else
                await waitFor(() =>
                    expect(screen.getByTestId('connection-address-field')).toHaveTextContent('https://')
                );
            expect(put).toHaveBeenCalledTimes(sent ? 1 : 0);
        }
    );

    it('says the exchange does not work while the setting is filled but its last check failed', async () => {
        answers(connectionOf({ state: 'key_refused' }));

        render(Page);

        expect(await screen.findByTestId('external-api-not-working')).toHaveTextContent('Обмен работает, только пока');
    });

    it('keeps a section whose data could not be read as a folded place with «Повторить», and reads it again', async () => {
        answers(connectionOf(), { '/api/v1/external-api/warehouses': REFUSAL });

        render(Page);

        expect(await screen.findByTestId('external-api-warehouses-refusal')).toHaveTextContent('произошла ошибка');
        expect(screen.getByTestId('external-api-marketplaces-intro')).toBeInTheDocument();
        answers(connectionOf());
        await fireEvent.click(screen.getByTestId('external-api-warehouses-retry'));

        await waitFor(() => expect(screen.getByTestId('external-api-warehouses-intro')).toBeInTheDocument());
        expect(screen.queryByTestId('external-api-warehouses-refusal')).toBeNull();
    });

    it('words the refusal when the setting itself cannot be read, and reads it again on «Повторить»', async () => {
        answers(emptyConnection(), { '/api/v1/external-api': REFUSAL });
        render(Page);

        expect(await screen.findByTestId('external-api-refusal')).toHaveTextContent(
            'В программе wyo произошла ошибка.'
        );
        answers(emptyConnection());
        await fireEvent.click(screen.getByTestId('external-api-retry'));

        expect(await screen.findByTestId('external-api-connection')).toBeInTheDocument();
        expect(screen.queryByTestId('external-api-refusal')).toBeNull();
    });

    it('goes back to the introduction and «Подключение» only when the owner disconnects', async () => {
        answers(connectionOf());
        del.mockResolvedValue({ ok: true, data: undefined });
        render(Page);
        await waitFor(() => expect(screen.getByTestId('external-api-handover')).toBeInTheDocument());

        const section = screen.getByTestId('external-api-connection') as HTMLDetailsElement;
        section.open = true;
        await fireEvent(section, new Event('toggle'));
        await fireEvent.click(within(section).getByTestId('connection-disconnect'));
        await fireEvent.click(screen.getByTestId('connection-disconnect-window-confirm'));

        await waitFor(() => expect(screen.queryByTestId('external-api-warehouses')).toBeNull());
        expect(screen.getByTestId('external-api-empty-next')).toBeInTheDocument();
        expect(screen.getByTestId('connection-address-box')).toHaveValue('');
    });
});

import { describe, expect, it, vi } from 'vitest';
import type { TApiResult } from '$lib/api/api';
import { ApiAction } from './api-action.svelte';

const done = (): Promise<TApiResult<unknown>> => Promise.resolve({ ok: true, data: {} });

describe('ApiAction', () => {
    it('is working while its call runs, then says the success in words of the page and returns the answer', async () => {
        const action = new ApiAction();
        let finish: (result: TApiResult<unknown>) => void = () => undefined;
        const running = action.run(
            () => new Promise<TApiResult<unknown>>((resolve) => (finish = resolve)),
            'common.disclosure.saved'
        );

        expect(action.pending).toBe(true);
        expect([action.saved, action.error]).toEqual([null, null]);

        finish({ ok: true, data: { name: 'x' } });
        await expect(running).resolves.toEqual({ ok: true, data: { name: 'x' } });
        expect(action.pending).toBe(false);
        expect(action.saved).toBe('Сохранено.');
    });

    it('words a refusal from its code and its parameters, and clears the earlier answer when pressed again', async () => {
        const action = new ApiAction();
        await action.run(
            () =>
                Promise.resolve({
                    ok: false,
                    status: 429,
                    refusal: { code: 'too_many_attempts', params: { minutes: 7 } }
                }),
            'common.disclosure.saved'
        );
        expect(action.saved).toBeNull();
        expect(action.error).toBe('Слишком много попыток. Подождите 7 мин. и повторите.');

        const running = action.run(done, 'common.disclosure.saved');
        expect(action.error).toBeNull();
        await running;
        expect(action.saved).toBe('Сохранено.');
    });

    it('words a refusal from the family of texts of its place first', async () => {
        const action = new ApiAction();

        await action.run(
            () => Promise.resolve({ ok: false, status: 409, refusal: { code: 'account_has_orders' } }),
            undefined,
            'accounts.refusal'
        );

        expect(action.error).toContain('пришёл хотя бы один заказ');
    });

    it('says no success when it is given no words for one, as a press that moves the person on', async () => {
        const action = new ApiAction();

        await action.run(done);

        expect([action.saved, action.error]).toEqual([null, null]);
    });

    it('does not start a second call while one runs', async () => {
        const action = new ApiAction();
        const call = vi.fn(() => new Promise<TApiResult<unknown>>(() => undefined));
        void action.run(call, 'common.disclosure.saved');

        const second = await action.run(call, 'common.disclosure.saved');

        expect(call).toHaveBeenCalledOnce();
        expect(second).toBeNull();
    });
});

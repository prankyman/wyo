import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, apiRequest, messageParams, refusalParams, setSignedOutHandler } from './api';

/** A fetch that answers with `body` as JSON (or as raw text when it is a string) and `status`. */
function answering(status: number, body: unknown) {
    const text = typeof body === 'string' ? body : JSON.stringify(body);
    // A response with a "no content" status cannot be built with a body, not even an empty one.
    const content = status === 204 ? null : text;
    const fetchStub = vi.fn(
        async () => new Response(content, { status, headers: { 'Content-Type': 'application/json' } })
    );
    vi.stubGlobal('fetch', fetchStub);
    return fetchStub;
}

afterEach(() => {
    vi.unstubAllGlobals();
    setSignedOutHandler(null);
});

describe('apiRequest', () => {
    it('returns the data of an answer that went through', async () => {
        const stub = answering(200, { name: 'wyo' });

        await expect(apiRequest('GET', '/api/v1/meta')).resolves.toEqual({ ok: true, data: { name: 'wyo' } });
        expect(stub).toHaveBeenCalledWith('/api/v1/meta', {
            method: 'GET',
            headers: { Accept: 'application/json', 'X-Wyo-Person': '1' }
        });
    });

    it('sends a body as JSON with its content type', async () => {
        const stub = answering(200, {});

        await apiRequest('POST', '/api/v1/things', { name: 'a' });

        expect(stub).toHaveBeenCalledWith('/api/v1/things', {
            method: 'POST',
            headers: { Accept: 'application/json', 'X-Wyo-Person': '1', 'Content-Type': 'application/json' },
            body: '{"name":"a"}'
        });
    });

    it('returns nothing as data for an answer with no body', async () => {
        answering(204, '');

        await expect(apiRequest('DELETE', '/api/v1/things/1')).resolves.toEqual({ ok: true, data: undefined });
    });

    it('returns the refusal of the wyo server with its status', async () => {
        const refusal = { code: 'invalid', params: { max: 3 }, issues: [{ path: 'name', code: 'too_long' }] };
        answering(422, { error: refusal });

        await expect(apiRequest('POST', '/api/v1/things', {})).resolves.toEqual({ ok: false, status: 422, refusal });
    });

    it('says the call never arrived when the network fails, and does not throw', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn(async () => {
                throw new TypeError('Failed to fetch');
            })
        );

        await expect(apiRequest('GET', '/api/v1/meta')).resolves.toEqual({
            ok: false,
            status: 0,
            refusal: { code: 'network' }
        });
    });

    it.each([
        ['an error page that is not JSON', 502, '<html>Bad gateway</html>'],
        ['JSON that is not a refusal', 500, { message: 'x' }],
        ['a success whose body is not JSON', 200, 'not json']
    ])('calls %s a bad answer', async (_case, status, body) => {
        answering(status, body);

        await expect(apiRequest('GET', '/api/v1/meta')).resolves.toEqual({
            ok: false,
            status,
            refusal: { code: 'bad_answer' }
        });
    });
});

describe('the idle sign-out header', () => {
    it('marks a call the person caused, and leaves the mark out of a background re-read', async () => {
        const stub = answering(200, {});

        await apiRequest('GET', '/api/v1/notices');
        await apiRequest('GET', '/api/v1/notices', undefined, { background: true });
        await api.get('/api/v1/notices', { background: true });

        const sentHeaders = stub.mock.calls.map((call) => (call as unknown as [string, RequestInit])[1].headers);
        expect(sentHeaders).toEqual([
            { Accept: 'application/json', 'X-Wyo-Person': '1' },
            { Accept: 'application/json' },
            { Accept: 'application/json' }
        ]);
    });
});

describe('a signed-out answer', () => {
    it('is handed to the one handler, a call by the person or a background one alike, and still returned', async () => {
        const handler = vi.fn();
        setSignedOutHandler(handler);
        answering(401, { error: { code: 'signed_out' } });

        const person = await apiRequest('GET', '/api/v1/settings');
        await apiRequest('GET', '/api/v1/notices', undefined, { background: true });

        expect(handler).toHaveBeenCalledTimes(2);
        expect(person).toEqual({ ok: false, status: 401, refusal: { code: 'signed_out' } });
    });

    it('is not handed over for another refusal, and with no handler set nothing happens', async () => {
        const handler = vi.fn();
        setSignedOutHandler(handler);
        answering(401, { error: { code: 'sign_in_refused' } });
        await apiRequest('POST', '/api/v1/session', {});
        expect(handler).not.toHaveBeenCalled();

        setSignedOutHandler(null);
        answering(401, { error: { code: 'signed_out' } });
        await expect(apiRequest('GET', '/api/v1/session')).resolves.toMatchObject({ ok: false });
    });
});

describe('api', () => {
    it('asks for the program meta', async () => {
        const stub = answering(200, { name: 'wyo', version: 'dev', mode: 'test' });

        await expect(api.getMeta()).resolves.toEqual({ ok: true, data: { name: 'wyo', version: 'dev', mode: 'test' } });
        expect(stub).toHaveBeenCalledWith('/api/v1/meta', expect.objectContaining({ method: 'GET' }));
    });

    it.each([
        ['post', 'POST'],
        ['put', 'PUT'],
        ['patch', 'PATCH']
    ] as const)('sends a %s with a body', async (name, method) => {
        const stub = answering(200, {});

        await api[name]('/api/v1/things', { a: 1 });

        expect(stub).toHaveBeenCalledWith('/api/v1/things', expect.objectContaining({ method, body: '{"a":1}' }));
    });

    it('sends a delete without a body', async () => {
        const stub = answering(204, '');

        await api.delete('/api/v1/things/1');

        expect(stub).toHaveBeenCalledWith('/api/v1/things/1', expect.objectContaining({ method: 'DELETE' }));
    });
});

describe('messageParams', () => {
    it('keeps the texts and numbers of a notice’s parameters and says nothing for a notice that has none', () => {
        expect(messageParams({ version: '1.3.0', days: 3, list: [1] })).toEqual({ version: '1.3.0', days: 3 });
        expect(messageParams(undefined)).toBeUndefined();
    });
});

describe('refusalParams', () => {
    it('keeps the texts and numbers a message can be filled with and drops the rest', () => {
        expect(
            refusalParams({ code: 'x', params: { max: 3, name: 'a', list: [1], nested: { a: 1 }, no: null } })
        ).toEqual({
            max: 3,
            name: 'a'
        });
        expect(refusalParams({ code: 'x' })).toBeUndefined();
    });
});

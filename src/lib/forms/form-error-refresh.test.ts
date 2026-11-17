import { describe, expect, it } from 'vitest';
import { splitPath } from 'sveltekit-superforms';
import {
    collectErrors,
    firstErrorByField,
    hasChanged,
    pathKey,
    refreshedErrors,
    sameValue,
    serverOnlyErrors,
    snapshotValue,
    valueAtPath
} from './form-error-refresh';

/**
 * The rules these tests hold to are the ones in `form-error-refresh.ts`: a message the browser
 * cannot re-check stays until its field changes, a failing field speaks only when it is allowed
 * to, and everything else is silent.
 */

const noServerErrors = new Map<string, unknown>();

/** One place for the shape the refresh takes, so each test states only what it is about. */
function refresh(overrides: {
    current?: unknown;
    fresh?: unknown;
    justLeft?: string[];
    tainted?: unknown;
    data?: unknown;
    serverOnly?: Map<string, unknown>;
}) {
    return refreshedErrors({
        current: overrides.current ?? {},
        fresh: overrides.fresh ?? {},
        justLeft: overrides.justLeft ?? [],
        tainted: overrides.tainted ?? {},
        data: overrides.data ?? {},
        serverOnly: overrides.serverOnly ?? noServerErrors
    });
}

describe('collectErrors', () => {
    it('finds every message by path and ignores the blanks superforms leaves behind', () => {
        const found = collectErrors({
            email: ['form.emailInvalid'],
            city: undefined,
            postalCode: [],
            address: { city: ['address.cityRequired'] },
            items: { '0': { _errors: ['bag.itemGone'] } }
        });

        expect([...found.entries()]).toEqual([
            ['email', ['form.emailInvalid']],
            ['address.city', ['address.cityRequired']],
            ['items.0._errors', ['bag.itemGone']]
        ]);
    });
});

describe('firstErrorByField', () => {
    it('keeps the first message of each field by its path and leaves out the form’s own messages', () => {
        expect(
            firstErrorByField({
                _errors: ['form.refused'],
                owner: { title: ['a.required', 'a.tooLong'] },
                lines: { _errors: ['lines.empty'], 0: { itemId: ['item.required'] } },
                note: [undefined]
            })
        ).toEqual({ 'owner.title': 'a.required', lines: 'lines.empty', 'lines.0.itemId': 'item.required' });
    });
});

describe('valueAtPath', () => {
    it.each([
        ['email', 'a@b.example'],
        ['address.city', 'Alpha'],
        ['address.missing', undefined],
        ['email.deeper', undefined],
        ['', undefined]
    ])('reads %s', (path, expected) => {
        const data = { email: 'a@b.example', address: { city: 'Alpha' } };
        expect(valueAtPath(path === '' ? undefined : data, path)).toEqual(path === '' ? undefined : expected);
    });
});

describe('hasChanged', () => {
    it.each([
        ['a field the visitor edited', 'email', true],
        ['a field below one they edited', 'address', true],
        ['a field nobody has touched', 'city', false]
    ])('answers for %s', (_case, path, expected) => {
        const tainted = { email: true, address: { city: true }, city: undefined };
        expect(hasChanged(tainted, path)).toBe(expected);
    });
});

describe('sameValue', () => {
    it.each([
        ['identical strings', 'alpha', 'alpha', true],
        ['different strings', 'alpha', 'beta', false],
        ['equal dates', new Date('2026-09-17'), new Date('2026-09-17'), true],
        ['different dates', new Date('2026-09-17'), new Date('2026-09-18'), false],
        ['equal arrays', ['a', 'b'], ['a', 'b'], true],
        ['arrays of different length', ['a'], ['a', 'b'], false],
        ['equal objects', { city: 'Alpha' }, { city: 'Alpha' }, true],
        ['objects with a different key count', { city: 'Alpha' }, { city: 'Alpha', zip: '1' }, false],
        ['objects with a different key', { city: 'Alpha' }, { town: 'Alpha' }, false],
        ['a string and a number', '7', 7, false]
    ])('compares %s', (_case, left, right, expected) => {
        expect(sameValue(left, right)).toBe(expected);
    });
});

describe('snapshotValue', () => {
    it('copies an object so a later edit of the form does not look like no change at all', () => {
        const value = { city: 'Alpha' };
        const snapshot = snapshotValue(value) as { city: string };
        value.city = 'Beta';

        expect(snapshot.city).toBe('Alpha');
    });

    it('keeps a value it cannot copy, rather than throwing on the way past', () => {
        const uncopyable = { keep: () => 'me' };
        expect(snapshotValue(uncopyable)).toBe(uncopyable);
    });

    it('hands back a primitive unchanged', () => {
        expect(snapshotValue('1234567')).toBe('1234567');
    });
});

describe('pathKey', () => {
    it.each([
        ['name', 'name'],
        ['lines[0].qty', 'lines.0.qty'],
        ['lines.0.qty', 'lines.0.qty'],
        ['matrix[1][2]', 'matrix.1.2']
    ])('reads %s as the key %s', (path, key) => {
        expect(pathKey(path)).toBe(key);
    });

    it('cuts a path exactly where the form library does, so the two can never read one field as two', () => {
        for (const path of ['name', 'lines[0].qty', 'lines.0.qty', 'matrix[1][2]', 'a.b[3].c.d', '[0]x'])
            expect(pathKey(path), path).toBe(splitPath(path).join('.'));
    });
});

describe('serverOnlyErrors', () => {
    it('records the messages the browser cannot reproduce, with the value they were raised on', () => {
        const records = serverOnlyErrors(
            { code: ['form.codeTaken'], email: ['form.emailInvalid'] },
            { email: ['form.emailInvalid'] },
            { code: '1234567', email: 'not-an-email' }
        );

        expect([...records.entries()]).toEqual([['code', '1234567']]);
    });
});

describe('refreshedErrors', () => {
    it('clears a message the check no longer produces', () => {
        const result = refresh({ current: { email: ['form.emailInvalid'] }, data: { email: 'a@b.example' } });

        expect(result.errors).toEqual({});
    });

    it('replaces a message on screen with what the check says now', () => {
        const result = refresh({
            current: { postalCode: ['form.postalCodeRequired'] },
            fresh: { postalCode: ['form.invalidPostalCode'] }
        });

        expect(result.errors).toEqual({ postalCode: ['form.invalidPostalCode'] });
    });

    it('shows the message of the field just left, even though nothing showed one before', () => {
        const result = refresh({ fresh: { city: ['form.cityRequired'] }, justLeft: ['city'] });

        expect(result.errors).toEqual({ city: ['form.cityRequired'] });
    });

    it('stays silent about a field nobody has reached', () => {
        const result = refresh({
            fresh: { city: ['form.cityRequired'], postalCode: ['form.postalCodeRequired'] },
            justLeft: ['city']
        });

        expect(result.errors).toEqual({ city: ['form.cityRequired'] });
    });

    it('speaks up about a field whose value changed — which is how an autofilled field is caught', () => {
        const result = refresh({
            fresh: { postalCode: ['form.invalidPostalCode'] },
            justLeft: ['city'],
            tainted: { postalCode: true }
        });

        expect(result.errors).toEqual({ postalCode: ['form.invalidPostalCode'] });
    });

    it('keeps a server message while its field still holds the value it came with', () => {
        const result = refresh({
            current: { code: ['form.codeTaken'] },
            data: { code: '1234567' },
            justLeft: ['code'],
            serverOnly: new Map([['code', '1234567']])
        });

        expect(result.errors).toEqual({ code: ['form.codeTaken'] });
        expect([...result.serverOnly.keys()]).toEqual(['code']);
    });

    it('drops a server message once its field is changed', () => {
        const result = refresh({
            current: { code: ['form.codeTaken'] },
            data: { code: '7707083893' },
            justLeft: ['code'],
            tainted: { code: true },
            serverOnly: new Map([['code', '1234567']])
        });

        expect(result.errors).toEqual({});
        expect(result.serverOnly.size).toBe(0);
    });

    it('lets the browser speak about a changed field the server had complained about', () => {
        const result = refresh({
            current: { code: ['form.codeTaken'] },
            fresh: { code: ['form.codeInvalid'] },
            data: { code: '77' },
            justLeft: ['code'],
            serverOnly: new Map([['code', '1234567']])
        });

        expect(result.errors).toEqual({ code: ['form.codeInvalid'] });
        expect(result.serverOnly.size).toBe(0);
    });

    it('carries a message under the field the visitor left, however the path is spelled', () => {
        const result = refresh({
            fresh: { items: { '0': { quantity: ['bag.tooMany'] } } },
            justLeft: ['items[0].quantity']
        });

        expect(result.errors).toEqual({ items: { '0': { quantity: ['bag.tooMany'] } } });
    });

    it('shows an array’s own message when the visitor leaves the array', () => {
        const result = refresh({
            fresh: { acceptedTermsIds: { _errors: ['form.termsRequired'] } },
            justLeft: ['acceptedTermsIds']
        });

        expect(result.errors).toEqual({ acceptedTermsIds: { _errors: ['form.termsRequired'] } });
    });
});

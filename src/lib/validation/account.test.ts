import { describe, expect, it } from 'vitest';
import { MARKETPLACE } from '$test/support/marketplaces';
import {
    accountChangePayload,
    accountChangeSchema,
    accountCreateSchema,
    DEADLINE_HOURS,
    emptyKey,
    keyReplaceSchema
} from './account';

const issues = (result: { success: boolean; error?: { issues: { path: PropertyKey[]; message: string }[] } }) =>
    (result.error?.issues ?? []).map((issue) => [issue.path.join('.'), issue.message]);

describe('the forms of a marketplace account', () => {
    it('asks for a name and every field of the adapter’s key form, nested under `key` as the wyo server names them', () => {
        const schema = accountCreateSchema(MARKETPLACE.keyFields);

        expect(issues(schema.safeParse({ name: '  ', key: { token: '' } }))).toEqual([
            ['name', 'errors.required'],
            ['key.token', 'errors.required']
        ]);
        expect(schema.safeParse({ name: 'Магазин', key: { token: ' abc ' } }).data).toEqual({
            name: 'Магазин',
            key: { token: 'abc' }
        });
        expect(emptyKey(MARKETPLACE.keyFields)).toEqual({ token: '' });
    });

    it('asks only for the key when a key is replaced', () => {
        expect(issues(keyReplaceSchema(MARKETPLACE.keyFields).safeParse({ key: { token: '' } }))).toEqual([
            ['key.token', 'errors.required']
        ]);
    });

    it.each([
        ['48', true, 48],
        ['1', true, 1],
        [String(DEADLINE_HOURS.max), true, DEADLINE_HOURS.max],
        ['0', false, null],
        [String(DEADLINE_HOURS.max + 1), false, null],
        ['12,5', false, null]
    ])('takes %s hours to hand an order over: %s', (typed, valid, hours) => {
        const result = accountChangeSchema.safeParse({ name: 'Магазин', deadlineHours: typed });

        expect(result.success).toBe(valid);
        if (result.success)
            expect(accountChangePayload(result.data)).toEqual({ name: 'Магазин', deadlineHours: hours });
    });
});

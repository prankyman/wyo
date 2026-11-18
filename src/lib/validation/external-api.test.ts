import { describe, expect, it } from 'vitest';
import {
    connectSchema,
    connectSchemaFor,
    eventsPayload,
    eventsSchema,
    handoverSettingsPayload,
    marketplacesPayload,
    warehousesPayload
} from './external-api';

const messages = (result: { error?: { issues: { path: PropertyKey[]; message: string }[] } }) =>
    result.error?.issues.map((issue) => [issue.path.join('.'), issue.message]);

describe('connectSchema', () => {
    it('takes an https address and a key as typed, without surrounding spaces', () => {
        expect(connectSchema.safeParse({ address: ' https://fc.example.com/api ', key: ' k ' })).toMatchObject({
            success: true,
            data: { address: 'https://fc.example.com/api', key: 'k' }
        });
    });

    it.each([
        ['an address with no scheme', 'fc.example.com', 'errors.invalid_address'],
        ['an address with an unencrypted scheme', 'http://fc.example.com', 'errors.invalid_address'],
        ['an address with spaces inside', 'https://fc example.com', 'errors.invalid_address'],
        ['no address', '', 'errors.required']
    ])('refuses %s under the address', (_case, address, message) => {
        expect(messages(connectSchema.safeParse({ address, key: 'k' }))).toEqual([['address', message]]);
    });

    it('asks for the key', () => {
        expect(messages(connectSchema.safeParse({ address: 'https://fc.example.com', key: '' }))).toEqual([
            ['key', 'errors.required']
        ]);
    });
});

describe('connectSchemaFor', () => {
    it('takes a plain http address when the wyo server is a build for development and tests, and still not one without a scheme', () => {
        const schema = connectSchemaFor(() => true);

        expect(schema.safeParse({ address: 'http://127.0.0.1:8792', key: 'k' }).success).toBe(true);
        expect(messages(schema.safeParse({ address: '127.0.0.1:8792', key: 'k' }))).toEqual([
            ['address', 'errors.invalid_address']
        ]);
    });

    it('asks again at every check, because the page learns what the wyo server is after it is drawn', () => {
        let allowed = false;
        const schema = connectSchemaFor(() => allowed);

        expect(schema.safeParse({ address: 'http://fc.example.com', key: 'k' }).success).toBe(false);
        allowed = true;
        expect(schema.safeParse({ address: 'http://fc.example.com', key: 'k' }).success).toBe(true);
    });
});

describe('eventsSchema', () => {
    it('needs no secret while the wyo server asks, and does not mind one typed', () => {
        expect(eventsSchema(() => false).safeParse({ mode: 'poll', secret: '' }).success).toBe(true);
    });

    it('needs the signing secret for notifications when none is saved yet, and not when one is', () => {
        expect(messages(eventsSchema(() => false).safeParse({ mode: 'notify', secret: '' }))).toEqual([
            ['secret', 'errors.required']
        ]);
        expect(eventsSchema(() => true).safeParse({ mode: 'notify', secret: '' }).success).toBe(true);
    });

    it('refuses a secret that is not «whsec_» and base64', () => {
        expect(messages(eventsSchema(() => true).safeParse({ mode: 'notify', secret: 'plain-text' }))).toEqual([
            ['secret', 'errors.invalid_secret']
        ]);
        expect(
            eventsSchema(() => true).safeParse({ mode: 'notify', secret: 'whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw' })
                .success
        ).toBe(true);
    });

    it('sends the secret only when one was typed', () => {
        expect(eventsPayload({ mode: 'notify', secret: '' })).toEqual({ mode: 'notify' });
        expect(eventsPayload({ mode: 'notify', secret: 'whsec_AAAA' })).toEqual({
            mode: 'notify',
            secret: 'whsec_AAAA'
        });
    });
});

describe('the payloads of the matches', () => {
    it('sends a warehouse row with no match without a warehouse, which removes the match', () => {
        expect(
            warehousesPayload({
                rows: [
                    { accountId: 7, warehouseNumber: '1001', fcWarehouse: 'MSK-1' },
                    { accountId: 7, warehouseNumber: '1002', fcWarehouse: '' }
                ]
            })
        ).toEqual({
            rows: [
                { accountId: 7, warehouseNumber: '1001', fcWarehouse: 'MSK-1' },
                { accountId: 7, warehouseNumber: '1002' }
            ]
        });
    });

    it('sends a marketplace code as chosen, an empty one meaning no match, and leaves the hand-over settings alone', () => {
        expect(
            marketplacesPayload({
                rows: [
                    { accountId: 7, fcMarketplace: 'mp-a' },
                    { accountId: 8, fcMarketplace: '' }
                ]
            })
        ).toEqual({
            rows: [
                { accountId: 7, fcMarketplace: 'mp-a' },
                { accountId: 8, fcMarketplace: '' }
            ]
        });
    });

    it('sends the hand-over settings of each account and leaves the marketplace code alone', () => {
        expect(
            handoverSettingsPayload({ rows: [{ accountId: 7, handoverSettings: { city: 'Москва', office_type: '' } }] })
        ).toEqual({ rows: [{ accountId: 7, handoverSettings: { city: 'Москва', office_type: '' } }] });
    });
});

import { describe, expect, it } from 'vitest';
import { fieldRefusalKey, refusalKey } from './refusal-key';

describe('the words of a refusal', () => {
    it('words a code from messages/ and falls back to a general text for a code nobody wrote one for', () => {
        expect(refusalKey('network')).toBe('errors.network');
        expect(refusalKey('never_heard_of_it')).toBe('errors.unknown');
    });

    it('words a code from the family of texts of the place first, and from the general texts when the family has none', () => {
        expect(refusalKey('key_refused', 'accounts.refusal')).toBe('accounts.refusal.key_refused');
        expect(refusalKey('network', 'accounts.refusal')).toBe('errors.network');
        expect(fieldRefusalKey('key_expired', 'accounts.refusal')).toBe('accounts.refusal.key_expired');
        expect(fieldRefusalKey('required', 'accounts.refusal')).toBe('errors.required');
    });

    it('words the refusal of one field the same way, with its own general text', () => {
        expect(fieldRefusalKey('invalid')).toBe('errors.invalid');
        expect(fieldRefusalKey('never_heard_of_it')).toBe('errors.invalid');
    });

    it('words a code from the lookup of the place when it is given one, and from the general texts when the lookup has none', () => {
        const lookup = (code: string) => (code === 'key_refused' ? 'accounts.refusal.key_refused' : null);

        expect(refusalKey('key_refused', lookup)).toBe('accounts.refusal.key_refused');
        expect(fieldRefusalKey('key_refused', lookup)).toBe('accounts.refusal.key_refused');
        expect(refusalKey('network', lookup)).toBe('errors.network');
        expect(fieldRefusalKey('never_heard_of_it', lookup)).toBe('errors.invalid');
    });
});

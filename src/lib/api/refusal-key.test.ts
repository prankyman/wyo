import { describe, expect, it } from 'vitest';
import { fieldRefusalKey, refusalKey } from './refusal-key';

describe('the words of a refusal', () => {
    it('words a code from messages/ and falls back to a general text for a code nobody wrote one for', () => {
        expect(refusalKey('network')).toBe('errors.network');
        expect(refusalKey('never_heard_of_it')).toBe('errors.unknown');
    });

    it('words the refusal of one field the same way, with its own general text', () => {
        expect(fieldRefusalKey('invalid')).toBe('errors.invalid');
        expect(fieldRefusalKey('never_heard_of_it')).toBe('errors.invalid');
    });
});

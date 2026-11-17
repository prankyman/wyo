import { describe, expect, it } from 'vitest';
import { messages } from './messages';
import { tError } from './form-error';

const key = 'errors.invalid';
const translated = messages.ru[key];

describe('tError', () => {
    it('translates a bare message key', () => {
        expect(tError(key, 'ru')).toBe(translated);
    });

    it('translates the first key of an array - superforms’ usual shape', () => {
        expect(tError([key, 'errors.unknown'], 'ru')).toBe(translated);
    });

    it('translates the first key of a nested _errors object', () => {
        expect(tError({ _errors: [key] }, 'ru')).toBe(translated);
    });

    it('returns an empty string for the absent-error cases', () => {
        expect(tError(undefined, 'ru')).toBe('');
        expect(tError(null, 'ru')).toBe('');
        expect(tError([], 'ru')).toBe('');
        expect(tError({ _errors: [] }, 'ru')).toBe('');
        expect(tError({}, 'ru')).toBe('');
    });

    it('ignores a non-string array entry rather than rendering [object Object]', () => {
        expect(tError([{ nested: true }], 'ru')).toBe('');
    });

    it('interpolates params into the translated message', () => {
        expect(tError('start.version', 'ru', { version: '1.2.3' })).toBe(
            messages.ru['start.version'].replace('{version}', '1.2.3')
        );
    });

    it('passes an unknown key through untranslated rather than blanking the field', () => {
        expect(tError('some.unmapped.key', 'ru')).toBe('some.unmapped.key');
    });
});

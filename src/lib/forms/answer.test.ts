import { t } from '$lib/i18n/i18n';
import { describe, expect, it } from 'vitest';
import { answerLines, answerOf, answerOutcome, answerValue, firstError } from './answer';

describe('answerOf', () => {
    it('reads a success and a refusal, and nothing else', () => {
        expect(answerOf({ saved: 'added', count: 2 })).toEqual({ saved: 'added', count: 2 });
        expect(answerOf({ error: 'errors.invalid' })).toEqual({ error: 'errors.invalid' });
        expect(answerOf(undefined)).toBeUndefined();
        expect(answerOf('text')).toBeUndefined();
        expect(answerOf({ other: true })).toBeUndefined();
    });
});

describe('answerLines', () => {
    it('words a refusal from its key with its values', () => {
        expect(answerLines({ error: 'errors.invalid' }, 'ru', 'Сохранено')).toEqual({
            saved: null,
            error: 'Значение не подходит.'
        });
    });

    it('takes a fixed success text', () => {
        expect(answerLines({ saved: 'added' }, 'ru', 'Сохранено')).toEqual({ saved: 'Сохранено', error: null });
    });

    it('lets the page word a success from the answer', () => {
        const lines = answerLines({ saved: 'rebuild', count: 3 }, 'ru', (answer) => `${answer.saved}:${answer.count}`);
        expect(lines).toEqual({ saved: 'rebuild:3', error: null });
    });

    it('draws nothing before the form has answered', () => {
        expect(answerLines(undefined, 'ru', 'Сохранено')).toEqual({ saved: null, error: null });
    });
});

describe('answerValue', () => {
    it.each([
        [{ saved: 'scan', value: { lineId: 'l-1' } }, { lineId: 'l-1' }],
        [{ saved: 'scan', value: 'text' }, null],
        [{ saved: 'scan' }, null],
        [null, null]
    ])('reads the object an answer carries: %j → %j', (message, expected) => {
        expect(answerValue(message)).toEqual(expected);
    });
});

describe('answerOutcome', () => {
    it('words a success and a refusal as one line each, from its own key', () => {
        expect(answerOutcome({ saved: 'errors.invalid' }, 'ru')).toEqual({
            kind: 'saved',
            text: t('errors.invalid', 'ru')
        });
        expect(answerOutcome({ error: 'errors.not_found' }, 'ru')).toEqual({
            kind: 'error',
            text: t('errors.not_found', 'ru')
        });
    });

    it('answers null when there is no answer', () => {
        expect(answerOutcome(undefined, 'ru')).toBeNull();
    });
});

describe('firstError', () => {
    it.each([
        ['a bare message', 'errors.invalid', 'errors.invalid'],
        ['the first of a list', ['errors.invalid', 'errors.unknown'], 'errors.invalid'],
        ['an empty list', [], undefined],
        ['the first non-empty entry of a list', [[], 'errors.unknown'], 'errors.unknown'],
        [
            'a field of an object, in the schema’s order',
            { name: ['errors.invalid'], other: ['errors.unknown'] },
            'errors.invalid'
        ],
        ['a field inside a list of rows', { lines: [{}, { qty: ['errors.unknown'] }] }, 'errors.unknown'],
        ['the form’s own _errors', { _errors: ['errors.network'] }, 'errors.network'],
        ['no message at all', { name: [], lines: [{}] }, undefined],
        ['nothing', undefined, undefined]
    ])('finds %s', (_case, errors, expected) => {
        expect(firstError(errors)).toBe(expected);
    });
});

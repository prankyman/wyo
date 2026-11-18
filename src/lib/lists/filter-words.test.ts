import { describe, expect, it } from 'vitest';
import { filtersInWords } from './filter-words';

describe('the filters in use, in words', () => {
    it('quotes each chip’s words and lists them, for the sentence of an empty filtered list', () => {
        expect(filtersInWords([{ label: 'Состояние: Новый' }, { label: 'С 01 нояб. 2026 г.' }], 'ru')).toBe(
            '«Состояние: Новый», «С 01 нояб. 2026 г.»'
        );
    });
});

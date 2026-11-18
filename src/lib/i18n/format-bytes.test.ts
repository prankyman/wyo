import { describe, expect, it } from 'vitest';
import { formatBytes } from './format-bytes';

describe('formatBytes', () => {
    it.each([
        [0, '0 Б'],
        [999, '999 Б'],
        [1024, '1 КБ'],
        [1536, '1,5 КБ'],
        [5_242_880, '5 МБ'],
        [1_288_490_189, '1,2 ГБ']
    ])('writes %s bytes as «%s» in the units of the language', (bytes, text) => {
        expect(formatBytes(bytes, 'ru')).toBe(text);
    });
});

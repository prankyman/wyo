import { describe, expect, it } from 'vitest';
import type { TLocale } from './types';
import { decimalComma, decimalText } from './format-number';

describe('the decimal comma', () => {
    it.each([
        ['2.5', '2,5'],
        [58.5, '58,5'],
        ['12', '12'],
        ['', '']
    ])('fills a box with %j as %j', (value, expected) => {
        expect(decimalComma(value)).toBe(expected);
    });

    it.each<[TLocale, string]>([
        ['ru', '58,5'],
        ['xx', '58.5']
    ])('writes 58.5 for a %s reader as %s', (lang, expected) => {
        expect(decimalText(58.5, lang)).toBe(expected);
    });
});

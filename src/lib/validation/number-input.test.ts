import { describe, expect, it } from 'vitest';
import { z } from 'zod';
import { NUMBER_RULES, numberField, readNumberText, type INumberRule } from './number-input';

const QUANTITY: INumberRule = { decimals: 3 };
const SHIFT: INumberRule = { decimals: 1, signed: true, min: -5, max: 5 };
const WHOLE: INumberRule = { decimals: 0, min: 0, max: 999_999 };

describe('readNumberText', () => {
    it.each([
        ['12', QUANTITY, '12'],
        ['12,5', QUANTITY, '12.5'],
        ['12.500', QUANTITY, '12.5'],
        [' 1 234,5 ', QUANTITY, '1234.5'],
        ['1\u00a0234', QUANTITY, '1234'],
        ['1\u202f234', QUANTITY, '1234'],
        [',5', QUANTITY, '0.5'],
        ['5,', QUANTITY, '5'],
        ['007', QUANTITY, '7'],
        ['0', QUANTITY, '0'],
        ['-2,5', SHIFT, '-2.5'],
        ['\u22122,5', SHIFT, '-2.5'],
        ['\u20132', SHIFT, '-2'],
        ['-0', SHIFT, '0'],
        ['5', SHIFT, '5'],
        ['-5', SHIFT, '-5'],
        ['999999', WHOLE, '999999']
    ])('reads «%s» as %s', (text, rule, value) => {
        expect(readNumberText(text, rule)).toEqual({ ok: true, value });
    });

    it.each([
        ['', QUANTITY, 'empty'],
        ['   ', QUANTITY, 'empty'],
        ['abc', QUANTITY, 'notANumber'],
        ['12a', QUANTITY, 'notANumber'],
        ['1e3', QUANTITY, 'notANumber'],
        ['0x10', QUANTITY, 'notANumber'],
        ['Infinity', QUANTITY, 'notANumber'],
        [',', QUANTITY, 'notANumber'],
        ['-', SHIFT, 'notANumber'],
        ['--1', SHIFT, 'notANumber'],
        ['+3', QUANTITY, 'notANumber'],
        ['1-', SHIFT, 'notANumber'],
        ['1,234.5', QUANTITY, 'multipleSeparators'],
        ['1,2,3', QUANTITY, 'multipleSeparators'],
        ['1,2345', QUANTITY, 'tooManyDecimals'],
        ['2,25', SHIFT, 'tooManyDecimals'],
        ['5,0', WHOLE, 'tooManyDecimals'],
        ['-1', QUANTITY, 'negative'],
        ['\u22121', WHOLE, 'negative'],
        ['-5,1', SHIFT, 'belowMin'],
        ['5,1', SHIFT, 'aboveMax'],
        ['1000000', WHOLE, 'aboveMax']
    ])('refuses «%s» (%o) as %s', (text, rule, reason) => {
        expect(readNumberText(text, rule)).toEqual({ ok: false, reason });
    });

    it('refuses a value at a bound only when the bound is exclusive (money and quantities «above 0»)', () => {
        const above = { decimals: 2, min: 0, minExclusive: true };
        expect(readNumberText('0', above)).toEqual({ ok: false, reason: 'belowMin' });
        expect(readNumberText('0,01', above)).toEqual({ ok: true, value: '0.01' });
    });

    it('reads a number handed by a service as its own text', () => {
        expect(readNumberText(12.5, QUANTITY)).toEqual({ ok: true, value: '12.5' });
        expect(readNumberText(-2, SHIFT)).toEqual({ ok: true, value: '-2' });
    });

    it('keeps money exact: no floating-point rounding on the way', () => {
        expect(readNumberText('99 999 999,99', NUMBER_RULES.money)).toEqual({ ok: true, value: '99999999.99' });
        expect(readNumberText('100 000 000,01', NUMBER_RULES.money)).toEqual({ ok: false, reason: 'aboveMax' });
        expect(readNumberText('0,1', NUMBER_RULES.money)).toEqual({ ok: true, value: '0.1' });
        expect(readNumberText('12,345', NUMBER_RULES.money)).toEqual({ ok: false, reason: 'tooManyDecimals' });
    });

    it('holds the quantity column: 11 whole digits and 3 places', () => {
        expect(readNumberText('99999999999,999', NUMBER_RULES.quantity)).toEqual({
            ok: true,
            value: '99999999999.999'
        });
        expect(readNumberText('100000000000', NUMBER_RULES.quantity)).toEqual({ ok: false, reason: 'aboveMax' });
    });
});

describe('numberField', () => {
    const schemaOf = (field: z.ZodType) => z.object({ amount: field });
    const issueMessage = (field: z.ZodType, amount: unknown) => {
        const result = schemaOf(field).safeParse({ amount });
        return result.success ? null : result.error.issues[0].message;
    };

    it('turns typed text into its canonical text, and refuses with the field key', () => {
        const field = numberField(SHIFT, 'shift.invalid');
        expect(schemaOf(field).parse({ amount: '-2,5' })).toEqual({ amount: '-2.5' });
        expect(issueMessage(field, '7')).toBe('shift.invalid');
        expect(issueMessage(field, '1e1')).toBe('shift.invalid');
    });

    it('gives a number when asked for one', () => {
        const field = numberField(SHIFT, 'shift.invalid', { as: 'number' });
        expect(schemaOf(field).parse({ amount: '-2,5' })).toEqual({ amount: -2.5 });
    });

    it.each([
        ['none', null],
        ['zero', '0'],
        [9100, '9100']
    ] as const)('reads a blank box by the field’s meaning of blank: %s', (blank, value) => {
        expect(schemaOf(numberField(WHOLE, 'k', { blank })).parse({ amount: '  ' })).toEqual({ amount: value });
    });

    it('reads a box missing from the post as a blank one', () => {
        expect(z.object({ amount: numberField(WHOLE, 'k', { blank: 'zero' }) }).parse({})).toEqual({ amount: '0' });
        expect(z.object({ amount: numberField(WHOLE, 'k') }).safeParse({}).error?.issues[0].message).toBe('k');
    });

    it('refuses a blank box when the field is required, the default', () => {
        expect(issueMessage(numberField(WHOLE, 'whole.required'), '')).toBe('whole.required');
        expect(issueMessage(numberField(WHOLE, 'whole.required'), undefined)).toBe('whole.required');
    });

    it('names a reason by its own key when the field has one, the field key otherwise', () => {
        const field = numberField(QUANTITY, { default: 'qty.invalid', tooManyDecimals: 'qty.places' });
        expect(issueMessage(field, '1,2345')).toBe('qty.places');
        expect(issueMessage(field, 'x')).toBe('qty.invalid');
    });

    it('accepts a number already read (a form built from stored values)', () => {
        expect(schemaOf(numberField(SHIFT, 'k', { as: 'number' })).parse({ amount: 2 })).toEqual({ amount: 2 });
    });
});

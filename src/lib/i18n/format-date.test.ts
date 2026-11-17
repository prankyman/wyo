import { describe, expect, it } from 'vitest';
import {
    DATE_FORMAT,
    DATE_FORMAT_LONG,
    DATE_TIME_FORMAT,
    formatDate,
    formatDateParts,
    formatDateTime,
    formatIsoDay
} from './format-date';

const iso = '2026-09-07T12:05:00.000Z';
const date = new Date(iso);

// `locale` is required: it used to default to the cold-start language, which printed a page's dates in the wrong one.
describe('format-date', () => {
    it('formats a date with the preset options, from an ISO string or a Date alike', () => {
        expect(formatDate(iso, 'ru')).toBe(date.toLocaleDateString('ru', DATE_FORMAT));
        expect(formatDate(date, 'ru')).toBe(formatDate(iso, 'ru'));
    });

    it('formats a date-time with the preset options', () => {
        expect(formatDateTime(iso, 'ru')).toBe(date.toLocaleString('ru', DATE_TIME_FORMAT));
    });

    it('takes the options it is given instead of the preset', () => {
        expect(formatDate(iso, 'ru', DATE_FORMAT_LONG)).toBe(date.toLocaleDateString('ru', DATE_FORMAT_LONG));
    });

    it('dates the same instant differently in two languages', () => {
        expect(formatDate(iso, 'en')).not.toBe(formatDate(iso, 'ru'));
    });

    it('cuts a date into the pieces the language writes, in its own order', () => {
        const kinds = (locale: string) =>
            formatDateParts(iso, locale, { day: '2-digit', month: '2-digit', year: 'numeric', timeZone: 'UTC' })
                .filter((part) => part.type !== 'literal')
                .map((part) => part.type);

        expect(kinds('ru')).toEqual(['day', 'month', 'year']);
        expect(kinds('en')).toEqual(['month', 'day', 'year']);
    });

    it('writes a calendar day the same in every time zone', () => {
        expect(formatIsoDay('2026-09-07', 'ru')).toBe(
            new Date('2026-09-07T00:00:00.000Z').toLocaleDateString('ru', { ...DATE_FORMAT, timeZone: 'UTC' })
        );
        expect(formatIsoDay('2026-09-07', 'ru')).toContain('07');
    });
});

/**
 * The one place dates are turned into text. The `locale` argument every caller passes is the LANGUAGE CODE of the page
 * (`lang` in a component), never a regional tag, never a ternary, never omitted: `locale-argument-policy.test.ts` fails the
 * build on all three.
 *
 * Presets name the option shapes in use. Pass `{}` to get Intl's own numeric default, which is what a bare
 * `toLocaleDateString(locale)` produces.
 */
export const DATE_FORMAT: Intl.DateTimeFormatOptions = { day: '2-digit', month: 'short', year: 'numeric' };
export const DATE_TIME_FORMAT: Intl.DateTimeFormatOptions = { ...DATE_FORMAT, hour: '2-digit', minute: '2-digit' };
/** `Sep 7, 2026`. */
export const DATE_FORMAT_NATURAL: Intl.DateTimeFormatOptions = { year: 'numeric', month: 'short', day: 'numeric' };
export const DATE_TIME_FORMAT_NATURAL: Intl.DateTimeFormatOptions = {
    ...DATE_FORMAT_NATURAL,
    hour: '2-digit',
    minute: '2-digit'
};
/** `7 September 2026`. */
export const DATE_FORMAT_LONG: Intl.DateTimeFormatOptions = { year: 'numeric', month: 'long', day: 'numeric' };

export function formatDate(
    value: string | Date,
    locale: string,
    options: Intl.DateTimeFormatOptions = DATE_FORMAT
): string {
    return new Date(value).toLocaleDateString(locale, options);
}

export function formatDateTime(
    value: string | Date,
    locale: string,
    options: Intl.DateTimeFormatOptions = DATE_TIME_FORMAT
): string {
    return new Date(value).toLocaleString(locale, options);
}

/**
 * The same text cut into its pieces - day, month, year, hour, minute, the AM/PM marker and the literal characters between
 * them - for code that must know the ORDER the language writes them in, not only the finished text.
 */
export function formatDateParts(
    value: string | Date,
    locale: string,
    options: Intl.DateTimeFormatOptions
): Intl.DateTimeFormatPart[] {
    return new Intl.DateTimeFormat(locale, options).formatToParts(new Date(value));
}

/**
 * A calendar day `YYYY-MM-DD` in the reader's language: «07 нояб. 2026 г.». The day is read at midnight UTC and written in
 * UTC, so no time zone can move it to the day before.
 */
export function formatIsoDay(day: string, locale: string): string {
    return formatDate(`${day}T00:00:00.000Z`, locale, { ...DATE_FORMAT, timeZone: 'UTC' });
}

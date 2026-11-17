import { z } from 'zod';

/**
 * One reading of a number a person typed (constitution, "Forms"). The browser's `NumberField` and the wyo server's checks
 * both read a typed box the same way, so a value the box lets through is the value the server stores, and the two never
 * disagree about «58,5».
 *
 * Pure module: it reaches for nothing that only a server or only a browser has.
 */

/** What a field accepts. Bounds are inclusive unless `minExclusive` says the lower one is not («above 0»). */
export interface INumberRule {
    /** Places after the separator the field keeps (0\u20134); more are refused, never cut. */
    decimals: number;
    /** A leading minus is accepted; the field shows «±» beside its box. */
    signed?: boolean;
    min?: number;
    max?: number;
    /** The value must be above `min`, not equal to it: a quantity or an amount of 0 means nothing was given. */
    minExclusive?: boolean;
}

/** Why a typed number was refused. */
export type TNumberRefusalEnum =
    'empty' | 'notANumber' | 'multipleSeparators' | 'tooManyDecimals' | 'negative' | 'belowMin' | 'aboveMax';

export type TNumberReading = { ok: true; value: string } | { ok: false; reason: TNumberRefusalEnum };

/**
 * The rule of each meaning of a number, so one meaning has one rule on every screen. A field
 * whose own bound differs spreads one of these and names the bound (`{ ...NUMBER_RULES.measure, max: 999_999 }`).
 */
export const NUMBER_RULES = {
    /** A quantity of things: 11 whole digits and 3 places at the most; above 0. */
    quantity: { decimals: 3, min: 0, minExclusive: true, max: 99_999_999_999.999 },
    /** A quantity where 0 is an answer (a count of nothing). */
    quantityOrZero: { decimals: 3, min: 0, max: 99_999_999_999.999 },
    /** Money a person types: kopecks, above 0, at most 100 000 000 ₽. */
    money: { decimals: 2, min: 0, minExclusive: true, max: 100_000_000 },
    /** A price: 10 whole digits and 4 places at the most. */
    price: { decimals: 4, min: 0, max: 9_999_999_999.9999 },
    /** Kilograms, centimetres, litres: 3 places, above 0. */
    measure: { decimals: 3, min: 0, minExclusive: true, max: 999_999_999.999 },
    /** A count of whole things: whole, 0\u2013999 999. */
    count: { decimals: 0, min: 0, max: 999_999 },
    /** «Порядок в списке»: whole, 0\u2013100 000. */
    sortOrder: { decimals: 0, min: 0, max: 100_000 },
    /** A network port. */
    port: { decimals: 0, min: 1, max: 65_535 }
} as const satisfies Record<string, INumberRule>;

/** Every kind of space a keyboard, a paste or a formatted number puts between the thousands. */
const SPACES = /[\s\u00a0\u202f]/g;

/** The minus signs a paste may carry: the hyphen, the true minus (U+2212) and the en dash (U+2013). */
const MINUS = /^[-\u2212\u2013]/;

/**
 * Reads what a person typed: spaces removed; «,» or «.» separates the places; one leading minus only on a signed field;
 * «,5» is 0.5 and «5,» is 5. Refused: a plus sign (a paste into `NumberField` drops a leading one), an exponent («1e3»), hexadecimal («0x10»), letters, two separators, more places
 * than the rule keeps, and a value outside its bounds. The answer is the canonical decimal text — a dot, no thousands
 * separators, no leading or trailing zeros — so money and quantities stay exact on their way to the wyo server.
 */
export function readNumberText(text: string | number, rule: INumberRule): TNumberReading {
    // A number or a stored decimal handed by a service or the seed is read as its own text: the reading is one for all.
    let compact = String(text ?? '').replace(SPACES, '');
    if (compact.length === 0) return { ok: false, reason: 'empty' };

    const negative = MINUS.test(compact);
    if (negative) compact = compact.slice(1);

    const separators = compact.match(/[.,]/g);
    if (separators !== null && separators.length > 1) return { ok: false, reason: 'multipleSeparators' };
    if (!/^\d*[.,]?\d*$/.test(compact) || !/\d/.test(compact)) return { ok: false, reason: 'notANumber' };

    const [wholePart, fractionPart = ''] = compact.split(/[.,]/);
    if (fractionPart.length > rule.decimals) return { ok: false, reason: 'tooManyDecimals' };

    const whole = wholePart.replace(/^0+(?=\d)/, '') || '0';
    const fraction = fractionPart.replace(/0+$/, '');
    const magnitude = fraction.length > 0 ? `${whole}.${fraction}` : whole;
    const zero = /^0(\.0*)?$/.test(magnitude);
    if (negative && !zero && !rule.signed) return { ok: false, reason: 'negative' };
    const value = negative && !zero ? `-${magnitude}` : magnitude;

    // At most 15 significant digits reach here (11 whole and 4 places at the most), which a JavaScript number holds
    // exactly enough to compare with a bound; the value itself stays text.
    const amount = Number(value);
    if (rule.min !== undefined && (amount < rule.min || (rule.minExclusive === true && amount === rule.min)))
        return { ok: false, reason: 'belowMin' };
    if (rule.max !== undefined && amount > rule.max) return { ok: false, reason: 'aboveMax' };
    return { ok: true, value };
}

/** What a blank box means: no value (`none`), zero (`zero`), a refusal (`required`), or a value of its own (a port's 9100). */
export type TNumberBlank = 'none' | 'zero' | 'required' | number;

/** The message keys of a field: one for every refusal, or one per reason with a default. */
export type TNumberMessages = string | ({ default: string } & Partial<Record<TNumberRefusalEnum, string>>);

interface INumberFieldOptions<TAs extends 'text' | 'number'> {
    blank?: TNumberBlank;
    /** `text` (the default) keeps the canonical decimal text, for a `Decimal` column; `number` turns it into a number. */
    as?: TAs;
}

type TBlankOf<TBlank> = TBlank extends 'none' ? null : never;

/**
 * The zod schema of a typed number: every schema of a number box uses it, in the browser (`zod4Client`) and in
 * any check of the same text, so the box and its check read the text the same way. A number
 * already read (a form filled from stored values) is accepted as its text.
 */
export function numberField<TAs extends 'text' | 'number' = 'text', TBlank extends TNumberBlank = 'required'>(
    rule: INumberRule,
    messages: TNumberMessages,
    options: INumberFieldOptions<TAs> & { blank?: TBlank } = {}
): z.ZodType<(TAs extends 'number' ? number : string) | TBlankOf<TBlank>, unknown> {
    const blank = options.blank ?? 'required';
    const keyOf = (reason: TNumberRefusalEnum) =>
        typeof messages === 'string' ? messages : (messages[reason] ?? messages.default);
    const out = (value: string) => (options.as === 'number' ? Number(value) : value);

    // `optional()` first: a box missing from the post altogether is read as a blank one, by the field's meaning of blank.
    return z
        .unknown()
        .optional()
        .transform((input, context) => {
            const text = typeof input === 'number' ? String(input) : typeof input === 'string' ? input : '';
            if (text.trim() === '' && blank !== 'required') {
                if (blank === 'none') return null;
                return out(blank === 'zero' ? '0' : String(blank));
            }
            const reading = readNumberText(text, rule);
            if (reading.ok) return out(reading.value);
            context.addIssue({ code: 'custom', message: keyOf(reading.reason), input });
            return z.NEVER;
        }) as unknown as z.ZodType<(TAs extends 'number' ? number : string) | TBlankOf<TBlank>, unknown>;
}

/**
 * What a field whose box may hold anything shows on its phone keyboard (constitution, "Forms"): digits only for a whole
 * number, digits and a separator otherwise. A signed field's minus comes from «±», because these keyboards may lack one
 * (https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Global_attributes/inputmode).
 */
export function inputModeOf(decimals: number): 'numeric' | 'decimal' {
    return decimals === 0 ? 'numeric' : 'decimal';
}

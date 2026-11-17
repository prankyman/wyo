/**
 * What `NumberField` lets into its box. Typing: digits; one «,» or «.» when the field keeps
 * places; a leading «-» when it accepts negatives; any other key types nothing. A paste is tidied — spaces (the
 * non-breaking one too) removed, the true minus (U+2212) and the en dash (U+2013) read as a minus on a signed field, a leading «+» dropped —
 * and whatever else it holds stays in the box, so the field's refusal names the problem: nothing a person typed or pasted
 * is changed behind their back, and more places than allowed are refused by the reading, never cut.
 */

interface ITypingRule {
    decimals: number;
    signed: boolean;
}

/** Whether typing `typed` over the selection `start`…`end` of `current` keeps the box a number in the making. */
export function acceptsTyped(current: string, typed: string, start: number, end: number, rule: ITypingRule): boolean {
    const outside = current.slice(0, start) + current.slice(end);
    let position = start;
    let hasSeparator = /[.,]/.test(outside);
    let hasMinus = outside.startsWith('-');
    for (const character of typed) {
        if (/\d/.test(character)) {
            if (position === 0 && hasMinus) return false;
        } else if (character === ',' || character === '.') {
            if (rule.decimals === 0 || hasSeparator) return false;
            hasSeparator = true;
        } else if (character === '-') {
            if (!rule.signed || position !== 0 || hasMinus) return false;
            hasMinus = true;
        } else return false;
        position += 1;
    }
    return true;
}

/** A pasted text, tidied as above; the rest is kept for the reading to judge. */
export function normalizePasted(text: string, rule: ITypingRule): string {
    const compact = text.replace(/[\s\u00a0\u202f]/g, '').replace(/^\+/, '');
    return rule.signed ? compact.replace(/^[\u2212\u2013]/, '-') : compact;
}

/** «±»: a leading minus comes off, or one goes on. */
export function flipSign(text: string): string {
    return text.startsWith('-') ? text.slice(1) : `-${text}`;
}

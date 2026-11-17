import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { ROOT, svelteFilesUnder } from '$test/support/svelte-markup';

/**
 * Every form built with `superForm` or `spaForm` states what happens to its boxes after a successful save:
 * `resetForm: true` or `resetForm: false` (or a function that answers).
 *
 * The library's own default is `resetForm: true` - "going back to the initial state of the form data", i.e. to what the page
 * had when the form was made (https://superforms.rocks/concepts/enhance#resetform). On a form that edits a record and stays
 * on the page, that put the old values back the moment a save succeeded, so a changed name jumped back to the old one until
 * the page was reloaded. A form that adds a record and wants empty boxes for the next one is fine with `true`; nothing tells
 * the two apart except the person writing the form, so the choice has to be written down where the form is made.
 *
 * A source-reading rule: fixing one form is a one-time job, this test is what stops the next one appearing.
 */

/** Removes comments, so a `)` or the word `resetForm` inside one cannot fool the scan below. */
function withoutComments(source: string): string {
    return source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|\s)\/\/.*$/gm, '$1');
}

/** The text between the parentheses of every `superForm(` and `spaForm(` call in a source. */
function formCalls(source: string): string[] {
    const code = withoutComments(source);
    const calls: string[] = [];
    for (const match of code.matchAll(/\b(?:superForm|spaForm)\(/g)) {
        let depth = 1;
        let end = match.index + match[0].length;
        while (end < code.length && depth > 0) {
            if (code[end] === '(') depth++;
            else if (code[end] === ')') depth--;
            end++;
        }
        calls.push(code.slice(match.index + match[0].length, end - 1));
    }
    return calls;
}

/**
 * Whether a call states `resetForm`, in its own text or through an options helper of the same source that it names
 * (`shared('edit-name')`, `{ id, ...nameForm }`): the helper's definition then carries it for every call.
 */
function statesReset(call: string, code: string): boolean {
    if (/\bresetForm\s*:/.test(call)) return true;
    return [...new Set(call.match(/[A-Za-z_$][\w$]*/g) ?? [])].some((name) => {
        const definition = new RegExp(`(?:const|let|function)\\s+${name.replace('$', '\\$')}\\b`).exec(code);
        return definition !== null && /\bresetForm\s*:/.test(code.slice(definition.index, definition.index + 600));
    });
}

/** The forms of a source that leave `resetForm` out. */
function formsWithoutReset(source: string): number {
    const code = withoutComments(source);
    return formCalls(code).filter((call) => !statesReset(call, code)).length;
}

describe('superForm reset policy: every form says what a successful save does to its boxes', () => {
    it('finds a call that leaves resetForm out, and accepts the ways of saying it', () => {
        expect(formsWithoutReset("const f = superForm(data, { id: 'a' });")).toBe(1);
        expect(formsWithoutReset("const f = spaForm(schema, {}, { id: 'a' });")).toBe(1);
        expect(formsWithoutReset("const f = superForm(data, { id: 'a', resetForm: false });")).toBe(0);
        expect(formsWithoutReset('const f = spaForm(schema, {}, { resetForm: () => true });')).toBe(0);
        expect(
            formsWithoutReset('const shared = { resetForm: false };\nconst f = superForm(data, { ...shared });')
        ).toBe(0);
        expect(formsWithoutReset('// superForm(data, {})\n/* spaForm(schema) */\nconst f = 1;')).toBe(0);
    });

    it('has no form in the interface that leaves resetForm out', () => {
        // `src/test` holds harnesses that exist to drive the library, not forms of the interface.
        const violations = svelteFilesUnder('src')
            .filter((file) => !file.startsWith('src/test/'))
            .filter((file) => formsWithoutReset(readFileSync(`${ROOT}/${file}`, 'utf8')) > 0)
            .map((file) => `${file} builds a form without saying what a save does to its boxes (resetForm)`);

        expect(violations, violations.join('\n')).toEqual([]);
    });
});

import { afterEach, describe, expect, it } from 'vitest';
import { focusFirstRefusedField } from './collapsible-section';

/**
 * Where a refusal puts the cursor: the first refused box of the page, or — for a section answering its
 * own save — the first refused box of that section, found inside it even when another element of the page has the same id.
 */

const flushMicrotask = () => new Promise<void>((resolve) => queueMicrotask(resolve));

afterEach(() => {
    document.body.innerHTML = '';
});

describe('focusFirstRefusedField', () => {
    it('puts the cursor in the first refused box of the page, the box its label names', async () => {
        document.body.innerHTML = `
            <div data-field-error><label for="a">A</label><button>?</button><input id="a" /></div>
            <div data-field-error><label for="b">B</label><input id="b" /></div>`;

        focusFirstRefusedField();
        await flushMicrotask();

        expect(document.activeElement?.id).toBe('a');
    });

    it('keeps a section to its own refused box, even when the page has a refused box before it and the same id elsewhere', async () => {
        document.body.innerHTML = `
            <div data-field-error><label for="other">Other</label><input id="other" /></div>
            <input id="inn" data-test="decoy" />
            <section data-test="own">
                <div data-field-error><label for="inn">ИНН</label><input id="inn" data-test="own-inn" /></div>
            </section>`;
        const own = document.querySelector<HTMLElement>('[data-test="own"]')!;

        focusFirstRefusedField(document, own);
        await flushMicrotask();

        expect(document.activeElement?.getAttribute('data-test')).toBe('own-inn');
    });

    it('takes the first box of a refused field whose label names no box, and leaves a cursor already in the field alone', async () => {
        document.body.innerHTML = `
            <div data-field-error><label>Plain</label><input type="hidden" /><select id="s"><option>1</option></select></div>`;
        focusFirstRefusedField();
        await flushMicrotask();
        expect(document.activeElement?.id).toBe('s');

        document.body.innerHTML = `
            <div data-field-error><label for="x">X</label><input id="x" /><input id="y" /></div>`;
        document.getElementById('y')!.focus();
        focusFirstRefusedField();
        await flushMicrotask();
        expect(document.activeElement?.id).toBe('y');
    });

    it('does nothing when nothing is refused', async () => {
        document.body.innerHTML = '<input id="z" />';
        focusFirstRefusedField();
        await flushMicrotask();
        expect(document.activeElement).toBe(document.body);
    });
});

import { describe, expect, it } from 'vitest';
import { changedSince, disclosureFormOf, fieldValues, firstField } from './disclosure-form';

/**
 * How a form opened by a press tells a change from none: it reads the page's own fields, so every
 * kind of field counts the same way — boxes, tick boxes, lists, text areas, files — and a field added later counts only
 * when it holds something. `DisclosureForm.svelte.test.ts` covers the component around it.
 */

function formOf(html: string): HTMLElement {
    const root = document.createElement('div');
    root.innerHTML = html;
    document.body.replaceChildren(root);
    return root;
}

describe('the change reader of a form opened by a press', () => {
    it('reads every kind of named field, and leaves the hidden values and the buttons out', () => {
        const root = formOf(`
            <input name="id" type="hidden" value="p1" />
            <input name="name" value="Zebra" />
            <input name="active" type="checkbox" checked />
            <select name="dpi"><option value="203">203</option><option value="300" selected>300</option></select>
            <select name="days" multiple><option value="mo" selected>mo</option><option value="tu" selected>tu</option></select>
            <textarea name="note">first</textarea>
            <input name="photo" type="file" />
            <input name="go" type="submit" value="Save" />
        `);
        expect([...fieldValues(root).values()]).toEqual(['Zebra', 'on', '300', 'mo\ntu', 'first', '']);
    });

    it('sees a changed list, text area or multiple choice, and nothing when all is as it was', () => {
        const root = formOf(`
            <select name="dpi"><option value="203">203</option><option value="300">300</option></select>
            <select name="days" multiple><option value="mo">mo</option><option value="tu">tu</option></select>
            <textarea name="note">first</textarea>
        `);
        const before = fieldValues(root);
        expect(changedSince(before, root)).toBe(false);

        (root.querySelector('textarea') as HTMLTextAreaElement).value = 'second';
        expect(changedSince(before, root)).toBe(true);
        (root.querySelector('textarea') as HTMLTextAreaElement).value = 'first';
        (root.querySelector('select[multiple] option') as HTMLOptionElement).selected = true;
        expect(changedSince(before, root)).toBe(true);
    });

    it('counts an added field only when it holds something other than what it was drawn with; a removed one never', () => {
        const root = formOf(`<input name="a" value="1" />`);
        const before = fieldValues(root);

        root.insertAdjacentHTML(
            'beforeend',
            `<input name="b" value="5" /><textarea name="c"></textarea><select name="d"><option value="x">x</option></select>`
        );
        expect(changedSince(before, root)).toBe(false);
        root.querySelector('input[name="a"]')?.remove();
        expect(changedSince(before, root)).toBe(false);

        (root.querySelector('input[name="b"]') as HTMLInputElement).value = '6';
        expect(changedSince(before, root)).toBe(true);
    });

    it('puts the cursor in the first field a person can type into', () => {
        const root = formOf(`
            <input name="code" disabled />
            <input name="number" readonly />
            <input name="name" />
        `);
        expect(firstField(root)).toBe(root.querySelector('input[name="name"]'));
    });

    it('finds no form around code that runs outside a component', () => {
        expect(disclosureFormOf()).toBeUndefined();
    });
});

import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Enforces "Forms", rule 5: every one-line text box of the interface is `TextBox`, every drop-down `SelectBox`,
 * every text area `Textarea`, so the look of a box is decided once — in `field-classes.ts` — and tested once, in the
 * three components' own tests. In the maintainer's other repository the same class string had once been declared as `inputClass`, `fieldClass`,
 * `selectClass` and five other names in 226 files, in about a dozen variants, over 865 bare tags.
 *
 * It fails:
 * 1. a declaration of a local look — `const inputClass | fieldClass | FIELD_CLASS | selectClass | labelClass |
 *    errorClass | boxClass | noticeClass` — anywhere under `src/` (the shared looks are imported from
 *    `field-classes.ts`);
 * 2. a bare `<select`, `<textarea` or `<input` in a `.svelte` file outside the controls themselves and the test
 *    harnesses. A tick box, a radio, a hidden value and a slider stay raw `<input>` tags: they have
 *    no shared box to decide;
 * 3. a file box anywhere but `FilePicker`, which offers the camera AND the gallery on a phone.
 *
 * Comments are not read, so a tag named in a comment is not one.
 */

const ROOT = path.resolve(__dirname, '../../../..');
const SRC = path.join(ROOT, 'src');

/** The input types that stay raw tags; `range` is a slider, which may stand beside its number box. */
const RAW_TYPES = new Set(['hidden', 'checkbox', 'radio', 'file', 'range']);

const LOCAL_LOOK =
    /\bconst (inputClass|fieldClass|FIELD_CLASS|selectClass|labelClass|errorClass|boxClass|noticeClass)\b/;

/** The files that may draw a raw box, each with its reason. */
const RAW_BOX_FILES: Record<string, string> = {
    'src/lib/components/ui/TextBox.svelte': 'The text box itself.',
    'src/lib/components/ui/SelectBox.svelte': 'The drop-down itself.',
    'src/lib/components/ui/Textarea.svelte': 'The text area itself.',
    'src/lib/components/ui/NumberField.svelte':
        'The number box: it reads numbers as it is typed («Forms», rule 6) and keeps its own box for that.'
};

function walk(dir: string, out: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) walk(full, out);
        else if (/\.(svelte|ts)$/.test(entry.name)) out.push(full);
    }
    return out;
}

const FILES = walk(SRC).map((file) => ({
    file: path.relative(ROOT, file).split(path.sep).join('/'),
    source: fs.readFileSync(file, 'utf8')
}));

/** The source without its comments: HTML comments in markup, block and line comments in scripts. */
function withoutComments(source: string): string {
    return source
        .replace(/<!--[\s\S]*?-->/g, '')
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/(^|[^:'"`\\])\/\/[^\n]*/g, '$1');
}

/** The bare boxes of a markup: every `<select`, `<textarea`, and `<input` of a type that is not raw, with its line. */
function bareBoxes(source: string): { tag: string; line: number }[] {
    const clean = withoutComments(source);
    const found: { tag: string; line: number }[] = [];
    for (const match of clean.matchAll(/<(input|select|textarea)\b((?:[^>{]|\{[^}]*\})*)>/g)) {
        const [, tag, attributes] = match;
        if (tag === 'input') {
            const type = /\btype="([a-z-]+)"/.exec(attributes)?.[1];
            if (type && RAW_TYPES.has(type)) continue;
        }
        found.push({ tag, line: clean.slice(0, match.index).split('\n').length });
    }
    return found;
}

describe('the form controls', () => {
    it('declare no local look anywhere under src/: the looks are imported from field-classes.ts', () => {
        const local = FILES.filter(({ file }) => !file.endsWith('form-control-policy.test.ts'))
            .filter(({ source }) => LOCAL_LOOK.test(withoutComments(source)))
            .map(({ file }) => file);

        expect(local).toEqual([]);
    });

    it('draw every text box, drop-down and text area through TextBox, SelectBox and Textarea', () => {
        const bare = FILES.filter(({ file }) => file.endsWith('.svelte'))
            .filter(({ file }) => !file.startsWith('src/test/') && !(file in RAW_BOX_FILES))
            .flatMap(({ file, source }) => bareBoxes(source).map(({ tag, line }) => `${file}:${line} <${tag}>`));

        expect(bare).toEqual([]);
    });

    it('draw every file field through FilePicker: the camera and the gallery on every phone', () => {
        const raw = FILES.filter(({ file }) => file.endsWith('.svelte'))
            .filter(({ file }) => !file.startsWith('src/test/') && file !== 'src/lib/components/ui/FilePicker.svelte')
            .filter(({ source }) => /<input\b(?:[^>{]|\{[^}]*\})*\btype="file"/.test(withoutComments(source)))
            .map(({ file }) => file);

        expect(raw).toEqual([]);
    });

    it('keep the exceptions list true: every listed file exists and still draws a raw box', () => {
        for (const file of Object.keys(RAW_BOX_FILES)) {
            const source = FILES.find((entry) => entry.file === file)?.source;
            expect(source, `${file} is listed but does not exist`).toBeDefined();
            expect(bareBoxes(source ?? '').length, `${file} draws no raw box any more`).toBeGreaterThan(0);
        }
    });
});

describe('the scan of this test', () => {
    it('finds selects, text areas and typed inputs, skips raw types and ignores comments', () => {
        const found = bareBoxes(
            [
                '<!-- <select> in a comment -->',
                '<input type="hidden" name="id" />',
                '<input type="checkbox" bind:checked />',
                '<input name="city" value={city} />',
                '<input type="date" min={today} />',
                '<select name="kind">',
                '<textarea rows="3"></textarea>',
                '<TextBox name="city" />'
            ].join('\n')
        );

        expect(found).toEqual([
            { tag: 'input', line: 4 },
            { tag: 'input', line: 5 },
            { tag: 'select', line: 6 },
            { tag: 'textarea', line: 7 }
        ]);
    });
});

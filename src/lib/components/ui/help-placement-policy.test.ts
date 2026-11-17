import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { svelteFilesUnder } from '$test/support/svelte-markup';

/**
 * A «?» sits immediately to the right of the words it explains — a label, a heading, a sentence — on the same line, never
 * on a line of its own under them (a «?» of a section drawn under the section's heading as the first thing inside the
 * section is the defect this check was written for; constitution, help texts, placement). The components that hold a «?» place it right: `Field` beside its label,
 * `PageHeading` and `HeadingWithHelp` beside their heading, `CollapsibleSection` (and `SectionForm`) in
 * their heading row through `help`, `DisclosureWithHelp` beside its button. This check reads every `.svelte` file under
 * `src/` and fails a hand-placed `<HelpTooltip>` that
 *
 * 1. is the first thing inside its parent — a «?» with nothing to its left (a `<div class="flex"><HelpTooltip/></div>`
 *    as the first child of a section);
 * 2. follows a closing heading tag (`</h1>` … `</h6>`) — the heading and its «?» are `PageHeading` or `HeadingWithHelp`;
 * 3. follows a closing block tag (`</p>`, `</div>`, `</dt>`, `</dd>`, `</label>`, `</legend>`, `</li>`) inside a parent that is not
 *    a flex or grid row: the block ends its line, so the «?» starts a new one under it.
 *
 * A `<HelpTooltip>` after text or an inline tag, or the first thing inside a flex or inline-flex parent that holds words
 * after it, is placed right and passes.
 */

const ROOT = process.cwd();

/** Files whose hand-placed «?» is right for a reason stated here. */
const EXCEPTIONS: ReadonlyArray<[file: string, reason: string]> = [];

const BLOCK_TAGS = new Set(['p', 'div', 'dt', 'dd', 'label', 'legend', 'li', 'section', 'ul', 'ol', 'table', 'form']);
/** Components that draw their own words first and their children right after them on the same line: a «?» first inside is beside the words. */
const INLINE_PARENTS = new Set(['StatusBadge']);
const VOID_TAGS = new Set([
    'br',
    'hr',
    'img',
    'input',
    'meta',
    'link',
    'area',
    'base',
    'col',
    'embed',
    'source',
    'track',
    'wbr'
]);

/**
 * The markup of a component: without `<script>`, `<style>`, HTML comments, the `heading` snippets (a snippet named `heading` is
 * drawn by `SlideOverPanel` and `ModalWindow` right after their title, so a «?» in it is beside the words by construction), and
 * with the other Svelte block tags taken out.
 */
function markupOf(source: string): string {
    return (
        source
            .replace(/<script[\s\S]*?<\/script>/g, '')
            .replace(/<style[\s\S]*?<\/style>/g, '')
            .replace(/<!--[\s\S]*?-->/g, '')
            .replace(/\{#snippet heading\([^)]*\)\}[\s\S]*?\{\/snippet\}/g, '')
            // A rendered snippet is words (a table head's label), so a «?» after it is beside them.
            .replace(/\{@render [^}]*\}/g, 'rendered')
            .replace(/\{[#:/@][^}]*\}/g, '')
    );
}

/** The class constants of the script (`const labelClass = '…'`), to read a parent's `class={labelClass}`. */
function classConstants(source: string): Map<string, string> {
    const constants = new Map<string, string>();
    for (const match of source.matchAll(/const\s+(\w+)\s*=\s*['"`]([^'"`]*)['"`]/g)) constants.set(match[1], match[2]);
    return constants;
}

interface IOpenTag {
    name: string;
    classes: string;
    /** Where the tag's `<` starts. */
    start: number;
    /** Where the tag's `>` ends: whatever follows is its first child. */
    end: number;
}

/** The tag that holds the position `at`, with its classes, or null at the top level. */
function parentOf(markup: string, at: number, constants: Map<string, string>): IOpenTag | null {
    const stack: IOpenTag[] = [];
    const tags = /<(\/?)([A-Za-z][\w:.-]*)((?:"[^"]*"|'[^']*'|\{[^}]*\}|[^>"'{])*)(\/?)>/g;
    for (const match of markup.matchAll(tags)) {
        const index = match.index ?? 0;
        if (index >= at) break;
        const [whole, closing, name, attributes, selfClosing] = match;
        if (closing) {
            for (let i = stack.length - 1; i >= 0; i--) {
                if (stack[i].name === name) {
                    stack.length = i;
                    break;
                }
            }
            continue;
        }
        if (selfClosing || whole.endsWith('/>') || VOID_TAGS.has(name.toLowerCase())) continue;
        const literal = attributes.match(/\bclass="([^"]*)"/)?.[1];
        const named = attributes.match(/\bclass=\{(\w+)\}/)?.[1];
        const classes = literal ?? (named ? (constants.get(named) ?? '') : '');
        stack.push({ name, classes, start: index, end: index + whole.length });
    }
    return stack[stack.length - 1] ?? null;
}

const isRow = (classes: string): boolean => /\b(inline-)?(flex|grid)\b/.test(classes);

/** What is wrong with the «?» at `at`, or nothing. */
function problemAt(markup: string, at: number, constants: Map<string, string>): string | null {
    const before = markup.slice(0, at).replace(/\s+$/, '');
    const parent = parentOf(markup, at, constants);
    if (parent && before.length <= parent.end) {
        // The «?» is the first thing inside its parent: nothing stands to its left. A row that holds words after it is fine,
        // and so is a component that draws its own words before its children.
        if (INLINE_PARENTS.has(parent.name)) return null;
        const rest = markup.slice(at);
        const closeOfParent = rest.indexOf(`</${parent.name}>`);
        const after = closeOfParent === -1 ? '' : rest.slice(rest.indexOf('>') + 1, closeOfParent);
        const words = after
            .replace(/<[^>]*>/g, '')
            .replace(/\{[^}]*\}/g, 'x')
            .trim();
        if (isRow(parent.classes) && words.length > 0) return null;
        // An inline wrapper around the «?» alone (a `no-print` span, an inline-block box) that itself follows words or an
        // element: the «?» still stands right of them.
        const inlineWrapper = parent.name === 'span' || /\binline(?:-block|-flex)?\b/.test(parent.classes);
        const grand = parentOf(markup, parent.start, constants);
        const beforeParent = markup.slice(0, parent.start).replace(/\s+$/, '');
        if (inlineWrapper && (grand ? beforeParent.length > grand.end : beforeParent.length > 0)) return null;
        return `the «?» is the first thing inside <${parent.name}> — nothing stands to its left`;
    }
    const closed = before.match(/<\/([A-Za-z][\w-]*)>$/)?.[1]?.toLowerCase();
    if (!closed) return null;
    if (/^h[1-6]$/.test(closed))
        return `the «?» follows </${closed}> — the heading and its «?» are PageHeading or HeadingWithHelp`;
    if (BLOCK_TAGS.has(closed) && !(parent && isRow(parent.classes)))
        return `the «?» follows </${closed}> inside <${parent?.name ?? 'the component'}>, which is not a flex or grid row — it starts a line of its own`;
    return null;
}

/** Every wrongly placed «?» of one file: `line: problem`. */
export function misplacedHelp(source: string): string[] {
    const markup = markupOf(source);
    const constants = classConstants(source);
    const problems: string[] = [];
    for (const match of markup.matchAll(/<HelpTooltip\b/g)) {
        const at = match.index ?? 0;
        const problem = problemAt(markup, at, constants);
        if (problem) problems.push(`${markup.slice(0, at).split('\n').length}: ${problem}`);
    }
    return problems;
}

describe('a «?» stands right of the words it explains', () => {
    const allowed = new Set(EXCEPTIONS.map(([file]) => file));
    const files = svelteFilesUnder('src').filter((file) => !allowed.has(file));

    it('finds the files it checks', () => {
        expect(files.length).toBeGreaterThan(40);
    });

    it('lets no «?» stand alone, under a heading or under a block of its own', () => {
        const offenders = files.flatMap((file) => {
            const problems = misplacedHelp(fs.readFileSync(path.join(ROOT, file), 'utf8'));
            return problems.map((problem) => `${file}:${problem}`);
        });
        expect(offenders).toEqual([]);
    });

    it('names a reason for every exception', () => {
        for (const [file, reason] of EXCEPTIONS) {
            expect(reason, file).not.toBe('');
            expect(fs.existsSync(path.join(ROOT, file)), file).toBe(true);
        }
    });
});

import fs from 'node:fs';
import path from 'node:path';
import { parse } from 'svelte/compiler';

/**
 * The markup of a `.svelte` file as a tree of its elements and components, for the policy tests that check how a screen is
 * built (`form-buttons-policy`, `page-width-policy`, `section-form-policy`). Svelte's own parser reads the file, so an
 * arrow or a `<` inside an expression never breaks the reading the way a regular expression would. The blocks (`{#if}`,
 * `{#each}`, `{#await}`, `{#key}`, `{#snippet}`) are see-through: what a block holds counts as a child of the element around
 * the block, because in the page it is drawn there.
 */

export const ROOT = path.resolve(__dirname, '../../..');

export interface IMarkupNode {
    /** `element` for an HTML tag (`form`, `div`), `component` for a Svelte component (`GuardedForm`, `Button`). */
    kind: 'element' | 'component';
    name: string;
    /** The attributes written in the tag: a static value as written, an expression as its source text, a bare one `true`. */
    attrs: Record<string, string | true>;
    children: IMarkupNode[];
    /** The line of the tag in its file, for the failure messages. */
    line: number;
    /** Drawn by an `{#each}` around it: the page may hold it many times. */
    repeated?: boolean;
    /** The conditions of the `{#if}` blocks that draw it directly (their source text): the page shows it only when they hold. */
    shownBy?: string[];
    /** The name of the `{#snippet}` that draws it directly (`footer` of a window): where the snippet is rendered, it is drawn. */
    snippet?: string;
}

interface IAstAttribute {
    type: string;
    name?: string;
    value?: true | IAstValuePart | IAstValuePart[];
}

interface IAstValuePart {
    type: string;
    start: number;
    end: number;
    data?: string;
}

interface IAstNode {
    type: string;
    start: number;
    test?: { start: number; end: number };
    name?: string;
    attributes?: IAstAttribute[];
    fragment?: IAstFragment | null;
    consequent?: IAstFragment | null;
    alternate?: IAstFragment | null;
    body?: IAstFragment | null;
    fallback?: IAstFragment | null;
    pending?: IAstFragment | null;
    then?: IAstFragment | null;
    catch?: IAstFragment | null;
    expression?: { name?: string };
}

interface IAstFragment {
    nodes: IAstNode[];
}

const ELEMENT_TYPES = new Set(['RegularElement', 'SvelteElement']);
const BLOCK_PARTS = ['consequent', 'alternate', 'body', 'fallback', 'pending', 'then', 'catch', 'fragment'] as const;

function lineAt(source: string, offset: number): number {
    let line = 1;
    for (let index = 0; index < offset; index++) if (source.charCodeAt(index) === 10) line++;
    return line;
}

function attributeValue(source: string, value: IAstAttribute['value']): string | true {
    if (value === true || value === undefined) return true;
    const parts = Array.isArray(value) ? value : [value];
    return parts
        .map((part) => (part.type === 'Text' ? (part.data ?? '') : source.slice(part.start, part.end)))
        .join('');
}

function nodesOf(source: string, fragment: IAstFragment | null | undefined): IMarkupNode[] {
    if (!fragment) return [];
    return fragment.nodes.flatMap((node) => nodeOf(source, node));
}

function nodeOf(source: string, node: IAstNode): IMarkupNode[] {
    const isElement = ELEMENT_TYPES.has(node.type);
    if (isElement || node.type === 'Component') {
        const attrs: Record<string, string | true> = {};
        for (const attribute of node.attributes ?? []) {
            if (attribute.type === 'Attribute' && attribute.name)
                attrs[attribute.name] = attributeValue(source, attribute.value);
        }
        return [
            {
                kind: isElement ? 'element' : 'component',
                name: node.name ?? '',
                attrs,
                children: nodesOf(source, node.fragment),
                line: lineAt(source, node.start)
            }
        ];
    }
    // A block, or a node that draws nothing of its own: its contents stand in its place. What an `{#each}` draws is marked.
    if (node.type === 'IfBlock' && node.test) {
        const test = source.slice(node.test.start, node.test.end);
        const shown = nodesOf(source, node.consequent).map((inner) => ({
            ...inner,
            shownBy: [...(inner.shownBy ?? []), test]
        }));
        return [...shown, ...nodesOf(source, node.alternate)];
    }
    const contents = BLOCK_PARTS.flatMap((part) => nodesOf(source, node[part]));
    if (node.type === 'SnippetBlock' && node.expression?.name) {
        const name = node.expression.name;
        return contents.map((inner) => ({ ...inner, snippet: inner.snippet ?? name }));
    }
    return node.type === 'EachBlock' ? contents.map((inner) => ({ ...inner, repeated: true })) : contents;
}

/** The top-level nodes of a file's markup (its scripts and styles are not part of it). */
export function markupTree(source: string): IMarkupNode[] {
    const ast = parse(source, { modern: true }) as unknown as { fragment: IAstFragment };
    return nodesOf(source, ast.fragment);
}

/** What stands at the top of a file but is never drawn in the page's flow: the head, the window and the snippets declared there. */
const OUTSIDE_THE_FLOW = new Set(['SvelteHead', 'SvelteWindow', 'SvelteDocument', 'SvelteBody', 'SnippetBlock']);

/**
 * The outermost elements and components a file draws in the page (the stack policy): `markupTree` without the
 * `<svelte:head>`, `<svelte:window>`, `<svelte:document>` and `<svelte:body>` and without the snippets declared at the top
 * (they are drawn where they are rendered). The blocks are see-through, as in `markupTree`.
 */
export function rootMarkup(source: string): IMarkupNode[] {
    const ast = parse(source, { modern: true }) as unknown as { fragment: IAstFragment };
    return nodesOf(source, { nodes: ast.fragment.nodes.filter((node) => !OUTSIDE_THE_FLOW.has(node.type)) });
}

/**
 * The Svelte components a file imports, by the name it uses for each, as repository paths (`$lib/…` and `./…`); a package's
 * component is left out.
 */
export function importedComponentFiles(file: string, source: string): Map<string, string> {
    const found = new Map<string, string>();
    for (const [, name, from] of source.matchAll(/import\s+(\w+)\s+from\s+'([^']+\.svelte)'/g)) {
        if (from.startsWith('$lib/')) found.set(name, `src/lib/${from.slice(5)}`);
        else if (from.startsWith('.'))
            found.set(name, path.posix.normalize(path.posix.join(path.posix.dirname(file), from)));
    }
    return found;
}

/** Every node of the tree, depth first, each with the chain of nodes around it (outermost first). */
export function* walkMarkup(
    nodes: readonly IMarkupNode[],
    ancestors: readonly IMarkupNode[] = []
): Generator<{ node: IMarkupNode; ancestors: readonly IMarkupNode[] }> {
    for (const node of nodes) {
        yield { node, ancestors };
        yield* walkMarkup(node.children, [...ancestors, node]);
    }
}

/** The words of a `class` attribute (an expression inside it is read as text, so a literal class in it still counts). */
export function classWords(node: IMarkupNode): string[] {
    const value = node.attrs.class;
    return typeof value === 'string' ? value.split(/[\s{}'"`?:]+/).filter(Boolean) : [];
}

/** Every `.svelte` file under these folders (relative to the repository), as repository paths with forward slashes. */
export function svelteFilesUnder(...folders: string[]): string[] {
    const found: string[] = [];
    const visit = (dir: string) => {
        if (!fs.existsSync(dir)) return;
        for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
            const full = path.join(dir, entry.name);
            if (entry.isDirectory()) visit(full);
            else if (entry.name.endsWith('.svelte')) found.push(path.relative(ROOT, full).split(path.sep).join('/'));
        }
    };
    for (const folder of folders) visit(path.join(ROOT, folder));
    return found.sort();
}

/** The markup tree of one repository file. */
export function readMarkup(file: string): IMarkupNode[] {
    return markupTree(fs.readFileSync(path.join(ROOT, file), 'utf8'));
}

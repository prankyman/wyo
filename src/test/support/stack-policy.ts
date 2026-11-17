import fs from 'node:fs';
import path from 'node:path';
import {
    classWords,
    importedComponentFiles,
    markupTree,
    rootMarkup,
    ROOT,
    walkMarkup,
    type IMarkupNode
} from './svelte-markup';

/**
 * The rule of one vertical gap, read from the source (the maintainer's rule «gap-4 everywhere»):
 * every page and every stack of blocks is `Stack` (`src/lib/components/ui/Stack.svelte`, a flex column with `gap-4`), and no
 * block brings an outer margin of its own. `stack-policy.test.ts` runs it over every `.svelte` file; the screen sweeps measure
 * the same rule in the browser (`e2e/helpers/stack-gaps.ts`). Each finding is one line, `file:line what is wrong`.
 */

/** The components that draw a block of a page: a margin passed to one of them is that block's outer margin. */
export const BLOCK_COMPONENTS: readonly string[] = [
    'Stack',
    'CollapsibleSection',
    'FilterSection',
    'NoteBox',
    'DataList',
    'EmptyState',
    'PageHeading'
];

/**
 * The frames that hold the page's content in a Stack of their own (a sign-in frame, for example): what stands inside them in a
 * page is read like the blocks of a Stack. None exists yet; a plan that adds one lists it here.
 */
const FRAMES: readonly string[] = [];

/** An outer margin above or below: `mb-4`, `mt-2`, `my-6`, `-mt-1` (a variant such as `sm:` is split off by `classWords`). */
const OUTER_MARGIN = /^-?m[bty]-/;

/** What a Stack never takes in its classes: a gap of its own, a `space-y-*`, or a margin. */
const NOT_ON_A_STACK = /^(?:gap-|space-y-|-?m[bty]-)/;

/** How deep a page is followed through the components that draw its whole body. */
const MAX_DEPTH = 4;

const marginsOf = (node: IMarkupNode): string[] => classWords(node).filter((word) => OUTER_MARGIN.test(word));

/**
 * The margins and gaps of one file's markup: a Stack with a gap, a `space-y-*` or a margin of its own; an element or component
 * directly inside a Stack (the blocks around it see-through) with an outer margin; a block component given an outer margin.
 */
export function marginFindings(file: string, nodes: readonly IMarkupNode[]): string[] {
    const found = new Map<number, string>();
    for (const { node } of walkMarkup(nodes)) {
        if (node.kind !== 'component') continue;
        if (node.name === 'Stack' || FRAMES.includes(node.name)) {
            const refused = node.name === 'Stack' ? classWords(node).filter((word) => NOT_ON_A_STACK.test(word)) : [];
            if (refused.length > 0)
                found.set(
                    node.line,
                    `<Stack> takes ${refused.join(' ')}: its gap is the one 16 px and it has no margin`
                );
            for (const child of node.children) {
                const margins = marginsOf(child);
                if (margins.length > 0 && !found.has(child.line))
                    found.set(child.line, `<${child.name}> directly inside a Stack carries ${margins.join(' ')}`);
            }
        } else if (BLOCK_COMPONENTS.includes(node.name)) {
            const margins = marginsOf(node);
            if (margins.length > 0 && !found.has(node.line))
                found.set(node.line, `<${node.name}> is given ${margins.join(' ')}: a block brings no outer margin`);
        }
    }
    return [...found].sort(([a], [b]) => a - b).map(([line, problem]) => `${file}:${line} ${problem}`);
}

const readSource = (file: string): string => fs.readFileSync(path.join(ROOT, file), 'utf8');

/** The generic primitives: a page drawn as one of them (a form, a dialog) is not a page-body component. */
const PRIMITIVES = 'src/lib/components/ui/';

/**
 * The outermost markup of a page: every element or component it draws at the top must be a `Stack`, or a file of `allowed`, or —
 * when it is the page's only root — a component of the project that draws the page's whole body and follows the same rule
 * itself (followed through up to four components). A finding names the file where the fix belongs: the component's when the
 * page only draws it, the page's otherwise (a dialog beside the page's column goes inside it).
 */
export function rootFindings(file: string, allowed: Readonly<Record<string, string>>, depth = 0): string[] {
    const source = readSource(file);
    const imports = importedComponentFiles(file, source);
    const roots = rootMarkup(source);
    return roots.flatMap((root) => {
        if (root.kind === 'component' && root.name === 'Stack') return [];
        const target = root.kind === 'component' ? imports.get(root.name) : undefined;
        if (target !== undefined && target in allowed) return [];
        const pageBody =
            target !== undefined &&
            roots.length === 1 &&
            !target.startsWith(PRIMITIVES) &&
            depth < MAX_DEPTH &&
            fs.existsSync(path.join(ROOT, target));
        if (pageBody) return rootFindings(target, allowed, depth + 1);
        return [`${file}:${root.line} <${root.name}> stands at the top outside a Stack`];
    });
}

/** Every finding of one file: its margins, and its outermost markup when it is a page. */
export function stackFindings(file: string, allowed: Readonly<Record<string, string>>): string[] {
    if (file in allowed) return [];
    const margins = marginFindings(file, markupTree(readSource(file)));
    // `+page@.svelte` is a page too: the `@` only chooses which layout it stands in.
    return /\/\+page(@[^/]*)?\.svelte$/.test(file) ? [...rootFindings(file, allowed), ...margins] : margins;
}

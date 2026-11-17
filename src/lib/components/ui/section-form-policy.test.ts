import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { markupTree, ROOT, svelteFilesUnder, walkMarkup, type IMarkupNode } from '$test/support/svelte-markup';

/**
 * Enforces "Forms" — one form per section: each folded section has its own «Сохранить» and
 * «Закрыть», and sections are never nested in sections. On an edit screen every folded section that holds fields is a form of
 * its own (`SectionForm`, `src/lib/components/ui/SectionForm.svelte`); a create screen keeps one form over its sections,
 * because nothing is stored yet.
 *
 * It fails, in every `.svelte` file under `src/` (the test harnesses excepted):
 * 1. a form (`<form>`, `GuardedForm`) holding two or more folded sections — `CollapsibleSection` or a component that draws
 *    one — outside the create screens named in CREATE_SCREENS;
 * 2. a folded section drawn inside another: a section whose body holds a `CollapsibleSection` or a component that draws one.
 *
 * Which components draw a section is read from the files themselves (a static map: a file draws one when its markup holds
 * `CollapsibleSection` or a component that draws one), so a new wrapper is followed without a list to keep.
 */

const CREATE_SCREENS: Readonly<Record<string, string>> = {};

const SECTION = 'CollapsibleSection';
const OWN_FILES = new Set(['src/lib/components/ui/CollapsibleSection.svelte']);

const FILES = svelteFilesUnder('src').filter((file) => !file.startsWith('src/test/'));

/** The components a file imports, by the name its markup uses, resolved to repository paths. */
function importsOf(file: string, source: string): Map<string, string> {
    const imports = new Map<string, string>();
    for (const match of source.matchAll(/import\s+(\w+)\s+from\s+['"]([^'"]+\.svelte)['"]/g)) {
        const [, name, specifier] = match;
        const resolved = specifier.startsWith('$lib/')
            ? path.join('src/lib', specifier.slice('$lib/'.length))
            : specifier.startsWith('$test/')
              ? path.join('src/test', specifier.slice('$test/'.length))
              : path.join(path.dirname(file), specifier);
        imports.set(name, path.normalize(resolved).split(path.sep).join('/'));
    }
    return imports;
}

interface IParsed {
    tree: IMarkupNode[];
    imports: Map<string, string>;
}

const parsed = new Map<string, IParsed>(
    FILES.map((file) => {
        const source = fs.readFileSync(path.join(ROOT, file), 'utf8');
        return [file, { tree: markupTree(source), imports: importsOf(file, source) }];
    })
);

/** The files that draw a folded section: `CollapsibleSection` itself, and every file whose markup holds one of them. */
const drawsSection = new Set<string>(OWN_FILES);
for (let grew = true; grew;) {
    grew = false;
    for (const [file, { tree, imports }] of parsed) {
        if (drawsSection.has(file)) continue;
        const found = Array.from(walkMarkup(tree)).some(({ node }) => isSection(node, imports));
        if (found) {
            drawsSection.add(file);
            grew = true;
        }
    }
}

function isSection(node: IMarkupNode, imports: Map<string, string>): boolean {
    if (node.kind !== 'component') return false;
    if (node.name === SECTION) return true;
    const target = imports.get(node.name);
    return target !== undefined && drawsSection.has(target);
}

const isForm = (node: IMarkupNode) =>
    (node.kind === 'element' && node.name === 'form') || (node.kind === 'component' && node.name === 'GuardedForm');

/**
 * The sections a node holds, the nearest ones only (a section's own inner sections are rule 2's business); a section an
 * `{#each}` draws counts twice, because the page may hold it many times.
 */
function sectionsIn(nodes: IMarkupNode[], imports: Map<string, string>, repeated = false): IMarkupNode[] {
    return nodes.flatMap((node) => {
        const many = repeated || node.repeated === true;
        if (isSection(node, imports)) return many ? [node, node] : [node];
        return sectionsIn(node.children, imports, many);
    });
}

function formsWithSections(file: string): string[] {
    const { tree, imports } = parsed.get(file)!;
    return Array.from(walkMarkup(tree))
        .filter(({ node }) => isForm(node) && sectionsIn(node.children, imports).length >= 2)
        .map(({ node }) => `${file}:${node.line}`);
}

function nestedSections(file: string): string[] {
    const { tree, imports } = parsed.get(file)!;
    return Array.from(walkMarkup(tree))
        .filter(({ node }) => isSection(node, imports) && sectionsIn(node.children, imports).length > 0)
        .map(({ node }) => `${file}:${node.line}`);
}

// Every page of the interface is parsed: under a whole run's load that takes longer than a single test's 5 seconds.
describe(
    'section form policy ("Forms": one form per section, never a section inside a section)',
    { timeout: 60_000 },
    () => {
        it('reads the components that draw a section from the files, so the map cannot pass vacuously', () => {
            expect(drawsSection.has('src/lib/components/ui/SectionForm.svelte')).toBe(true);
            expect(drawsSection.has('src/lib/components/ui/FilterSection.svelte')).toBe(true);
            expect(drawsSection.size).toBeGreaterThanOrEqual(3);
        });

        it('holds no two folded sections in one form, but on the named create screens', () => {
            expect(FILES.filter((file) => !(file in CREATE_SCREENS)).flatMap(formsWithSections)).toEqual([]);
        });

        it('keeps no create-screen exception that matches nothing', () => {
            expect(Object.keys(CREATE_SCREENS).filter((file) => formsWithSections(file).length === 0)).toEqual([]);
        });

        it('draws no folded section inside another', () => {
            expect(FILES.flatMap(nestedSections)).toEqual([]);
        });

        it('finds both defects it guards against', () => {
            const imports = new Map<string, string>();
            const [form] = markupTree(
                '<GuardedForm><CollapsibleSection>{#if a}<input />{/if}</CollapsibleSection><CollapsibleSection><CollapsibleSection /></CollapsibleSection></GuardedForm>'
            );
            expect(sectionsIn(form.children, imports)).toHaveLength(2);
            expect(sectionsIn(form.children[1].children, imports)).toHaveLength(1);
        });
    }
);

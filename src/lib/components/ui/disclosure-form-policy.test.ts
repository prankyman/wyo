import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Enforces "Forms", rule 7: a form opened by a press is `DisclosureForm`
 * (`src/lib/components/ui/DisclosureForm.svelte`), so on every screen its opener disappears while it is open, «Закрыть»
 * stands beside its main button and asks before a change is dropped — tested once, in `DisclosureForm.svelte.test.ts`.
 *
 * It fails, in the markup of every `.svelte` file under `src/` (test harnesses excepted):
 * - a `<details>` whose body holds a form (`<form`, `<GuardedForm`, or a component whose name ends in `Form`) — sections
 *   are `CollapsibleSection`, which closes by its title row and is not counted;
 * - an `{#if}` block holding a form whose condition is a name that a press handler of the same file sets
 *   (`onclick={() => (adding = true)}`).
 * EXCEPTIONS lists the rest with their reasons; an entry that matches nothing fails.
 */

const ROOT = path.resolve(__dirname, '../../../..');
const OWN = new Set(['src/lib/components/ui/DisclosureForm.svelte', 'src/lib/components/ui/CollapsibleSection.svelte']);

interface IException {
    path: string;
    /** `details` or the name the `{#if}` tests. */
    opener: string;
    reason: string;
}

const EXCEPTIONS: IException[] = [];

function svelteFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) svelteFiles(full, acc);
        else if (entry.name.endsWith('.svelte')) acc.push(full);
    }
    return acc;
}

function relative(file: string): string {
    return path.relative(ROOT, file).split(path.sep).join('/');
}

function markupOf(source: string): string {
    return source
        .replace(/<script\b[\s\S]*?<\/script>/g, '')
        .replace(/<style\b[\s\S]*?<\/style>/g, '')
        .replace(/<!--[\s\S]*?-->/g, '');
}

const HOLDS_FORM = /<form\b|<GuardedForm\b|<[A-Z]\w*Form\b/;

/** The body of every block that opens at `open` and closes at the matching `close`, nested blocks counted. */
function blocks(markup: string, open: RegExp, close: RegExp): { head: string; body: string }[] {
    const found: { head: string; body: string }[] = [];
    const tokens = new RegExp(`${open.source}|${close.source}`, 'g');
    const stack: { head: string; start: number }[] = [];
    for (const match of markup.matchAll(tokens)) {
        if (new RegExp(`^(?:${open.source})$`).test(match[0])) stack.push({ head: match[0], start: match.index });
        else {
            const top = stack.pop();
            if (top) found.push({ head: top.head, body: markup.slice(top.start, match.index) });
        }
    }
    return found;
}

/** The names a press handler of the file sets: `onclick={() => (adding = true)}`, `onclick={() => (open = !open)}`. */
function pressedNames(markup: string): Set<string> {
    const names = new Set<string>();
    for (const match of markup.matchAll(/on(?:click|pointerdown)=\{[^}]*?\b([A-Za-z_$][\w$]*)\s*=(?!=)/g))
        names.add(match[1]);
    return names;
}

/** Every opener in the file that shows a form: `details`, or the name an `{#if}` tests. */
function openersOf(file: string): string[] {
    const markup = markupOf(fs.readFileSync(path.join(ROOT, file), 'utf8'));
    const openers: string[] = [];
    for (const { body } of blocks(markup, /<details\b/, /<\/details>/))
        if (HOLDS_FORM.test(body)) openers.push('details');
    const pressed = pressedNames(markup);
    for (const { head, body } of blocks(markup, /\{#if [^}]*\}/, /\{\/if\}/)) {
        const name = /^\{#if !?\s*([A-Za-z_$][\w$]*)/.exec(head)?.[1];
        if (name && pressed.has(name) && HOLDS_FORM.test(body.slice(head.length))) openers.push(name);
    }
    return openers;
}

function allOpeners(): { path: string; opener: string }[] {
    return svelteFiles(path.join(ROOT, 'src'))
        .map(relative)
        .filter((file) => !file.startsWith('src/test/') && !OWN.has(file))
        .flatMap((file) => openersOf(file).map((opener) => ({ path: file, opener })));
}

const key = (entry: { path: string; opener: string }) => `${entry.path} — ${entry.opener}`;

describe('disclosure form policy (Forms, rule 7)', () => {
    it('opens every form shown by a press through DisclosureForm', () => {
        const allowed = new Set(EXCEPTIONS.map(key));
        expect(
            allOpeners()
                .map(key)
                .filter((entry) => !allowed.has(entry))
        ).toEqual([]);
    });

    it('keeps no exception that matches nothing, and gives each its reason', () => {
        const found = new Set(allOpeners().map(key));
        expect(EXCEPTIONS.map(key).filter((entry) => !found.has(entry))).toEqual([]);
        expect(EXCEPTIONS.filter((entry) => entry.reason.length < 30)).toEqual([]);
    });
});

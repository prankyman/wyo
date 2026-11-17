import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Enforces "Press feedback and single submission" (constitution): every
 * button is a `Button`, every form a `GuardedForm`, so every pressable control shows that it was
 * pressed and nothing can be sent twice. A raw `<button>` has no pressed or working state and a raw
 * `<form>` accepts a second tap, which is exactly what a person on a phone produces when
 * the first tap shows no reaction.
 *
 * Scanned: the markup of every `.svelte` file under `src/routes` and `src/lib/components` — the
 * `<script>` and `<style>` blocks and HTML comments are left out, so prose mentioning `<button>`
 * does not count.
 *
 * A `<summary>` — the part of a `<details>` that opens it, pressed like a button — cannot be a
 * `Button`, so it must carry the pressed look itself: `PRESS_FEEDBACK` or `buttonClasses(…)` from
 * `press.ts`, written in its tag or in the script constant its `class={…}` names.
 *
 * ALLOWLIST is the complete record of the exceptions: a primitive that has to draw its own element.
 * Each entry says why. An entry for a file that no longer needs it fails the second test, so the
 * list cannot quietly outlive its reasons. Adding a screen here is not an exception — it is the
 * problem this test exists to catch.
 *
 * A control that cannot act — greyed out (`disabled`) or busy (`aria-disabled`) — shows no hover
 * and no pressed look. Every `hover:` and `active:` class (and a `has-[…:active]` one) in `press.ts`,
 * in the allowlisted primitives, and in the extra `class` a screen gives a `Button`, `ClickableCard`
 * or `ClickableRow` must sit behind the `pressable:` variant (`src/app.css`); a `:hover`/`:active`
 * rule in an allowlisted primitive's `<style>` carries the same `:not(:disabled, [aria-disabled='true'])`.
 */

const ROOT = path.resolve(__dirname, '../../../..');
const SCANNED = ['src/routes', 'src/lib/components'];

interface IException {
    path: string;
    reason: string;
}

const ALLOWLIST: IException[] = [
    {
        path: 'src/lib/components/ui/Button.svelte',
        reason: 'The primitive itself: it renders the one <button> every screen uses.'
    },
    {
        path: 'src/lib/components/ui/GuardedForm.svelte',
        reason: 'The primitive itself: it renders the one <form> every screen uses.'
    },
    {
        path: 'src/lib/components/ui/IconButton.svelte',
        reason: 'An icon with a hover tooltip drawn inside the button; carries PRESS_FEEDBACK and FOCUS_RING from press.ts.'
    },
    {
        path: 'src/lib/components/ui/Tabs.svelte',
        reason: 'A tab strip with its own underline look; pressed and focus states in its scoped CSS.'
    },
    {
        path: 'src/lib/components/ui/HelpTooltip.svelte',
        reason: 'A bare question-mark icon that toggles a portaled tooltip; carries PRESS_FEEDBACK and dims when pressed.'
    },
    {
        path: 'src/lib/components/ui/AttentionIcon.svelte',
        reason: 'A bare warning icon that toggles a portaled popup, like HelpTooltip; carries PRESS_FEEDBACK.'
    },
    {
        path: 'src/lib/components/ui/RemovableChip.svelte',
        reason: 'The chip of a picker or a filter, a button holding its words and the remove cross; carries PRESS_FEEDBACK.'
    },
    {
        path: 'src/lib/components/ui/toast/Toaster.svelte',
        reason: 'The close cross of a toast, coloured by the toast it sits on; carries PRESS_FEEDBACK.'
    }
];

function svelteFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) svelteFiles(full, acc);
        else if (entry.name.endsWith('.svelte')) acc.push(full);
    }
    return acc;
}

/** The markup only: scripts, styles and comments removed. */
function markupOf(source: string): string {
    return source
        .replace(/<script\b[\s\S]*?<\/script>/g, '')
        .replace(/<style\b[\s\S]*?<\/style>/g, '')
        .replace(/<!--[\s\S]*?-->/g, '');
}

interface IRawUse {
    path: string;
    buttons: number;
    forms: number;
}

function rawUses(): IRawUse[] {
    return SCANNED.flatMap((dir) => svelteFiles(path.join(ROOT, dir)))
        .map((file) => {
            const markup = markupOf(fs.readFileSync(file, 'utf8'));
            return {
                path: path.relative(ROOT, file).split(path.sep).join('/'),
                buttons: (markup.match(/<button[\s>/]/g) ?? []).length,
                forms: (markup.match(/<form[\s>/]/g) ?? []).length
            };
        })
        .filter((use) => use.buttons + use.forms > 0)
        .sort((a, b) => a.path.localeCompare(b.path));
}

/** What gives a raw element the pressed look (`press.ts`). */
const PRESS_CLASSES = /\bPRESS_FEEDBACK\b|\bbuttonClasses\(/;

/**
 * Whether a `<summary>` opening tag carries the pressed look: the tag names it, or its
 * `class={name}` points at a script constant that does.
 */
function summaryIsPressable(tag: string, script: string): boolean {
    if (PRESS_CLASSES.test(tag)) return true;
    const name = tag.match(/\bclass=\{\s*([A-Za-z_$][\w$]*)\s*\}/)?.[1];
    if (!name) return false;
    const declaration = script.match(new RegExp(`\\b(?:const|let)\\s+${name}\\b[^=]*=([\\s\\S]*?);`))?.[1];
    return declaration !== undefined && PRESS_CLASSES.test(declaration);
}

/** `<summary>` tags without the pressed look, per file. */
function unpressedSummaries(): string[] {
    return SCANNED.flatMap((dir) => svelteFiles(path.join(ROOT, dir)))
        .flatMap((file) => {
            const source = fs.readFileSync(file, 'utf8');
            const script = (source.match(/<script\b[\s\S]*?<\/script>/g) ?? []).join('\n');
            // An attribute value in braces may hold a `>`, so a tag runs to the first `>` outside braces.
            const tags = markupOf(source).match(/<summary\b(?:[^>{]|\{[^}]*\})*>/g) ?? [];
            const relative = path.relative(ROOT, file).split(path.sep).join('/');
            return tags.filter((tag) => !summaryIsPressable(tag, script)).map((tag) => `${relative}: ${tag}`);
        })
        .sort();
}

const allowed = new Set(ALLOWLIST.map((entry) => entry.path));

/** Comments removed, so prose that quotes a class (`active:scale-[0.97]`) does not count. */
function withoutComments(source: string): string {
    return source
        .replace(/<!--[\s\S]*?-->/g, '')
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/(^|[^:'"`])\/\/[^\n]*/g, '$1');
}

/** A class split at its variant colons — the ones outside `[…]`: `pressable:has-[a:b]:x` → 3 parts. */
function variantParts(token: string): string[] {
    const parts: string[] = [];
    let depth = 0;
    let current = '';
    for (const char of token) {
        if (char === '[') depth++;
        if (char === ']') depth--;
        if (char === ':' && depth === 0) {
            parts.push(current);
            current = '';
        } else {
            current += char;
        }
    }
    parts.push(current);
    return parts;
}

/** A variant that styles the element itself while it is hovered or pressed. */
function isPressStateVariant(variant: string): boolean {
    return variant === 'hover' || variant === 'active' || /^has-\[.*:(active|hover)\b/.test(variant);
}

/**
 * The hover and pressed classes in `text` that a control which cannot act would still show: no
 * `pressable:` before the hover or pressed variant. `group-hover:` and `peer-*:` style another
 * element (a tooltip inside the button) and are not counted.
 */
function unguardedPressStyles(text: string): string[] {
    const tokens = text.match(/[^\s'"`{}()<>]+/g) ?? [];
    return tokens.filter((token) => {
        const parts = variantParts(token);
        const utility = parts[parts.length - 1];
        if (parts.length < 2 || !/^!?-?[a-z]/.test(utility)) return false;
        const variants = parts.slice(0, -1);
        const state = variants.findIndex(isPressStateVariant);
        return state !== -1 && !variants.slice(0, state).includes('pressable');
    });
}

function readRelative(relative: string): string {
    return fs.readFileSync(path.join(ROOT, relative), 'utf8');
}

function scriptOf(source: string): string {
    return (source.match(/<script\b[\s\S]*?<\/script>/g) ?? []).join('\n');
}

/**
 * The text a `class` attribute can draw from: the attribute itself, and — for every name it uses —
 * that name's declaration in the script, followed through the names that declaration uses in turn
 * (`class="{actionClass}"` → `palette.action` → `PALETTES`).
 */
function classSources(attribute: string, script: string): string {
    const texts = [attribute];
    const seen = new Set<string>();
    const pending = [...(attribute.match(/[A-Za-z_$][\w$]*/g) ?? [])];
    while (pending.length > 0) {
        const name = pending.pop() as string;
        if (seen.has(name)) continue;
        seen.add(name);
        const declaration = script.match(new RegExp(`\\b(?:const|let)\\s+${name}\\b[^=]*=([\\s\\S]*?);\\s*\\n`))?.[1];
        if (declaration === undefined) continue;
        texts.push(declaration);
        pending.push(...(declaration.match(/[A-Za-z_$][\w$]*/g) ?? []));
    }
    return texts.join('\n');
}

/** Components that take a screen's extra classes on top of their own hover and pressed looks. */
const PRESSABLE_COMPONENTS = /<(?:Button|ClickableCard|ClickableRow)\b(?:[^>{]|\{[^}]*\})*>/g;

/** Unguarded hover/pressed classes, per file: press.ts, the allowlisted primitives, and screen extras. */
function unguardedPressLooks(): string[] {
    const found: string[] = [];
    const report = (file: string, text: string) =>
        unguardedPressStyles(text).forEach((token) => found.push(`${file}: ${token}`));

    report('src/lib/components/ui/press.ts', withoutComments(readRelative('src/lib/components/ui/press.ts')));
    for (const entry of ALLOWLIST) {
        const source = withoutComments(readRelative(entry.path)).replace(/<style\b[\s\S]*?<\/style>/g, '');
        report(entry.path, source);
    }
    for (const file of SCANNED.flatMap((dir) => svelteFiles(path.join(ROOT, dir)))) {
        const relative = path.relative(ROOT, file).split(path.sep).join('/');
        const source = withoutComments(fs.readFileSync(file, 'utf8'));
        const script = scriptOf(source);
        for (const tag of markupOf(source).match(PRESSABLE_COMPONENTS) ?? []) {
            const attribute = tag.match(/\bclass=("[^"]*"|\{[^}]*\})/)?.[1];
            if (attribute) report(relative, classSources(attribute, script));
        }
    }
    return [...new Set(found)].sort();
}

/** `:hover`/`:active` rules in the allowlisted primitives' own CSS without the `pressable` guard. */
function unguardedPressRules(): string[] {
    const guard = /:not\(:disabled,\s*\[aria-disabled=['"]?true['"]?\]\)/;
    return ALLOWLIST.flatMap((entry) => {
        const styles = (readRelative(entry.path).match(/<style\b[\s\S]*?<\/style>/g) ?? []).join('\n');
        const selectors = withoutComments(styles).match(/[^{}]+(?=\{)/g) ?? [];
        return selectors
            .map((selector) => selector.trim())
            .filter((selector) => /:(hover|active)\b/.test(selector) && !guard.test(selector))
            .map((selector) => `${entry.path}: ${selector}`);
    });
}

describe('press feedback policy', () => {
    it('renders no raw <button> or <form> outside the shared primitives', () => {
        const offenders = rawUses().filter((use) => !allowed.has(use.path));

        const message = [
            `${offenders.length} file(s) still render a raw <button> or <form>:`,
            ...offenders.map((use) => `  ${use.path} — ${use.buttons} <button>, ${use.forms} <form>`),
            '',
            'Replace every <button> with Button (src/lib/components/ui/Button.svelte), every <form> with',
            'GuardedForm (src/lib/components/ui/GuardedForm.svelte), a card or row that opens a record with',
            'ClickableCard / ClickableRow + StretchedLink. How: docs/library-layout.md, "Pressable controls".',
            'Keep every data-test id exactly as it is. Only a primitive that must draw its own element goes',
            'into ALLOWLIST in src/lib/components/ui/press-feedback-policy.test.ts, with its reason.'
        ].join('\n');

        expect(
            offenders.map((use) => use.path),
            message
        ).toEqual([]);
    });

    it('draws every <summary> with the pressed look from press.ts', () => {
        const offenders = unpressedSummaries();

        const message = [
            `${offenders.length} <summary> element(s) have no pressed look:`,
            ...offenders.map((offender) => `  ${offender}`),
            '',
            'A <summary> is pressed like a button but cannot be a Button: give it PRESS_FEEDBACK and FOCUS_RING',
            'from src/lib/components/ui/press.ts, or buttonClasses(…) for a button look (an export toggle).'
        ].join('\n');

        expect(offenders, message).toEqual([]);
    });

    it('recognises a hover or pressed class that a greyed-out or busy control would still show', () => {
        expect(
            unguardedPressStyles(
                [
                    'hover:bg-ink-700 active:scale-[0.97] has-[[data-stretched-link]:active]:bg-ink-100',
                    "aria-disabled:active:scale-100 motion-safe:active:scale-[0.97] 'sm:hover:underline'"
                ].join(' ')
            )
        ).toEqual([
            'hover:bg-ink-700',
            'active:scale-[0.97]',
            'has-[[data-stretched-link]:active]:bg-ink-100',
            'aria-disabled:active:scale-100',
            'motion-safe:active:scale-[0.97]',
            'sm:hover:underline'
        ]);
        expect(
            unguardedPressStyles(
                [
                    'pressable:hover:bg-ink-700 motion-safe:pressable:active:scale-[0.97]',
                    'pressable:has-[[data-stretched-link]:active]:bg-ink-100 group-hover:opacity-100',
                    'class:active={current} active: hover disabled:opacity-50 [-webkit-tap-highlight-color:transparent]'
                ].join(' ')
            )
        ).toEqual([]);
    });

    it('holds every hover and pressed look back on a control that cannot act (pressable:)', () => {
        const offenders = unguardedPressLooks();

        const message = [
            `${offenders.length} hover or pressed class(es) would still change a greyed-out or busy control:`,
            ...offenders.map((offender) => `  ${offender}`),
            '',
            'Write it behind the pressable: variant (src/app.css): pressable:hover:bg-ink-700,',
            'motion-safe:pressable:active:scale-[0.97]. A disabled or aria-disabled control shows no hover and',
            'no pressed look (constitution, press feedback and single submission; docs/library-layout.md).'
        ].join('\n');

        expect(offenders, message).toEqual([]);
    });

    it("guards the allowlisted primitives' own :hover and :active CSS the same way", () => {
        expect(
            unguardedPressRules(),
            "add :where(:not(:disabled, [aria-disabled='true'])) before :hover/:active"
        ).toEqual([]);
    });

    it('keeps the pressable: variant refusing disabled, busy and opening controls', () => {
        const definition = readRelative('src/app.css').match(/@custom-variant pressable \((.*)\);/)?.[1] ?? '';

        expect(definition).toContain(":not(:disabled, [aria-disabled='true'])");
        expect(definition).toContain("[data-stretched-link][aria-disabled='true']");
    });

    it('keeps an allowlist entry only for a file that exists, still needs it, and says why', () => {
        const stillRaw = new Set(rawUses().map((use) => use.path));

        const stale = ALLOWLIST.filter((entry) => !stillRaw.has(entry.path)).map((entry) => entry.path);
        const unexplained = ALLOWLIST.filter((entry) => entry.reason.trim().length < 20).map((entry) => entry.path);

        expect(stale, 'allowlisted but no longer rendering a raw <button>/<form> — remove the entry').toEqual([]);
        expect(unexplained, 'an exception without a reason is not documented').toEqual([]);
    });
});

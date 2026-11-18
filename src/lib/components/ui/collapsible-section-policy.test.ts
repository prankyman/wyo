import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Enforces "Sections fold": the files that hold the sections of the folded
 * screens draw every titled section with
 * `CollapsibleSection`, so none of them falls back to a plain card with a heading. A file of the list that no longer uses
 * `CollapsibleSection`, or draws a raw `<h2>` or a visible `<legend>`, fails — unless the heading is one of the named
 * exceptions below, each with its reason. A NEW screen with two or more titled sections follows the same rule
 * (the constitution).
 */

const ROOT = path.resolve(__dirname, '../../../..');

/** The files holding the folded screens' sections: a screen with two or more titled sections adds its files here. */
const SECTION_FILES: readonly string[] = [
    'src/lib/components/settings/SettingsSection.svelte',
    'src/lib/components/settings/BackupsSection.svelte',
    'src/routes/(app)/account/+page.svelte'
];

/** The raw `<h2>` a file keeps on purpose, and why: none of them heads a section of the screen. */
const KEPT_HEADINGS: Readonly<Record<string, { count: number; reason: string }>> = {};

/** The markup only: scripts, styles and comments removed. */
function markupOf(source: string): string {
    return source
        .replace(/<script\b[\s\S]*?<\/script>/g, '')
        .replace(/<style\b[\s\S]*?<\/style>/g, '')
        .replace(/<!--[\s\S]*?-->/g, '');
}

const read = (file: string) => markupOf(fs.readFileSync(path.join(ROOT, file), 'utf8'));

/** The components that draw a titled section: the section itself and the section that is a form. */
const SECTION_TAG = /<(CollapsibleSection|SectionForm)[\s>]/;

describe('collapsible section policy (Screens, "Sections fold")', () => {
    it('keeps every listed file on CollapsibleSection', () => {
        // An edit screen's section that holds fields is a `SectionForm`, which draws a `CollapsibleSection`.
        expect(SECTION_FILES.filter((file) => !SECTION_TAG.test(read(file)))).toEqual([]);
    });

    it('draws no section heading outside CollapsibleSection, but for the named exceptions', () => {
        const offending = SECTION_FILES.flatMap((file) => {
            const markup = read(file);
            const headings = (markup.match(/<h2[\s>]/g) ?? []).length;
            const legends = (markup.match(/<legend\b[^>]*>/g) ?? []).filter((tag) => !tag.includes('sr-only')).length;
            const kept = KEPT_HEADINGS[file]?.count ?? 0;
            return headings !== kept || legends > 0
                ? [`${file}: ${headings} <h2> (kept ${kept}), ${legends} visible <legend>`]
                : [];
        });
        expect(offending).toEqual([]);
    });

    it('keeps no exception for a file off the list', () => {
        expect(Object.keys(KEPT_HEADINGS).filter((file) => !SECTION_FILES.includes(file))).toEqual([]);
    });
});

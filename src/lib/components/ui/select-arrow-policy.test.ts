import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * A select's arrow keeps its room.
 *
 * `@tailwindcss/forms` draws a drop-down's arrow 0.5rem from its right edge and gives the select 2.5rem of right
 * padding so the text stops before the arrow. A screen's `px-3` is a utility in
 * Tailwind's `utilities` layer and came later than the plugin's rule in the `base` layer, so it cut that padding
 * to 0.75rem and the text ran under the arrow. One rule in `src/app.css`, outside every layer, restores it: a rule
 * that is in no layer beats every rule that is in one, whatever its specificity or position, so no utility can
 * undo it again (CSS Cascading and Inheritance Level 5, section 6.4.3 "Layer Ordering",
 * https://www.w3.org/TR/css-cascade-5/#layer-ordering: unlayered styles "come last in the layer order", and section
 * 6.1: "for normal rules the declaration whose cascade layer is last wins").
 *
 * The cascade itself is proven in a real browser by the phone sweep (a select's shown text fits between its left
 * padding and its arrow). This test pins the rule's place: if someone moves it into a layer, or
 * lowers the padding, the sweep would only say so one browser run later.
 */

const CSS = fs.readFileSync(path.resolve(__dirname, '../../../app.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');

interface IBlock {
    /** What comes before the braces: a selector, or an at-rule such as `@layer base`. */
    prelude: string;
    /** What is between the braces. */
    body: string;
}

/** The blocks of the stylesheet that sit at its top level, i.e. in no `@layer`, `@media` or other block. */
function topLevelBlocks(css: string): IBlock[] {
    const blocks: IBlock[] = [];
    let depth = 0;
    let prelude = '';
    let body = '';
    for (const char of css) {
        if (char === '{') {
            if (depth === 0) {
                body = '';
            } else {
                body += char;
            }
            depth++;
        } else if (char === '}') {
            depth--;
            if (depth === 0) {
                blocks.push({ prelude: prelude.trim(), body: body.trim() });
                prelude = '';
            } else {
                body += char;
            }
        } else if (depth === 0) {
            // A statement such as `@import 'tailwindcss';` ends at its semicolon and has no braces.
            if (char === ';') prelude = '';
            else prelude += char;
        } else {
            body += char;
        }
    }
    return blocks;
}

const SELECT_RULE = topLevelBlocks(CSS).find((block) => /^select\b/.test(block.prelude));

describe('the select arrow rule in src/app.css', () => {
    it('is a rule in no layer, so no utility class can undo it', () => {
        expect(SELECT_RULE, 'src/app.css has no top-level rule for `select`').toBeDefined();
        const layered = topLevelBlocks(CSS).filter((block) => block.prelude.startsWith('@layer'));
        for (const layer of layered) {
            expect(layer.body, `${layer.prelude} must not hold the select arrow rule`).not.toMatch(
                /select:not\(\[multiple\]\)/
            );
        }
    });

    it('covers every drop-down and leaves lists (multiple, size) alone, which draw no arrow', () => {
        expect(SELECT_RULE?.prelude).toBe('select:not([multiple]):not([size])');
    });

    it('gives the select the 2.5rem of right padding the forms plugin keeps for the arrow, as a physical property', () => {
        // `padding-right`, not `padding-inline`: the arrow sits at `right`, not at the end of the writing direction.
        expect(SELECT_RULE?.body).toMatch(/(^|;|\s)padding-right:\s*2\.5rem\s*;?/);
    });
});

describe('the block walker of this test', () => {
    it('finds only top-level blocks, ignores statements and does not mistake a nested rule for one', () => {
        const blocks = topLevelBlocks(
            "@import 'tailwindcss';\n@layer base { select { padding-right: 1rem; } }\nselect:not([multiple]) { padding-right: 2.5rem; }"
        );

        expect(blocks.map((block) => block.prelude)).toEqual(['@layer base', 'select:not([multiple])']);
        expect(blocks[1].body).toBe('padding-right: 2.5rem;');
    });
});

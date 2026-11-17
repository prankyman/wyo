import type { Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import { levelProblems } from '../helpers/box-level.js';
import { stackGapProblems } from '../helpers/stack-gaps.js';

/**
 * The self-test of the two layout measurements the screen sweep adds to the phone's rules (`e2e/helpers/sweep.ts`): the gap between
 * the blocks of a column (`e2e/helpers/stack-gaps.ts`) and the level of the boxes of a form (`e2e/helpers/box-level.ts`). Like
 * `phone-measure-selftest.spec.ts`, every page is a small HTML document built with `page.setContent`, and each measurement is
 * shown a page that is wrong, which it must report, and a page that is right, which it must leave alone.
 */

const CSS = `*{box-sizing:border-box}html,body{margin:0}body{padding:16px;font:16px/24px sans-serif}
[data-stack]{display:flex;flex-direction:column}
.row{display:flex;gap:16px}.row>div{flex:1}label{display:block}input{display:block;width:100%;height:44px;margin-top:6px}`;

async function show(page: Page, body: string): Promise<void> {
    await page.setContent(
        `<!doctype html><html lang="en"><head><meta charset="utf-8"><style>${CSS}</style></head><body>${body}</body></html>`
    );
}

const block = (name: string): string => `<p data-test="${name}" style="margin:0">${name}</p>`;

test.describe('the layout measurements', () => {
    test('the gap measurement reports a column whose blocks are not 16 px apart, and an empty block, and leaves a right column alone', async ({
        page
    }) => {
        await show(
            page,
            `<div data-stack data-test="right" style="gap:16px">${block('one')}${block('two')}${block('three')}</div>`
        );
        expect(await stackGapProblems(page)).toEqual([]);

        await show(page, `<div data-stack data-test="wide" style="gap:24px">${block('one')}${block('two')}</div>`);
        expect(await stackGapProblems(page)).toEqual(['wide | one → two | 24 px']);

        await show(
            page,
            `<div data-stack data-test="margin" style="gap:16px">${block('one')}<p data-test="two" style="margin:8px 0 0">two</p></div>`
        );
        expect(await stackGapProblems(page)).toEqual(['margin | one → two | 24 px']);

        await show(
            page,
            `<div data-stack data-test="holey" style="gap:16px">${block('one')}<div data-test="empty"></div></div>`
        );
        expect(await stackGapProblems(page)).toEqual(['holey | empty | no height, yet it takes a gap']);
    });

    test('the level measurement reports boxes of one form that start at different heights, and a label too far above its box', async ({
        page
    }) => {
        const field = (id: string, label: string): string =>
            `<div><label for="${id}">${label}</label><input id="${id}" data-test="${id}"></div>`;

        await show(page, `<form class="row">${field('first', 'First')}${field('second', 'Second')}</form>`);
        expect(await levelProblems(page)).toEqual([]);

        // A label that runs over two lines pushes its box 24 px lower than the box beside it.
        await show(
            page,
            `<form class="row">${field('first', 'First')}${field('second', 'A label long enough to run over two lines<br>and then some more')}</form>`
        );
        const pushed = await levelProblems(page);
        expect(pushed.filter((problem) => problem.startsWith('level | '))).toHaveLength(1);
        expect(pushed.join('\n')).toContain('runs over 2 lines');

        // A label far above its box (26 px of space above the box): the designed gap is 6 px, the most allowed is 10.
        await show(page, `<form>${field('lonely', 'Lonely')}</form>`);
        await page.addStyleTag({ content: 'input{margin-top:26px}' });
        const far = await levelProblems(page);
        expect(far).toHaveLength(1);
        expect(far[0]).toMatch(
            /^gap \| lonely: \d+(\.\d)? px from the text of its label «Lonely» to the box, the most is 10 px$/
        );
    });
});

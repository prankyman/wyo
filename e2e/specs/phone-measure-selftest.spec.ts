import type { Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import {
    collectPhoneFailures,
    expectInsidePhone,
    expectNoSidewaysScroll,
    expectTapTarget,
    formatPhoneFailures,
    measurePhoneScreen,
    PHONE,
    type IMeasureStats,
    type IPhoneFailure
} from '../helpers/phone.js';
import {
    assertNoStaleExceptions,
    checkExceptionList,
    createExceptionRegistry,
    PHONE_EXCEPTIONS,
    type IPhoneException
} from '../support/phone-exceptions.js';

/**
 * The self-test of the phone measuring function (`e2e/helpers/phone.ts`): the sweep trusts
 * `measurePhoneScreen`, so the function is itself measured here. Every page is a small HTML
 * document built with `page.setContent` — no page of the wyo interface — that holds exactly one
 * defect (or one thing that only looks like a defect), and the test says which rule must report it. Breaking a rule in the
 * function makes the case of that rule fail: each case below is the mutation check of one rule of the sweep.
 *
 * The documents carry the viewport tag a phone gets from the wyo interface's `src/app.html`; without it a mobile browser lays the
 * page out 980 px wide.
 */

test.use({ viewport: PHONE });

const ROUTE = '/selftest-page';

const BASE_CSS = `
*{box-sizing:border-box}
html,body{margin:0}
body{padding:16px;font:16px/24px sans-serif;color:#111}
.clickable-surface{position:relative}
.clickable-surface [data-stretched-link]::after{content:'';position:absolute;inset:0;z-index:0}
`;

const SELECT_STYLE =
    'display:block;width:100%;height:44px;margin-bottom:8px;appearance:none;-webkit-appearance:none;padding:0 40px 0 12px;font:16px sans-serif;border:1px solid #444;background:#fff;border-radius:0';

const SR_ONLY =
    'position:absolute;width:1px;height:1px;margin:-1px;overflow:hidden;clip-path:inset(50%);opacity:0;white-space:nowrap';

const LONG_TEXT = 'Identical items of different groups go into different folders and then many more words in a row';

/** A select that shows the long text and points at the element `description` (which must have the id `full-text`). */
const describedSelect = (dataTest: string, description: string): string =>
    `<select data-test="${dataTest}" aria-describedby="full-text" style="${SELECT_STYLE}"><option>${LONG_TEXT}</option></select>${description}`;

/** Puts a document on the page: the application's viewport tag, a reset, and the body. */
async function show(page: Page, body: string): Promise<void> {
    await page.setContent(
        `<!doctype html><html lang="en"><head><meta charset="utf-8">` +
            `<meta name="viewport" content="width=device-width, initial-scale=1"><style>${BASE_CSS}</style></head>` +
            `<body>${body}</body></html>`
    );
}

/** The failures as `rule:target`, in the order the function reports them. */
const seen = (failures: readonly IPhoneFailure[]): string[] =>
    failures.map((failure) => `${failure.rule}:${failure.target}`);

interface ICase {
    name: string;
    body: string;
    /** `rule:target` of every failure the page must report, in order; empty for a page that must report nothing. */
    expected: string[];
}

/** One defect each; the rule that must report it is in `expected`. */
const DEFECTS: ICase[] = [
    {
        name: 'a link 40 px tall',
        body: '<a href="#one" data-test="link-40" style="display:inline-flex;align-items:center;height:40px;min-width:44px;border:1px solid #444">Open</a>',
        expected: ['tap-size:link-40']
    },
    {
        name: 'a button 30 px wide',
        body: '<button type="button" data-test="button-30" style="width:30px;height:44px;padding:0">Go</button>',
        expected: ['tap-size:button-30']
    },
    {
        name: 'a bare checkbox 16 px square',
        body: '<input type="checkbox" data-test="bare-box" aria-label="bare" style="width:16px;height:16px;margin:0">',
        expected: ['tap-size:bare-box']
    },
    {
        name: 'a hidden switch whose label is 20 px tall',
        body:
            `<label style="display:flex;align-items:center;height:20px;width:200px;position:relative">` +
            `<input type="checkbox" role="switch" data-test="slim-switch" style="${SR_ONLY}">Switch text</label>`,
        expected: ['tap-size:slim-switch']
    },
    {
        name: 'a switch hidden with display none whose label is 20 px tall',
        body:
            `<label style="display:flex;align-items:center;height:20px;width:200px">` +
            `<input type="checkbox" role="switch" data-test="gone-switch" style="display:none">Switch text</label>`,
        expected: ['tap-size:gone-switch']
    },
    {
        name: 'a file input hidden visually whose label is 20 px tall',
        body:
            `<label style="display:flex;align-items:center;height:20px;width:200px;position:relative">` +
            `<input type="file" data-test="hidden-file" style="${SR_ONLY}">Choose a file</label>`,
        expected: ['tap-size:hidden-file']
    },
    {
        name: 'a lone link in a block, which is not a sentence',
        body: '<p><a href="#lone" data-test="lone-link">Alone</a></p>',
        expected: ['tap-size:lone-link']
    },
    {
        name: 'a link followed only by a number, which is not a sentence either',
        body: '<p><a href="#num" data-test="num-link">12</a> (3)</p>',
        expected: ['tap-size:num-link']
    },
    {
        name: 'a stretched link that sits in no card',
        body:
            '<div style="position:relative;height:80px"><a data-stretched-link href="/x" data-test="loose-stretched" ' +
            'style="display:inline-flex;align-items:center;min-height:20px">Item</a></div>',
        expected: ['tap-size:loose-stretched']
    },
    {
        name: 'a button past the right edge of a strip that holds no table',
        body:
            '<div data-test="strip" style="overflow-x:auto;width:100%"><div style="display:flex;width:700px">' +
            '<button type="button" data-test="tab-near" style="width:100px;height:44px">A</button>' +
            '<button type="button" data-test="tab-far" style="margin-left:300px;width:100px;height:44px">B</button></div></div>',
        expected: ['off-screen:tab-far']
    },
    {
        name: 'a tab past the right edge of a panel that holds a table',
        body:
            '<div data-test="table-panel" style="overflow-x:auto;width:100%"><table style="width:700px"><tr><td>' +
            '<div role="tablist"><button type="button" role="tab" data-test="tab-in-table" ' +
            'style="margin-left:400px;width:100px;height:44px">T</button></div></td></tr></table></div>',
        expected: ['off-screen:tab-in-table']
    },
    {
        name: 'a table panel that starts left of the screen',
        body:
            '<div data-test="shifted-panel" style="overflow-x:auto;width:358px;margin-left:-40px"><table style="width:700px"><tr><td>' +
            '<button type="button" data-test="shifted-button" style="width:100px;height:44px">T</button></td></tr></table></div>',
        expected: ['off-screen:shifted-panel']
    },
    {
        name: 'a last button under a fixed bar with no room below it: no scroll position clears it',
        body:
            '<div style="height:1200px"></div><button type="button" data-test="under-bar" style="width:120px;height:44px">Save</button>' +
            '<div data-test="bottom-bar" style="position:fixed;left:0;right:0;bottom:0;height:100px;background:#ccc">Notice</div>',
        expected: ['covered:under-bar']
    },
    {
        name: 'a button under a fixed full-screen overlay on a long page: scrolling clears nothing',
        body:
            '<div style="height:1200px"></div><button type="button" data-test="under-overlay" style="width:120px;height:44px">Save</button>' +
            '<div style="height:1200px"></div>' +
            '<div data-test="overlay" style="position:fixed;inset:0;background:rgba(0,0,0,.3)"></div>',
        expected: ['covered:under-overlay']
    },
    {
        name: 'a button on a short page that cannot scroll, under a fixed bar',
        body:
            '<button type="button" data-test="short-page-button" style="width:120px;height:44px">Save</button>' +
            '<div data-test="top-bar" style="position:fixed;left:0;right:0;top:0;height:100px;background:#ccc">Notice</div>',
        expected: ['covered:short-page-button']
    },
    {
        name: 'a fixed button stacked under another fixed box: the page scrolls under both, so no scroll position clears it',
        body:
            '<div style="height:1200px"></div>' +
            '<button type="button" data-test="fixed-under" style="position:fixed;left:16px;top:200px;z-index:1;width:120px;height:44px">Fixed</button>' +
            '<div data-test="fixed-cover" style="position:fixed;left:0;right:0;top:180px;z-index:2;height:100px;background:#ccc">Cover</div>',
        expected: ['covered:fixed-under', 'fixed-overlap:fixed-under']
    },
    {
        name: 'two buttons that overlap by 10 px',
        body:
            '<button type="button" data-test="over-a" style="display:block;width:120px;height:44px">A</button>' +
            '<button type="button" data-test="over-b" style="display:block;position:relative;margin-top:-10px;width:120px;height:44px">B</button>',
        expected: ['overlap:over-a']
    },
    {
        name: 'two fixed boxes that overlap by 30 px',
        body:
            '<p>Some text.</p>' +
            '<div data-test="fixed-a" style="position:fixed;left:16px;right:16px;top:100px;height:80px;background:#ddd">A</div>' +
            '<div data-test="fixed-b" style="position:fixed;left:16px;right:16px;top:150px;height:80px;background:#eee">B</div>',
        expected: ['fixed-overlap:fixed-a']
    },
    {
        name: 'a select whose chosen text runs under its arrow',
        body: `<select data-test="sel-long" style="${SELECT_STYLE}"><option>${LONG_TEXT}</option></select>`,
        expected: ['select-text:sel-long']
    },
    {
        name: 'a select with fixed choices and a long option nobody chose',
        body: `<select data-test="sel-fixed" data-choices="fixed" style="${SELECT_STYLE}"><option>Yes</option><option>${LONG_TEXT}</option></select>`,
        expected: ['select-text:sel-fixed']
    },
    {
        name: 'a select whose long chosen text is described by an element that is not drawn (the hidden attribute)',
        body: describedSelect('sel-hidden-desc', `<p id="full-text" hidden>${LONG_TEXT}</p>`),
        expected: ['select-text:sel-hidden-desc']
    },
    {
        name: 'a select whose long chosen text is described by an element with visibility hidden',
        body: describedSelect('sel-invisible-desc', `<p id="full-text" style="visibility:hidden">${LONG_TEXT}</p>`),
        expected: ['select-text:sel-invisible-desc']
    },
    {
        name: 'a select whose long chosen text is described by an element drawn 1 px small (visually hidden)',
        body: describedSelect('sel-sr-desc', `<p id="full-text" style="${SR_ONLY}">${LONG_TEXT}</p>`),
        expected: ['select-text:sel-sr-desc']
    },
    {
        name: 'a select whose long chosen text is described by an element that holds only part of it',
        body: describedSelect('sel-part-desc', '<p id="full-text">Identical items of different groups</p>'),
        expected: ['select-text:sel-part-desc']
    },
    {
        name: 'a select whose aria-describedby points at an element that does not exist',
        body: `<select data-test="sel-lost-desc" aria-describedby="nowhere" style="${SELECT_STYLE}"><option>${LONG_TEXT}</option></select>`,
        expected: ['select-text:sel-lost-desc']
    },
    {
        name: 'a select with fixed choices, a long option and a visible description holding it: every option must still fit',
        body:
            `<select data-test="sel-fixed-desc" data-choices="fixed" aria-describedby="full-text" style="${SELECT_STYLE}"><option>Yes</option><option>${LONG_TEXT}</option></select>` +
            `<p id="full-text">${LONG_TEXT}</p>`,
        expected: ['select-text:sel-fixed-desc']
    },
    {
        name: 'a page wider than the screen',
        body: '<div style="width:600px;height:20px;background:#ccc"></div>',
        expected: ['sideways-scroll:page']
    },
    {
        name: 'a menu item 30 px tall inside a closed details',
        body:
            '<details data-test="menu"><summary data-test="menu-toggle" style="display:flex;align-items:center;min-height:44px;width:200px">Menu</summary>' +
            '<a href="#item" data-test="menu-item-30" style="display:flex;align-items:center;height:30px;width:200px">Item</a></details>',
        expected: ['tap-size:menu-item-30']
    },
    {
        name: 'a 14 px help «?» button that carries no help-icon marker',
        body: '<p>Valid for <span style="display:inline-block;vertical-align:middle"><button type="button" data-test="bare-help" aria-label="Help" style="width:14px;height:14px;padding:0"></button></span></p>',
        expected: ['tap-size:bare-help']
    }
];

/** Nothing wrong, or only something that looks wrong; the page must report nothing. */
const FINE: ICase[] = [
    {
        name: 'a switch with a 20 px title label elsewhere and a 44 px label around it',
        body:
            '<label for="two-labels" style="display:block;height:20px;width:200px">Title</label>' +
            '<label style="display:flex;align-items:center;min-height:44px;width:200px"><input id="two-labels" type="checkbox" role="switch" data-test="two-labels">Switch text</label>',
        expected: []
    },
    {
        name: 'a link inside a sentence',
        body: '<p>Please read the <a href="#terms" data-test="terms-link">terms of use</a> before you go on.</p>',
        expected: []
    },
    {
        name: 'a stretched link measured as its card, with a lifted button inside the card',
        body:
            '<div class="clickable-surface" data-test="card" style="height:96px;border:1px solid #444;padding:8px">' +
            '<a data-stretched-link href="/x" data-test="card-link" style="display:inline-flex;align-items:center;min-height:20px">Item 1</a>' +
            '<button type="button" data-test="card-button" style="position:absolute;right:8px;bottom:8px;z-index:1;width:44px;height:44px">x</button></div>',
        expected: []
    },
    {
        name: 'a checkbox in a 44 px label and a hidden switch (sr-only, or display none) in a 44 px label',
        body:
            '<label data-test="row-label" style="display:flex;align-items:center;gap:8px;min-height:44px"><input type="checkbox" data-test="row-box">Row text</label>' +
            `<label style="display:flex;align-items:center;min-height:44px;width:200px;position:relative"><input type="checkbox" role="switch" data-test="good-switch" style="${SR_ONLY}">Switch text</label>` +
            `<label style="display:flex;align-items:center;min-height:44px;width:200px"><input type="checkbox" role="switch" data-test="good-gone-switch" style="display:none">Switch text</label>`,
        expected: []
    },
    {
        name: 'controls at 44 px, a select that fits, fixed choices that all fit, an unmarked select with a long unchosen option',
        body:
            '<a href="#a" data-test="ok-link" style="display:inline-flex;align-items:center;min-width:44px;height:44px;border:1px solid #444;margin-right:8px">Open</a>' +
            '<button type="button" data-test="ok-button" style="width:44px;height:44px;padding:0">Go</button>' +
            '<input data-test="ok-input" style="display:block;width:100%;height:44px;margin:8px 0">' +
            `<select data-test="ok-select" style="${SELECT_STYLE}"><option>Yes</option></select>` +
            `<select data-test="ok-fixed" data-choices="fixed" style="${SELECT_STYLE}"><option>Yes</option><option>No</option></select>` +
            `<select data-test="ok-unmarked" style="${SELECT_STYLE}"><option>Yes</option><option>${LONG_TEXT}</option></select>`,
        expected: []
    },
    {
        name: 'a target beyond the screen inside a panel that holds a table',
        body:
            '<div data-test="table-panel" style="overflow-x:auto;width:100%;margin-bottom:8px"><table style="width:700px"><tr><td>' +
            '<button type="button" data-test="cell-button" style="margin-left:400px;width:100px;height:44px">T</button></td></tr></table></div>',
        expected: []
    },
    {
        name: 'a last button with room under the fixed bar',
        body:
            '<div style="height:1200px"></div><button type="button" data-test="clear-of-bar" style="width:120px;height:44px">Save</button>' +
            '<div style="height:120px"></div>' +
            '<div data-test="bottom-bar" style="position:fixed;left:0;right:0;bottom:0;height:100px;background:#ccc">Notice</div>',
        expected: []
    },
    {
        name: 'buttons that a tall fixed bar (60 % of the screen) covers at the middle of the screen, which the page scrolls up clear of it',
        body:
            '<div style="height:1200px"></div>' +
            '<button type="button" data-test="mid-button" style="display:block;width:120px;height:44px">Middle</button>' +
            '<div style="height:600px"></div>' +
            '<button type="button" data-test="last-button" style="display:block;width:120px;height:44px">Save</button>' +
            '<div style="height:540px"></div>' +
            '<div data-test="tall-bar" style="position:fixed;left:0;right:0;bottom:0;height:510px;background:#ccc">Notices</div>',
        expected: []
    },
    {
        name: 'a button that only the bottom edge of the screen leaves free, under a tall fixed bar at the top',
        body:
            '<div style="height:1200px"></div><button type="button" data-test="only-end-free" style="width:120px;height:44px">Go</button>' +
            '<div style="height:1200px"></div>' +
            '<div data-test="tall-top-bar" style="position:fixed;left:0;right:0;top:0;height:510px;background:#ccc">Notices</div>',
        expected: []
    },
    {
        name: 'a button the page can scroll clear of a fixed pop-up in the middle of the screen',
        body:
            '<div style="height:800px"></div><button type="button" data-test="clear-of-popup" style="width:120px;height:44px">Go</button>' +
            '<div style="height:800px"></div>' +
            '<div data-test="popup" style="position:fixed;left:16px;right:16px;top:300px;height:240px;background:#fff;border:1px solid #444">Pop-up text</div>',
        expected: []
    },
    {
        name: 'a select whose long chosen text is shown in full by a visible element it points at (aria-describedby), also beside a hidden one',
        body:
            `<select data-test="sel-described" aria-describedby="hint full-text" style="${SELECT_STYLE}"><option>${LONG_TEXT}</option></select>` +
            '<p id="hint" hidden>Choose the service.</p>' +
            `<p id="full-text">  ${LONG_TEXT}  </p>`,
        expected: []
    },
    {
        name: 'a button inside an input, and two links to one address that overlap',
        body:
            '<div style="position:relative;width:100%;margin-bottom:8px"><input data-test="combo" style="display:block;width:100%;height:44px">' +
            '<button type="button" data-test="caret" style="position:absolute;right:0;top:0;width:44px;height:44px;padding:0">v</button></div>' +
            '<a href="/same" data-test="same-a" style="display:block;width:120px;height:44px">A</a>' +
            '<a href="/same" data-test="same-b" style="display:block;position:relative;margin-top:-10px;width:120px;height:44px">B</a>',
        expected: []
    },
    {
        name: 'fixed boxes with a gap, an inert one, one moved away, and two pop-ups inside a click-through wrapper',
        body:
            '<div data-test="fixed-1" style="position:fixed;left:16px;right:16px;bottom:100px;height:50px;background:#ddd">one</div>' +
            '<div data-test="fixed-2" style="position:fixed;left:16px;right:16px;bottom:30px;height:50px;background:#ddd">two</div>' +
            '<div inert data-test="fixed-inert" style="position:fixed;left:16px;right:16px;bottom:60px;height:50px;background:#eee">inert</div>' +
            '<div data-test="fixed-away" style="position:fixed;left:16px;right:16px;bottom:60px;height:50px;transform:translateX(500px)">away</div>' +
            '<div data-test="wrapper" style="position:fixed;inset:0;pointer-events:none">' +
            '<div data-test="pop-1" style="pointer-events:auto;position:absolute;top:100px;left:16px;right:16px;height:50px;background:#eee">p1</div>' +
            '<div data-test="pop-2" style="pointer-events:auto;position:absolute;top:200px;left:16px;right:16px;height:50px;background:#eee">p2</div></div>',
        expected: []
    },
    {
        name: 'links that are not drawn: display none, hidden, inert, aria-hidden, hidden visually, clipped away, click-through',
        body:
            '<div style="display:none"><a href="#h1" style="display:block;height:20px">h1</a></div>' +
            '<a href="#h2" style="visibility:hidden;display:block;height:20px">h2</a>' +
            '<div inert><a href="#h3" style="display:block;height:20px;width:100px">h3</a></div>' +
            '<div aria-hidden="true"><a href="#h4" style="display:block;height:20px;width:100px">h4</a></div>' +
            `<a href="#h5" style="${SR_ONLY}">skip</a>` +
            '<div style="height:0;overflow:hidden"><a href="#h6" style="display:block;height:20px">clipped</a></div>' +
            '<a href="#h7" style="pointer-events:none;display:block;height:20px;width:100px">h7</a>',
        expected: []
    },
    {
        name: "a 14 px help «?» button inside the help-icon marker, in a label row (WCAG's Inline exception)",
        body: '<p>Valid for <span data-help-icon style="display:inline-block;vertical-align:middle"><button type="button" data-test="marked-help" aria-label="Help" style="width:14px;height:14px;padding:0"></button></span></p>',
        expected: []
    }
];

const SMALL_LINK =
    '<a href="#one" data-test="link-40" style="display:inline-flex;align-items:center;height:40px;width:60px;border:1px solid #444">Open</a>';

/** The reason every entry of these cases carries is over 20 characters, as the list requires. */
const REASON = 'The same action is offered by a 44 px button on the same page.';

test.describe('the phone measuring function', () => {
    test('a page with one defect reports exactly the rule that owns it', async ({ page }) => {
        for (const defect of DEFECTS) {
            await show(page, defect.body);
            const failures = await collectPhoneFailures(page, ROUTE);
            expect.soft(seen(failures), defect.name).toEqual(defect.expected);
        }
    });

    test('a page with nothing wrong, or only something that looks wrong, reports nothing', async ({ page }) => {
        for (const fine of FINE) {
            await show(page, fine.body);
            const failures = await collectPhoneFailures(page, ROUTE);
            expect.soft(seen(failures), fine.name).toEqual(fine.expected);
        }
    });

    test('a covered target names what covers it at the middle of the screen and says no scroll position clears it', async ({
        page
    }) => {
        await show(
            page,
            '<div style="height:1200px"></div><button type="button" data-test="under-overlay" style="width:120px;height:44px">Save</button>' +
                '<div style="height:1200px"></div>' +
                '<div data-test="overlay" style="position:fixed;inset:0;background:rgba(0,0,0,.3)"></div>'
        );
        const failures = await collectPhoneFailures(page, ROUTE);

        expect(failures).toHaveLength(1);
        expect(failures[0]).toMatchObject({ route: ROUTE, target: 'under-overlay', rule: 'covered' });
        expect(failures[0].detail).toContain('covered by overlay (fixed) at its centre in the middle of the screen');
        expect(failures[0].detail).toContain('no scroll position clears it');
    });

    test('a select whose text needs half a pixel more than its room fits, one that needs 3 px more does not', async ({
        page
    }) => {
        const text = 'Shipping address of the second company';
        // The canvas measures the text the way the function does; the select is made exactly `extra` px too narrow for it.
        const needed = await page.evaluate((shown) => {
            const context = document.createElement('canvas').getContext('2d')!;
            context.font = 'normal 400 16px sans-serif';
            return context.measureText(shown).width;
        }, text);
        // The select is border-box: 1 px of border on each side and 12 px + 40 px of padding (SELECT_STYLE).
        const selectNeeding = (extra: number): string =>
            `<select data-test="sel-edge" style="${SELECT_STYLE};width:${needed - extra + 2 + 52}px"><option>${text}</option></select>`;

        await show(page, selectNeeding(0.5));
        expect(seen(await collectPhoneFailures(page, ROUTE)), 'room + 0.5 px').toEqual([]);

        await show(page, selectNeeding(3));
        const failures = await collectPhoneFailures(page, ROUTE);
        expect(seen(failures), 'room + 3 px').toEqual(['select-text:sel-edge']);
        expect(failures[0].detail).toMatch(
            /^the chosen text "Shipping address of the second company" needs \d+(\.\d)? px, the room between padding and arrow is \d+(\.\d)? px$/
        );
    });

    test('every failure of a page is collected together and printed one per line', async ({ page }) => {
        await show(
            page,
            `${SMALL_LINK}<button type="button" data-test="button-30" style="display:block;width:30px;height:44px;padding:0">Go</button>` +
                '<div style="width:600px;height:20px;background:#ccc"></div>'
        );
        const stats = {} as IMeasureStats;
        const failures = await collectPhoneFailures(page, ROUTE, { stats });

        expect(seen(failures).sort()).toEqual(['sideways-scroll:page', 'tap-size:button-30', 'tap-size:link-40']);
        // The measurement says what it looked at: two tap targets, no select, no fixed box, and it was quick.
        expect(stats).toMatchObject({ targets: 2, selects: 0, fixedBoxes: 0 });
        expect(stats.milliseconds).toBeLessThan(1000);
        const small = failures.find((failure) => failure.target === 'link-40');
        expect(small).toMatchObject({ route: ROUTE, rule: 'tap-size', box: { x: 16, y: 16, w: 60, h: 40 } });
        const lines = formatPhoneFailures(failures);
        expect(lines).toHaveLength(3);
        expect(lines).toContain(`${ROUTE} | link-40 | tap-size | 60x40 at 16,16 | ${small!.detail}`);
        expect(formatPhoneFailures([])).toEqual([]);
    });

    test('the measurement leaves the scroll positions and the closed menus as it found them', async ({ page }) => {
        await show(
            page,
            '<div style="height:900px"></div>' +
                '<div style="overflow-x:auto;width:100%"><div style="display:flex;width:700px">' +
                '<button type="button" data-test="tab-near" style="width:100px;height:44px">A</button>' +
                '<button type="button" data-test="tab-far" style="margin-left:300px;width:100px;height:44px">B</button></div></div>' +
                '<details data-test="menu"><summary style="min-height:44px">Menu</summary><a href="#i" style="display:block;height:44px">Item</a></details>' +
                '<div style="height:900px"></div>'
        );
        await page.evaluate(() => window.scrollTo(0, 120));
        await collectPhoneFailures(page, ROUTE);

        expect(await page.evaluate(() => [window.scrollX, window.scrollY])).toEqual([0, 120]);
        expect(await page.evaluate(() => document.querySelector('div[style*="overflow-x"]')!.scrollLeft)).toBe(0);
        expect(await page.getByTestId('menu').evaluate((el) => (el as HTMLDetailsElement).open)).toBe(false);
    });

    test('a page whose room under the last control lags the fixed stack by two frames is measured once it has settled', async ({
        page
    }) => {
        // A stack of notices appears, and the room that keeps the last control clear of it follows the stack's drawn
        // height through a ResizeObserver, a frame or two later. Measured in between, the last control would be reported covered
        // although the page is right; the measurement waits until the layout stops moving. The stack is put in by one call and
        // the measurement starts at once, so it meets the page within the first frame.
        await show(
            page,
            '<div style="height:1400px"></div>' +
                '<button type="button" data-test="last-control" style="display:block;width:100%;height:44px">Last</button>'
        );
        await page.evaluate(() => {
            const stack = document.createElement('div');
            stack.setAttribute('data-test', 'stack');
            stack.style.cssText = 'position:fixed;left:16px;right:16px;bottom:0;height:400px;background:#fee;z-index:5';
            document.body.append(stack);
            new ResizeObserver(() => {
                requestAnimationFrame(() =>
                    requestAnimationFrame(() => {
                        document.body.style.paddingBottom = `${stack.offsetHeight + 16}px`;
                    })
                );
            }).observe(stack);
        });
        expect(seen(await collectPhoneFailures(page, ROUTE))).toEqual([]);
    });

    test('an exception skips only the failure it names, a stale one fails, a short reason throws', async ({ page }) => {
        const entry: IPhoneException = { route: ROUTE, target: 'link-40', rule: 'tap-size', reason: REASON };

        await show(page, SMALL_LINK);
        const named = createExceptionRegistry([entry]);
        expect(await collectPhoneFailures(page, ROUTE, { exceptions: named })).toEqual([]);
        expect(() => named.assertNoStale(ROUTE)).not.toThrow();

        // Another route, another rule, another target: the failure stays and the entry is stale.
        for (const other of [
            { route: '/another-page' },
            { rule: 'overlap' as const },
            { target: 'link-41' }
        ] satisfies Partial<IPhoneException>[]) {
            const registry = createExceptionRegistry([{ ...entry, ...other }]);
            expect
                .soft(seen(await collectPhoneFailures(page, ROUTE, { exceptions: registry })), JSON.stringify(other))
                .toEqual(['tap-size:link-40']);
        }

        // A route given as a regular expression; an entry that matched nothing on a clean page fails that page's test.
        const pattern = createExceptionRegistry([{ ...entry, route: /^\/selftest-/ }]);
        expect(await collectPhoneFailures(page, ROUTE, { exceptions: pattern })).toEqual([]);
        await show(page, '<p>Nothing to tap here.</p>');
        const stale = createExceptionRegistry([entry]);
        expect(await collectPhoneFailures(page, ROUTE, { exceptions: stale })).toEqual([]);
        expect(() => stale.assertNoStale(ROUTE)).toThrow(/link-40/);
        expect(() => stale.assertNoStale('/another-page')).not.toThrow();

        expect(() => createExceptionRegistry([{ ...entry, reason: 'too short' }])).toThrow(/reason/);
        expect(() => checkExceptionList([{ ...entry, reason: 'x'.repeat(19) }])).toThrow(/link-40/);
        expect(() => checkExceptionList([{ ...entry, reason: 'x'.repeat(20) }])).not.toThrow();

        // The list the sweep ships loads (a short reason would throw at import) and a route with no entry has nothing stale.
        expect(() => checkExceptionList(PHONE_EXCEPTIONS)).not.toThrow();
        expect(() => assertNoStaleExceptions('/a-route-with-no-entry')).not.toThrow();
    });

    test('measurePhoneScreen fails with every failure of the page in one message, and passes a clean page', async ({
        page
    }) => {
        await show(page, '<p>Nothing to tap here.</p>');
        expect(await measurePhoneScreen(page, ROUTE, { soft: false })).toEqual([]);

        await show(
            page,
            `${SMALL_LINK}<button type="button" data-test="button-30" style="display:block;width:30px;height:44px;padding:0">Go</button>`
        );
        const failed = await measurePhoneScreen(page, ROUTE, { soft: false }).then(
            () => null,
            (error: Error) => error.message
        );
        expect(failed).not.toBeNull();
        expect(failed).toContain(`${ROUTE} | link-40 | tap-size | 60x40 at 16,16 |`);
        expect(failed).toContain(`${ROUTE} | button-30 | tap-size | 30x44 at 16,`);
    });

    test('expectTapTarget, expectInsidePhone and expectNoSidewaysScroll keep their signatures at the new bar', async ({
        page
    }) => {
        await show(
            page,
            '<button type="button" data-test="good" style="display:block;width:44px;height:44px;padding:0;margin-bottom:8px">Go</button>' +
                '<button type="button" data-test="short" style="display:block;width:60px;height:40px;padding:0;margin-bottom:8px">Go</button>' +
                '<button type="button" data-test="narrow" style="display:block;width:30px;height:44px;padding:0;margin-bottom:8px">Go</button>' +
                '<div class="clickable-surface" style="height:96px;margin-bottom:8px;border:1px solid #444"><a data-stretched-link href="/x" data-test="card-link" style="display:inline-flex;align-items:center;min-height:20px">Item</a></div>' +
                '<label style="display:flex;align-items:center;min-height:44px;margin-bottom:8px"><input type="checkbox" data-test="boxed">Row</label>' +
                '<input type="checkbox" data-test="bare" aria-label="bare" style="display:block;width:16px;height:16px;margin:0 0 8px">' +
                '<button type="button" data-test="far" style="margin-left:380px;width:44px;height:44px;padding:0;position:absolute">Go</button>'
        );

        await expectTapTarget(page.getByTestId('good'), 'a 44 px button');
        await expectTapTarget(page.getByTestId('card-link'), 'a stretched link, measured as its card');
        await expectTapTarget(page.getByTestId('boxed'), 'a checkbox, measured as its label');
        await expectInsidePhone(page.getByTestId('good'), 'a button inside the screen');
        await expect(expectTapTarget(page.getByTestId('short'), 'a button 40 px tall')).rejects.toThrow(
            /at least 44 px tall/
        );
        await expect(expectTapTarget(page.getByTestId('narrow'), 'a button 30 px wide')).rejects.toThrow(
            /at least 44 px wide/
        );
        await expect(expectTapTarget(page.getByTestId('bare'), 'a bare checkbox')).rejects.toThrow(
            /at least 44 px wide/
        );
        await expect(expectInsidePhone(page.getByTestId('far'), 'a button past the edge')).rejects.toThrow(
            /inside the screen/
        );

        await expect(expectNoSidewaysScroll(page)).rejects.toThrow(/sideways/);
        await show(page, '<p>Fits.</p>');
        await expectNoSidewaysScroll(page);
    });
});

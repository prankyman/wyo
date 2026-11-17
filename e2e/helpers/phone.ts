import { expect as baseExpect, type Locator, type Page } from '@playwright/test';
import { phoneExceptions, type IPhoneExceptionRegistry, type TPhoneRule } from '../support/phone-exceptions.js';

/**
 * The one set of phone helpers. Screens are built for a phone 390 px wide first (constitution, "Every screen is
 * mobile-first"); this file is how a browser test measures that.
 *
 * Exports
 * -------
 * - `PHONE`                                  the viewport, `{ width: 390, height: 844 }`.
 * - `expectNoSidewaysScroll(page, message?)` the page is no wider than the screen (the one check; `message` names the page).
 * - `sidewaysOverflow(page)`                 how many px the page is wider than the screen (the same measure, as a number).
 * - `expectInsidePhone(locator, label)`      the element is visible and lies inside the screen's width.
 * - `expectTapTarget(locator, label)`        the element is a tap target of at least 44 x 44 CSS px, inside the screen. A
 *                                            `StretchedLink` is measured as its card, a checkbox or radio as its label.
 * - `measurePhoneScreen(page, route, options?)`  the sweep's measuring function: measures the whole page that is open, prints
 *                                            every failure at once, returns them. Called on a page that is loaded AND hydrated
 *                                            (the page's own ready marker, `ready` of its row in `e2e/support/screens.ts`): it navigates
 *                                            nothing and waits for nothing. `route` only names the page in messages and in the exceptions list.
 * - `collectPhoneFailures(page, route, options?)`  the same measurement without the assertion (the self-test and callers that
 *                                            want the list use it).
 * - `formatPhoneFailures(failures)`          the failures as lines `route | target | rule | WxH at x,y | detail`.
 * - types `IPhoneFailure`, `IPhoneBox`, `IMeasureOptions`, `IMeasureStats` (what a measurement looked at: `options.stats` is
 *   filled with the number of tap targets, selects and fixed boxes measured and the milliseconds it took); the rules are `TPhoneRule` (`e2e/support/phone-exceptions.ts`).
 *
 * A tap target
 * ------------
 * The box that takes a finger's tap, padding included: `a[href]`, `button`, `select`, `input` (not hidden), `textarea`,
 * `summary`, `[role=button|tab|menuitem|switch]` and `[tabindex]` (other than -1; a scrolling panel is not one). Bar: 44 x 44
 * CSS px, width as well as height (a 0.1 px rounding tolerance). Measured as:
 * - a `StretchedLink` inside a `.clickable-surface` card: the card (its `::after` covers the card);
 * - a checkbox or radio, or any input drawn 1 px small (the `sr-only` pattern): its `<label>`; with no label, its own box;
 * - anything else: its own box.
 * Not measured: an element that is not drawn (display none, hidden, zero box, clipped away by an `overflow: hidden` ancestor,
 * moved off by a clip), `inert`, under `aria-hidden="true"`, with `pointer-events: none`, or drawn 1 px or less (`sr-only`).
 * Exempt: a link in a sentence, that is, an `a` whose `display` is `inline` inside a block that also holds a word of text
 * (a letter, in any script) outside the page's tap targets.
 * Exempt from the SIZE rule only: a help «?». `HelpTooltip` marks its wrapper `data-help-icon`; the button inside is the size of its
 * icon, because a 44 px one made every label row 44 px tall and pushed the boxes under the labels out of line. It is a help icon
 * inside a line of text, the "Inline" exception of WCAG 2.5.8 (and of 2.5.5). It is still measured for being on the screen,
 * covered and overlapping; only `tap-size` skips it.
 *
 * What one call checks (one `page.evaluate`; every closed `<details>` is opened first - and closed again afterwards, as the
 * scroll positions are put back; the rule names are the `rule` column)
 * ---------------------------------------------------------------------------------------------------------------
 * - `sideways-scroll`  the page's width is the screen's (`scrollWidth <= clientWidth` of the document: `innerWidth` grows with the
 *                      content in a phone-emulating browser and would never fail); the detail names what makes it wider.
 * - `tap-size`         every tap target is 44 x 44 at least.
 * - `off-screen`       every tap target lies inside 0 .. screen width, except inside a horizontally scrolling panel that holds a
 *                      `<table>`; that panel itself must then lie inside the screen. A tab
 *                      (`role=tab`) or a menu item (`role=menuitem`) gets no allowance.
 * - `covered`          a target is covered only when NO scroll position of the page leaves it free: a fixed box that takes the
 *                      lower half of the screen covers whatever is scrolled under it, but the person scrolls the page further. Each
 *                      target is scrolled, in this order, to the middle of the screen (`block: 'center'`), with its top at the top
 *                      edge (`'start'`) and with its bottom at the bottom edge (`'end'`); at each position the element at the
 *                      target's centre (`document.elementFromPoint`; the centre of the part inside the screen, for a target taller
 *                      than the screen) is asked, and the first position where it is the target or inside it (the label, for a
 *                      checkbox; the link itself, for a stretched link) clears the target. Reported only if all three positions are
 *                      covered: a full-screen overlay, a fixed box over the target, a target on a page too short to scroll, a fixed
 *                      target under another fixed box. The detail names what covers it at the middle position and says that no
 *                      scroll position clears it. The scroll positions are restored afterwards.
 * - `overlap`          two tap targets that are both in the page or both fixed (or in a sticky box, the pinned top bar) do not
 *                      overlap by more than 1 px in either direction, unless one contains the other (in the page or as
 *                      boxes) or both open the same address.
 * - `fixed-overlap`    two `position: fixed` boxes that are drawn (not inert, not off the screen) do not overlap. A fixed box
 *                      with `pointer-events: none` lets taps through: its tappable children are the boxes instead.
 * - `select-text`      a single-choice select's shown text fits between its left padding and its arrow (its right padding),
 *                      measured with the select's own font. A text fits when it needs at most 1 CSS px more than the room (the canvas
 *                      measures text and the layout rounds boxes, so they differ by a fraction of a pixel). For `data-choices="fixed"`
 *                      every option's text must fit, with no relief. For any other select only the chosen text is measured, and a
 *                      name from data (a service of the price list, 400 px and more) that a native select can neither shorten nor
 *                      wrap is allowed to run long when the page shows it whole beside the select and points the select at it:
 *                      `aria-describedby` names an element that is drawn (not display none, `hidden`, `visibility: hidden`,
 *                      transparent, clipped away or 1 px small) and whose text holds the whole chosen text.
 *
 * A failure is `{ route, target, rule, box, detail }`. `target` is the element's `data-test`, else its tag and the first 40
 * characters of its text. `box` is `{ x, y, w, h }` in CSS px, rounded to 0.1: x and y are page coordinates (viewport
 * coordinates for a fixed element). A failure the exceptions list names (`e2e/support/phone-exceptions.ts`) is dropped first.
 * Failures come in the order of the rules above, and in page order within a rule. `measurePhoneScreen` then asserts - softly,
 * so the test goes on to the next page, unless `options.soft` is false - that there are none, and the message holds every
 * failure of the page, one line each: `route | target | rule | WxH at x,y | detail`.
 */

/** The phone viewport every screen is built for. */
export const PHONE = { width: 390, height: 844 };

/** Each wait follows a round trip to the wyo server: ten seconds, not the default five. */
const expect = baseExpect.configure({ timeout: 10_000 });

/** The bar: a tap target is at least this many CSS px wide and tall. */
const TAP_TARGET_SIZE = 44;

export interface IPhoneBox {
    x: number;
    y: number;
    w: number;
    h: number;
}

export interface IPhoneFailure {
    /** The route the page was measured as (a name for messages and the exceptions list; the function navigates nothing). */
    route: string;
    /** The element's `data-test`, else its tag and the first 40 characters of its text. */
    target: string;
    rule: TPhoneRule;
    box: IPhoneBox;
    detail: string;
}

/** How much a measurement looked at; a sweep can insist the page was not measured empty. */
export interface IMeasureStats {
    /** Tap targets measured (after the ones not drawn and the links in sentences were left out). */
    targets: number;
    /** Single-choice selects whose text was measured. */
    selects: number;
    /** Fixed boxes compared with each other. */
    fixedBoxes: number;
    /** Milliseconds the measurement took inside the page. */
    milliseconds: number;
}

export interface IMeasureOptions {
    /** The exceptions to apply; the shipped list (`PHONE_EXCEPTIONS`) when left out. */
    exceptions?: IPhoneExceptionRegistry;
    /** `measurePhoneScreen` only: false throws at once instead of failing the test softly. */
    soft?: boolean;
    /** Filled with what the measurement looked at. */
    stats?: IMeasureStats;
}

/** What the in-page measurement finds; the route is added, and the exceptions applied, on the test's side. */
interface IRawFailure {
    target: string;
    rule: TPhoneRule;
    box: IPhoneBox;
    detail: string;
}

interface IInPageResult {
    failures: IRawFailure[];
    stats: IMeasureStats;
    /** `mode: 'box'`: the tap box of the one element (viewport coordinates), null when it has none. */
    box: IPhoneBox | null;
}

type TInPageArg = { mode: 'page'; bar: number } | { mode: 'box'; bar: number; target: Element };

/**
 * The measurement, run inside the page by `page.evaluate`. It must stay one self-contained function: Playwright sends its
 * source text to the browser, so it cannot call anything defined outside it.
 */
function measureInPage(arg: TInPageArg): Promise<IInPageResult> | IInPageResult {
    const BAR = arg.bar;
    /** Sub-pixel rounding of the layout: a box 43.95 px tall is 44 px tall. */
    const SIZE_TOLERANCE = 0.1;
    /** Sub-pixel rounding at the screen's edges. */
    const EDGE_TOLERANCE = 0.5;
    /** Two boxes overlap when they share more than this many px in both directions. */
    const MIN_OVERLAP = 1;
    /** A select's text fits when it needs at most this many px more than the room: the canvas and the layout round differently. */
    const FIT_TOLERANCE = 1;
    /** The scroll positions tried, in this order, to bring a tap target clear of whatever is fixed over the page. */
    const SCROLL_POSITIONS = ['center', 'start', 'end'] as const;
    const CONTROLS =
        'a[href], button, select, input:not([type="hidden"]), textarea, summary, [role="button"], [role="tab"], [role="menuitem"], [role="switch"]';
    const INTERACTIVE = `${CONTROLS}, [tabindex]:not([tabindex="-1"])`;

    // `clientWidth` of the document, not `innerWidth`: in a phone-emulating browser `innerWidth` grows to the width of the
    // content when the page is wider than the screen (a 600 px page reports 600), so a comparison with it can never fail.
    const screenWidth = document.documentElement.clientWidth;
    const screenHeight = window.innerHeight;
    const scrollX0 = window.scrollX;
    const scrollY0 = window.scrollY;

    const round = (n: number): number => Math.round(n * 10) / 10;
    const styles = new Map<Element, CSSStyleDeclaration>();
    const styleOf = (el: Element): CSSStyleDeclaration => {
        let style = styles.get(el);
        if (!style) {
            style = getComputedStyle(el);
            styles.set(el, style);
        }
        return style;
    };
    const squash = (text: string): string => text.replace(/\s+/g, ' ').trim();
    const clip = (text: string, length: number): string => squash(text).slice(0, length);

    /** The element's `data-test`, else its tag and the first 40 characters of its text. */
    function nameOf(el: Element): string {
        const dataTest = el.getAttribute('data-test');
        if (dataTest) return dataTest;
        const tag = el.tagName.toLowerCase();
        const text =
            (el as HTMLElement).innerText?.trim() ||
            el.getAttribute('aria-label') ||
            el.getAttribute('placeholder') ||
            el.getAttribute('name') ||
            el.getAttribute('title') ||
            '';
        return text ? `${tag} ${clip(text, 40)}` : tag;
    }

    function isFixed(el: Element): boolean {
        for (let node: Element | null = el; node && node !== document.documentElement; node = node.parentElement) {
            if (styleOf(node).position === 'fixed') return true;
        }
        return false;
    }

    /**
     * A layer of its own over the page: fixed, or inside a `position: sticky` box — a pinned top bar — which stays on screen while the page scrolls under it, so its targets are compared with each other,
     * not with the page's, exactly as a fixed box's are.
     */
    function staysOnScreen(el: Element): boolean {
        for (let node: Element | null = el; node && node !== document.documentElement; node = node.parentElement) {
            const position = styleOf(node).position;
            if (position === 'fixed' || position === 'sticky') return true;
        }
        return false;
    }

    /** Where the element is for the report: page coordinates, viewport coordinates for a fixed element. */
    function boxOf(rect: DOMRect, fixed: boolean): IPhoneBox {
        return {
            x: round(rect.left + (fixed ? 0 : scrollX0)),
            y: round(rect.top + (fixed ? 0 : scrollY0)),
            w: round(rect.width),
            h: round(rect.height)
        };
    }

    /** Not drawn at all, or not part of the page a person can use: display none, hidden, inert, under aria-hidden. */
    function isOutOfPlay(el: Element): boolean {
        if (el.closest('[inert]') || el.closest('[aria-hidden="true"]')) return true;
        const style = styleOf(el);
        if (style.display === 'none' || style.visibility !== 'visible') return true;
        return el.getClientRects().length === 0;
    }

    /** Drawn for a person who looks at the page (aria-hidden does not matter here): a box over 1 px, not hidden, not transparent, not clipped away. */
    function isDrawn(el: Element): boolean {
        if (styleOf(el).visibility !== 'visible' || el.getClientRects().length === 0) return false;
        for (let node: Element | null = el; node; node = node.parentElement) {
            if (styleOf(node).opacity === '0') return false;
        }
        const rect = el.getBoundingClientRect();
        return rect.width > 1 && rect.height > 1 && !isClippedAway(el, rect);
    }

    /** Cut off by an ancestor that clips (`overflow: hidden` or `clip`), so no part of it is drawn. A scrolling panel does not count. */
    function isClippedAway(el: Element, rect: DOMRect): boolean {
        let style = styleOf(el);
        if (style.position === 'fixed') return false;
        let escaping = style.position === 'absolute';
        for (
            let node = el.parentElement;
            node && node !== document.body && node !== document.documentElement;
            node = node.parentElement
        ) {
            style = styleOf(node);
            if (escaping) {
                // An absolutely placed box is cut only by the clipping ancestors from its containing block upwards.
                if (style.position === 'static' && style.transform === 'none') continue;
                escaping = false;
            }
            const clipsX = style.overflowX === 'hidden' || style.overflowX === 'clip';
            const clipsY = style.overflowY === 'hidden' || style.overflowY === 'clip';
            if (!clipsX && !clipsY) continue;
            const frame = node.getBoundingClientRect();
            if (clipsX && (rect.right <= frame.left || rect.left >= frame.right)) return true;
            if (clipsY && (rect.bottom <= frame.top || rect.top >= frame.bottom)) return true;
        }
        return false;
    }

    /** The nearest ancestor-or-self that an absolutely placed child of `el` would be placed against. */
    function containingBlockOf(el: Element): Element {
        for (let node: Element | null = el; node && node !== document.body; node = node.parentElement) {
            const style = styleOf(node);
            if (style.position !== 'static' || style.transform !== 'none') return node;
        }
        return document.body;
    }

    function isScrollContainer(el: Element): boolean {
        const style = styleOf(el);
        const scrolls = (value: string) => value === 'auto' || value === 'scroll';
        return (
            (scrolls(style.overflowX) && el.scrollWidth > el.clientWidth) ||
            (scrolls(style.overflowY) && el.scrollHeight > el.clientHeight)
        );
    }

    interface ITarget {
        /** The control (the input, for a checkbox). */
        el: Element;
        /** The element whose box takes the tap: the control, its label, or its card. */
        owner: Element;
        /** The element to look at when asking what covers the target: the control, the label, or the stretched link itself. */
        probe: Element;
        how: 'self' | 'label' | 'card';
        rect: DOMRect;
        fixed: boolean;
    }

    /**
     * The label a checkbox, radio or hidden input is tapped through. A control can have several (a title label elsewhere in
     * the row and the row that wraps the control): the one that wraps the control is its tap box, else the biggest one drawn.
     */
    function labelOf(input: HTMLInputElement): HTMLLabelElement | null {
        const labels = Array.from(input.labels ?? []);
        const wrapping = input.closest('label');
        if (wrapping && !labels.includes(wrapping)) labels.push(wrapping);
        const drawn = labels.filter((label) => !isOutOfPlay(label));
        const area = (label: Element): number => {
            const box = label.getBoundingClientRect();
            return box.width * box.height;
        };
        return drawn.find((label) => label.contains(input)) ?? drawn.sort((a, b) => area(b) - area(a))[0] ?? null;
    }

    /** What takes the tap for `el`, or null when nothing drawn does. `strict` is false when only the box is wanted. */
    function resolve(el: Element, strict: boolean): ITarget | null {
        let owner: Element = el;
        let probe: Element = el;
        let how: ITarget['how'] = 'self';

        if (el instanceof HTMLInputElement) {
            const own = el.getBoundingClientRect();
            const tinyInput = own.width <= 1 || own.height <= 1;
            if (el.type === 'checkbox' || el.type === 'radio' || tinyInput) {
                const label = labelOf(el);
                if (label) {
                    owner = label;
                    probe = label;
                    how = 'label';
                } else if (tinyInput) {
                    return null;
                }
            }
        } else if (el.matches('a[data-stretched-link]') && el.closest('.clickable-surface')) {
            const card = containingBlockOf(el);
            if (card !== el && card !== document.body) {
                owner = card;
                how = 'card';
            }
        }

        // An input drawn with `display: none` is still tapped through its label, which is checked below.
        if (strict && how === 'self' && isOutOfPlay(el)) return null;
        const rect = owner.getBoundingClientRect();
        if (strict) {
            if (rect.width <= 1 || rect.height <= 1) return null;
            if (how !== 'self' && isOutOfPlay(owner)) return null;
            if (styleOf(owner).pointerEvents === 'none') return null;
            if (isClippedAway(owner, rect)) return null;
        }
        return { el, owner, probe, how, rect, fixed: staysOnScreen(owner) };
    }

    if (arg.mode === 'box') {
        const target = resolve(arg.target, false);
        return {
            failures: [],
            stats: { targets: 0, selects: 0, fixedBoxes: 0, milliseconds: 0 },
            box: target ? boxOf(target.rect, false) : null
        };
    }

    // --- The page as a person finds it: every closed <details> opened (the export menus are measured open).
    const closedDetails = Array.from(document.querySelectorAll('details:not([open])'));
    for (const details of closedDetails) (details as HTMLDetailsElement).open = true;

    const stats: IMeasureStats = { targets: 0, selects: 0, fixedBoxes: 0, milliseconds: 0 };
    const byRule: Record<TPhoneRule, IRawFailure[]> = {
        'sideways-scroll': [],
        'tap-size': [],
        'off-screen': [],
        covered: [],
        overlap: [],
        'fixed-overlap': [],
        'select-text': []
    };
    const fail = (rule: TPhoneRule, target: string, rect: DOMRect, fixed: boolean, detail: string): void => {
        byRule[rule].push({ target, rule, box: boxOf(rect, fixed), detail });
    };

    const measure = (): void => {
        // --- 1. The page never scrolls sideways.
        const root = document.documentElement;
        if (root.scrollWidth > screenWidth) {
            const wide: { el: Element; right: number }[] = [];
            for (const el of Array.from(document.body.querySelectorAll('*'))) {
                const rect = el.getBoundingClientRect();
                if (rect.width === 0 || rect.right <= screenWidth + EDGE_TOLERANCE || isFixed(el)) continue;
                const parent = el.parentElement;
                if (
                    parent &&
                    parent !== document.body &&
                    parent.getBoundingClientRect().right > screenWidth + EDGE_TOLERANCE
                )
                    continue;
                let clipped = false;
                for (let node = el.parentElement; node && node !== document.body; node = node.parentElement) {
                    if (styleOf(node).overflowX !== 'visible') clipped = true;
                }
                if (!clipped) wide.push({ el, right: rect.right + scrollX0 });
            }
            wide.sort((a, b) => b.right - a.right);
            const culprits = wide.slice(0, 3).map((item) => `${nameOf(item.el)} (right edge ${round(item.right)})`);
            byRule['sideways-scroll'].push({
                target: 'page',
                rule: 'sideways-scroll',
                box: { x: 0, y: 0, w: root.scrollWidth, h: root.scrollHeight },
                detail: `the page is ${root.scrollWidth} px wide on a ${screenWidth} px screen${culprits.length > 0 ? `; wider than the screen: ${culprits.join(', ')}` : ''}`
            });
        }

        // --- The tap targets.
        const targets = new Map<Element, ITarget>();
        for (const el of Array.from(document.querySelectorAll(INTERACTIVE))) {
            if (el instanceof HTMLElement && el.tagName === 'SUMMARY' && el.parentElement?.tagName !== 'DETAILS')
                continue;
            // A scrolling panel that is only focusable (for the keyboard) is not a control to tap.
            if (!el.matches(CONTROLS) && isScrollContainer(el)) continue;
            const target = resolve(el, true);
            if (!target) continue;
            // A link in a sentence is exempt: an inline link inside a block that also holds a word outside the tap targets.
            if (target.how === 'self' && el.tagName === 'A' && styleOf(el).display === 'inline') {
                let block = el.parentElement;
                while (block && styleOf(block).display === 'inline') block = block.parentElement;
                let rest = '';
                if (block) {
                    const walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT);
                    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
                        const parent = node.parentElement;
                        if (!parent) continue;
                        const control = parent.closest(INTERACTIVE);
                        if (control && block.contains(control)) continue;
                        const box = parent.getBoundingClientRect();
                        if (box.width <= 1 || box.height <= 1) continue;
                        rest += node.textContent ?? '';
                    }
                }
                if (/\p{L}/u.test(rest)) continue;
            }
            if (!targets.has(target.owner)) targets.set(target.owner, target);
        }
        const list = Array.from(targets.values());
        stats.targets = list.length;

        const nameOfTarget = (target: ITarget): string => {
            if (target.el.getAttribute('data-test')) return target.el.getAttribute('data-test') as string;
            if (target.how === 'label')
                return target.owner.getAttribute('data-test') ?? `${nameOf(target.el)} ${nameOf(target.owner)}`;
            return nameOf(target.el);
        };
        const where = (target: ITarget): string => {
            if (target.el.getAttribute('data-test')) return '';
            const ancestor = target.el.parentElement?.closest('[data-test]');
            return ancestor ? ` (in ${ancestor.getAttribute('data-test')})` : '';
        };
        const via = (target: ITarget): string =>
            target.how === 'label'
                ? ' (measured as its label)'
                : target.how === 'card'
                  ? ` (measured as its card ${nameOf(target.owner)})`
                  : '';

        // --- 2 and 3. The size, and the place on the screen.
        const panelsSeen = new Set<Element>();
        for (const target of list) {
            const { rect } = target;
            // A help «?» inside its marker is the size of its icon on purpose (see the header, "Exempt from the SIZE rule only").
            const helpIcon = target.el.closest('[data-help-icon]') !== null;
            if (!helpIcon && (rect.width < BAR - SIZE_TOLERANCE || rect.height < BAR - SIZE_TOLERANCE)) {
                fail(
                    'tap-size',
                    nameOfTarget(target),
                    rect,
                    target.fixed,
                    `${round(rect.width)}x${round(rect.height)}, the bar is ${BAR}x${BAR}${via(target)}${where(target)}`
                );
            }
            if (rect.left < -EDGE_TOLERANCE || rect.right > screenWidth + EDGE_TOLERANCE) {
                const noAllowance = target.el.closest(
                    '[role="tab"], [role="menuitem"], [role="tablist"], [role="menu"]'
                );
                let panel: Element | null = null;
                if (!noAllowance) {
                    for (
                        let node = target.owner.parentElement;
                        !panel && node && node !== document.body;
                        node = node.parentElement
                    ) {
                        const style = styleOf(node);
                        const scrolls = style.overflowX === 'auto' || style.overflowX === 'scroll';
                        if (scrolls && node.scrollWidth > node.clientWidth + 1) {
                            // The nearest horizontally scrolling panel decides: it must hold a table to be allowed.
                            if (node.querySelector('table')) panel = node;
                            break;
                        }
                    }
                }
                if (panel) {
                    const frame = panel.getBoundingClientRect();
                    if (
                        !panelsSeen.has(panel) &&
                        (frame.left < -EDGE_TOLERANCE || frame.right > screenWidth + EDGE_TOLERANCE)
                    ) {
                        fail(
                            'off-screen',
                            nameOf(panel),
                            frame,
                            isFixed(panel),
                            `the scrolling panel lies at x ${round(frame.left)}..${round(frame.right)}, outside the screen 0..${screenWidth}`
                        );
                    }
                    panelsSeen.add(panel);
                } else {
                    fail(
                        'off-screen',
                        nameOfTarget(target),
                        rect,
                        target.fixed,
                        `lies at x ${round(rect.left)}..${round(rect.right)}, outside the screen 0..${screenWidth}${via(target)}${where(target)}`
                    );
                }
            }
        }

        // A child of a fixed box that scrolls or clips (a stack of notices capped at half the screen) is drawn
        // only inside that box: the part scrolled out of it covers nothing, so only the drawn part is compared.
        const drawnPartOf = (el: Element, root: Element): DOMRect => {
            const rect = el.getBoundingClientRect();
            let [left, top, right, bottom] = [rect.left, rect.top, rect.right, rect.bottom];
            for (let node = el.parentElement; node && root.contains(node); node = node.parentElement) {
                const style = styleOf(node);
                const frame = node.getBoundingClientRect();
                if (style.overflowX !== 'visible')
                    [left, right] = [Math.max(left, frame.left), Math.min(right, frame.right)];
                if (style.overflowY !== 'visible')
                    [top, bottom] = [Math.max(top, frame.top), Math.min(bottom, frame.bottom)];
            }
            return new DOMRect(left, top, Math.max(0, right - left), Math.max(0, bottom - top));
        };
        /** The fixed box a target is drawn in, or null for a target of the page or of the pinned top bar. */
        const fixedRootOf = (el: Element): Element | null => {
            for (let node: Element | null = el; node && node !== document.documentElement; node = node.parentElement) {
                if (styleOf(node).position === 'fixed') return node;
            }
            return null;
        };
        /** What of the target is drawn: in a fixed box only the part inside the box's scrolling or clipping. */
        const drawnRectOf = (target: ITarget): DOMRect => {
            const root = target.fixed ? fixedRootOf(target.owner) : null;
            return root ? drawnPartOf(target.owner, root) : target.rect;
        };

        // --- 5. No two tap targets overlap. A target scrolled out of a fixed box's scrolling part is not drawn, so it overlaps nothing.
        const holds = (outer: DOMRect, inner: DOMRect): boolean =>
            inner.left >= outer.left - EDGE_TOLERANCE &&
            inner.right <= outer.right + EDGE_TOLERANCE &&
            inner.top >= outer.top - EDGE_TOLERANCE &&
            inner.bottom <= outer.bottom + EDGE_TOLERANCE;
        for (let i = 0; i < list.length; i += 1) {
            for (let j = i + 1; j < list.length; j += 1) {
                const a = list[i];
                const b = list[j];
                if (a.fixed !== b.fixed) continue;
                const aRect = drawnRectOf(a);
                const bRect = drawnRectOf(b);
                const width = Math.min(aRect.right, bRect.right) - Math.max(aRect.left, bRect.left);
                const height = Math.min(aRect.bottom, bRect.bottom) - Math.max(aRect.top, bRect.top);
                if (width <= MIN_OVERLAP || height <= MIN_OVERLAP) continue;
                if (
                    a.owner.contains(b.owner) ||
                    b.owner.contains(a.owner) ||
                    a.el.contains(b.el) ||
                    b.el.contains(a.el)
                )
                    continue;
                if (holds(a.rect, b.rect) || holds(b.rect, a.rect)) continue;
                if (a.el instanceof HTMLAnchorElement && b.el instanceof HTMLAnchorElement && a.el.href === b.el.href)
                    continue;
                fail(
                    'overlap',
                    nameOfTarget(a),
                    a.rect,
                    a.fixed,
                    `overlaps ${nameOfTarget(b)} by ${round(width)}x${round(height)}${where(a)}`
                );
            }
        }

        // --- 6. No two fixed boxes overlap. A fixed box that lets taps through stands for its tappable children.
        // Its children are compared by their drawn parts (`drawnPartOf`, above).
        const layers: { el: Element; rect: DOMRect }[] = [];
        const liveBoxes = (el: Element): Element[] =>
            styleOf(el).pointerEvents !== 'none' ? [el] : Array.from(el.children).flatMap(liveBoxes);
        const fixedElements = Array.from(document.body.querySelectorAll('*')).filter(
            (el) => styleOf(el).position === 'fixed'
        );
        for (const fixedElement of fixedElements) {
            if (fixedElement.parentElement && isFixed(fixedElement.parentElement)) continue;
            for (const el of liveBoxes(fixedElement)) {
                if (isOutOfPlay(el)) continue;
                const rect = drawnPartOf(el, fixedElement);
                const visibleWidth = Math.min(rect.right, screenWidth) - Math.max(rect.left, 0);
                const visibleHeight = Math.min(rect.bottom, screenHeight) - Math.max(rect.top, 0);
                if (rect.width <= 0 || rect.height <= 0 || visibleWidth <= 0 || visibleHeight <= 0) continue;
                layers.push({ el, rect });
            }
        }
        stats.fixedBoxes = layers.length;
        for (let i = 0; i < layers.length; i += 1) {
            for (let j = i + 1; j < layers.length; j += 1) {
                const a = layers[i];
                const b = layers[j];
                const width = Math.min(a.rect.right, b.rect.right) - Math.max(a.rect.left, b.rect.left);
                const height = Math.min(a.rect.bottom, b.rect.bottom) - Math.max(a.rect.top, b.rect.top);
                if (width <= MIN_OVERLAP || height <= MIN_OVERLAP) continue;
                if (a.el.contains(b.el) || b.el.contains(a.el)) continue;
                fail(
                    'fixed-overlap',
                    nameOf(a.el),
                    a.rect,
                    true,
                    `overlaps ${nameOf(b.el)} by ${round(width)}x${round(height)}`
                );
            }
        }

        // --- 7. A select shows its text between its left padding and its arrow.
        const canvas = document.createElement('canvas').getContext('2d');
        for (const select of Array.from(document.querySelectorAll('select'))) {
            if (select.multiple || select.size > 1 || !canvas || isOutOfPlay(select)) continue;
            const rect = select.getBoundingClientRect();
            if (rect.width <= 1 || rect.height <= 1 || isClippedAway(select, rect)) continue;
            stats.selects += 1;
            const style = styleOf(select);
            const room =
                rect.width -
                parseFloat(style.borderLeftWidth) -
                parseFloat(style.borderRightWidth) -
                parseFloat(style.paddingLeft) -
                parseFloat(style.paddingRight);
            canvas.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
            canvas.letterSpacing = style.letterSpacing === 'normal' ? '0px' : style.letterSpacing;
            const widthOf = (text: string): number => {
                const shown =
                    style.textTransform === 'uppercase'
                        ? text.toUpperCase()
                        : style.textTransform === 'lowercase'
                          ? text.toLowerCase()
                          : text;
                return canvas.measureText(shown).width;
            };
            const fixedChoices = select.getAttribute('data-choices') === 'fixed';
            const options = fixedChoices
                ? Array.from(select.options)
                : select.selectedOptions[0]
                  ? [select.selectedOptions[0]]
                  : [];
            // A name from data that no native select can shorten or wrap is allowed to run long when the page shows it whole in
            // a drawn element the select points at (`aria-describedby`). Fixed choices get no such relief: every option must fit.
            const describedIds = (select.getAttribute('aria-describedby') ?? '').split(/\s+/).filter(Boolean);
            const shownWhole = (text: string): boolean =>
                describedIds.some((id) => {
                    const described = document.getElementById(id);
                    return (
                        described !== null &&
                        isDrawn(described) &&
                        squash(described.textContent ?? '').includes(squash(text))
                    );
                });
            const tooWide = options
                .map((option) => ({ text: option.label || option.text, width: widthOf(option.label || option.text) }))
                .filter((item) => item.width > room + FIT_TOLERANCE && (fixedChoices || !shownWhole(item.text)));
            if (tooWide.length === 0) continue;
            tooWide.sort((a, b) => b.width - a.width);
            const worst = tooWide[0];
            const which = fixedChoices
                ? `${tooWide.length} of ${options.length} options do not fit; the longest`
                : 'the chosen text';
            const describedNote =
                !fixedChoices && describedIds.length > 0
                    ? '; the element it points at (aria-describedby) is not drawn or does not hold the whole text'
                    : '';
            fail(
                'select-text',
                nameOf(select),
                rect,
                isFixed(select),
                `${which} "${clip(worst.text, 40)}" needs ${round(worst.width)} px, the room between padding and arrow is ${round(room)} px${describedNote}`
            );
        }

        // --- 4. Nothing covers a tap target. The page can be scrolled, so a target is covered only when no scroll position
        // leaves it free: it is scrolled to the middle of the screen, then to the top edge, then to the bottom edge, and the
        // point at its centre is asked at each; the first position where the target itself is there clears it.
        const scrolled = new Map<Element, [number, number]>();
        for (const target of list) {
            for (let node: Element | null = target.owner.parentElement; node; node = node.parentElement) {
                if (
                    !scrolled.has(node) &&
                    (node.scrollWidth > node.clientWidth || node.scrollHeight > node.clientHeight)
                ) {
                    scrolled.set(node, [node.scrollLeft, node.scrollTop]);
                }
            }
            let coveredBy: Element | null = null;
            let cleared = false;
            for (const block of SCROLL_POSITIONS) {
                target.probe.scrollIntoView({ block, inline: 'nearest', behavior: 'instant' });
                const rect = target.probe.getBoundingClientRect();
                const x = rect.left + rect.width / 2;
                // The centre of the part inside the screen: a target taller than the screen is looked at where it can be tapped.
                const top = Math.max(rect.top, 0);
                const bottom = Math.min(rect.bottom, screenHeight);
                // Off the screen is already reported as `off-screen`; there is nothing to look at.
                if (x < 0 || x >= screenWidth || top >= bottom) {
                    cleared = true;
                    break;
                }
                const hit = document.elementFromPoint(x, (top + bottom) / 2);
                if (!hit || target.probe.contains(hit) || (target.how === 'label' && hit === target.el)) {
                    cleared = true;
                    break;
                }
                coveredBy ??= hit;
            }
            if (cleared || !coveredBy) continue;
            const named = coveredBy.closest('[data-test]');
            fail(
                'covered',
                nameOfTarget(target),
                target.rect,
                target.fixed,
                `covered by ${named ? named.getAttribute('data-test') : nameOf(coveredBy)}${isFixed(coveredBy) ? ' (fixed)' : ''} at its centre in the middle of the screen, and also with its top at the top edge and with its bottom at the bottom edge: no scroll position clears it${where(target)}`
            );
        }
        for (const [node, [left, top]] of scrolled) {
            node.scrollLeft = left;
            node.scrollTop = top;
        }
        window.scrollTo({ left: scrollX0, top: scrollY0, behavior: 'instant' });
    };

    const finish = (): IInPageResult => {
        const started = performance.now();
        measure();
        const rules: TPhoneRule[] = [
            'sideways-scroll',
            'tap-size',
            'off-screen',
            'covered',
            'overlap',
            'fixed-overlap',
            'select-text'
        ];
        stats.milliseconds = round(performance.now() - started);
        // Leave the page as it was found: a menu the caller opens afterwards must not find it already open.
        for (const details of closedDetails) (details as HTMLDetailsElement).open = false;
        return { failures: rules.flatMap((rule) => byRule[rule]), stats, box: null };
    };

    // The page is measured once its layout has stopped moving: wait until the document's height has been the same for three
    // frames in a row (30 frames at most: a page that never settles is measured as it stands). One frame is a
    // `requestAnimationFrame` plus a `setTimeout(0)`, which runs after the frame's observers have been delivered. (Only
    // the height is sampled.)
    const settle = async (): Promise<void> => {
        const sample = (): string => String(document.documentElement.scrollHeight);
        const frame = (): Promise<void> =>
            new Promise((resolveFrame) => requestAnimationFrame(() => setTimeout(resolveFrame, 0)));
        let last = sample();
        let steady = 0;
        for (let frames = 0; frames < 30 && steady < 3; frames++) {
            await frame();
            const now = sample();
            if (now === last) steady++;
            else {
                steady = 0;
                last = now;
            }
        }
    };

    const run = async (): Promise<IInPageResult> => {
        if (arg.mode === 'page') await settle();
        // A menu that fills itself when it opens gets a moment to do so.
        if (closedDetails.length > 0) {
            await new Promise((resolveMenu) => setTimeout(resolveMenu, 30));
            // The pause lets the stack change too: settle again, so the room is measured against the stack as it now stands.
            if (arg.mode === 'page') await settle();
        }
        return finish();
    };
    return run();
}

const roundTo = (value: number): string => String(Math.round(value * 10) / 10);

/** The failures as lines: `route | target | rule | WxH at x,y | detail`. */
export function formatPhoneFailures(failures: readonly IPhoneFailure[]): string[] {
    return failures.map(
        (failure) =>
            `${failure.route} | ${failure.target} | ${failure.rule} | ${roundTo(failure.box.w)}x${roundTo(failure.box.h)} at ${roundTo(failure.box.x)},${roundTo(failure.box.y)} | ${failure.detail}`
    );
}

/** The sweep's measurement of the page that is open (see the header), without the assertion: the failures left after the exceptions. */
export async function collectPhoneFailures(
    page: Page,
    route: string,
    options: IMeasureOptions = {}
): Promise<IPhoneFailure[]> {
    const result = await page.evaluate(measureInPage, { mode: 'page' as const, bar: TAP_TARGET_SIZE });
    const exceptions = options.exceptions ?? phoneExceptions;
    if (options.stats) Object.assign(options.stats, result.stats);
    return result.failures
        .map((failure) => ({ route, ...failure }))
        .filter((failure) => !exceptions.isExcepted(route, failure.target, failure.rule));
}

/**
 * Measures the page that is open (loaded and hydrated), asserts that nothing is wrong with it - softly, so the test goes on to
 * the next page - with every failure of the page in the one message, one line each, and returns the failures.
 */
export async function measurePhoneScreen(
    page: Page,
    route: string,
    options: IMeasureOptions = {}
): Promise<IPhoneFailure[]> {
    const failures = await collectPhoneFailures(page, route, options);
    const message =
        failures.length === 0
            ? `${route} fits the phone`
            : `${route} has ${failures.length} phone-layout failure${failures.length === 1 ? '' : 's'} (route | target | rule | WxH at x,y | detail):\n${formatPhoneFailures(failures).join('\n')}`;
    if (options.soft === false) expect(failures.length, message).toBe(0);
    else expect.soft(failures.length, message).toBe(0);
    return failures;
}

/**
 * How many CSS px the document is wider than the screen it is laid out in (0 or less: it fits). Against `clientWidth`, the width
 * of the screen the page is laid out in: `window.innerWidth` of a phone-emulating browser grows with the content, so
 * `scrollWidth - innerWidth` is 0 for a page of any width.
 */
export async function sidewaysOverflow(page: Page): Promise<number> {
    return page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
}

/**
 * The page never slides sideways: the document is no wider than the viewport (a table scrolls in its own panel). The one check
 * of the whole suite: no spec or helper reads `documentElement.scrollWidth` itself. `message` names the page when a test checks
 * several.
 */
export async function expectNoSidewaysScroll(page: Page, message = 'the page scrolls sideways'): Promise<void> {
    expect(await sidewaysOverflow(page), message).toBeLessThanOrEqual(0);
}

/** An element lies fully inside the phone's width. */
export async function expectInsidePhone(locator: Locator, label: string): Promise<void> {
    await expect(locator, label).toBeVisible();
    const box = await locator.boundingBox();
    expect(box, label).not.toBeNull();
    expect(box!.x, `${label} starts inside the screen`).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width, `${label} ends inside the screen`).toBeLessThanOrEqual(PHONE.width);
}

/**
 * A control the finger presses is at least 44 x 44 px (the constitution's mobile-first rule) and inside the screen. The box
 * that counts is the one that takes the tap: a `StretchedLink` is measured as its card, a checkbox or radio as its label.
 */
export async function expectTapTarget(locator: Locator, label: string): Promise<void> {
    await expectInsidePhone(locator, label);
    const handle = await locator.elementHandle();
    expect(handle, label).not.toBeNull();
    const result = await locator.page().evaluate(measureInPage, {
        mode: 'box' as const,
        bar: TAP_TARGET_SIZE,
        target: handle!
    });
    expect(result.box, `${label} has a box that takes the tap`).not.toBeNull();
    const box = result.box!;
    expect(box.w, `${label} is at least ${TAP_TARGET_SIZE} px wide (tap target)`).toBeGreaterThanOrEqual(
        TAP_TARGET_SIZE - 0.1
    );
    expect(box.h, `${label} is at least ${TAP_TARGET_SIZE} px tall (tap target)`).toBeGreaterThanOrEqual(
        TAP_TARGET_SIZE - 0.1
    );
    expect(box.x, `${label}'s tap box starts inside the screen`).toBeGreaterThanOrEqual(-0.5);
    expect(box.x + box.w, `${label}'s tap box ends inside the screen`).toBeLessThanOrEqual(PHONE.width + 0.5);
}

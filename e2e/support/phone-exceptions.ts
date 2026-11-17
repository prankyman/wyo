/**
 * The exceptions list of the screen sweep's phone rules (`e2e/helpers/phone.ts`).
 *
 * The sweep measures every page at 390 × 844 and fails on any control that is too small, off the screen, covered or overlapped.
 * An entry here silences ONE failure of ONE route: it names the route, the element's `data-test` and the rule that reports it,
 * and says why in a reason of at least 20 characters. The list only ever shrinks:
 *
 * - a reason under 20 characters throws when the list is loaded, so a bare «ok» cannot get in;
 * - an entry that matched nothing in the test of its route fails that test (`assertNoStaleExceptions(route)`, called at the
 *   end of a route's test), so a defect that is fixed takes its entry with it.
 *
 * An entry is allowed for exactly two reasons, the two WCAG 2.5.5 gives (https://www.w3.org/WAI/WCAG22/Understanding/target-size-enhanced.html):
 *
 * - "Equivalent": the same action is offered by a control of 44 × 44 px on the same page (name that control in the reason);
 * - "Essential": a different size would change the information the control carries (say what).
 *
 * "It is hard to fix" or "it is old code" is not a reason; the control is fixed instead. The list is EMPTY: no entry is
 * expected.
 */

/** The rules of the sweep, in the order a page reports them. */
export const PHONE_RULES = [
    'sideways-scroll',
    'tap-size',
    'off-screen',
    'covered',
    'overlap',
    'fixed-overlap',
    'select-text'
] as const;

export type TPhoneRule = (typeof PHONE_RULES)[number];

export interface IPhoneException {
    /** The route the failure was measured on, as the test names it: a string must match exactly, a regular expression is tested. */
    route: string | RegExp;
    /** The element's `data-test` (or, for an element with none, the tag and the first 40 characters of its text). */
    target: string;
    rule: TPhoneRule;
    /** Why this is an "Equivalent" or an "Essential" exception; at least `MIN_REASON_LENGTH` characters. */
    reason: string;
}

/** The exceptions. EMPTY (see the header). */
export const PHONE_EXCEPTIONS: readonly IPhoneException[] = [];

/** A reason shorter than this is no reason. */
export const MIN_REASON_LENGTH = 20;

/** Throws on an entry whose reason is under `MIN_REASON_LENGTH` characters; the message names the entry. */
export function checkExceptionList(list: readonly IPhoneException[]): void {
    for (const entry of list) {
        if (entry.reason.trim().length < MIN_REASON_LENGTH) {
            throw new Error(
                `Phone exception for ${describeEntry(entry)} has a reason under ${MIN_REASON_LENGTH} characters ("${entry.reason}"). ` +
                    `Give WCAG's "Equivalent" (the same action is offered by a 44 px control on the same page) or "Essential" reason, or fix the control.`
            );
        }
    }
}

export interface IPhoneExceptionRegistry {
    /** True (and the entry counted as matched on `route`) when an entry names this route, target and rule. */
    isExcepted(route: string, target: string, rule: TPhoneRule): boolean;
    /** Throws, naming each entry, when an entry of `route` matched nothing since the last call for `route`; then forgets `route`. */
    assertNoStale(route: string): void;
}

const routeMatches = (entry: IPhoneException, route: string): boolean =>
    typeof entry.route === 'string' ? entry.route === route : entry.route.test(route);

function describeEntry(entry: IPhoneException): string {
    const route = typeof entry.route === 'string' ? entry.route : String(entry.route);
    return `${route} | ${entry.target} | ${entry.rule}`;
}

/**
 * A registry over a list: it validates the list now (a short reason throws here) and counts, per route, which entries matched.
 * The shipped list has one registry of its own (`phoneExceptions`); the self-test builds registries over lists of its own.
 */
export function createExceptionRegistry(list: readonly IPhoneException[] = PHONE_EXCEPTIONS): IPhoneExceptionRegistry {
    checkExceptionList(list);
    /** Per route, the indexes of the entries that matched at least once. */
    const matched = new Map<string, Set<number>>();

    return {
        isExcepted(route, target, rule) {
            let found = false;
            list.forEach((entry, index) => {
                if (entry.target !== target || entry.rule !== rule || !routeMatches(entry, route)) return;
                found = true;
                const seen = matched.get(route) ?? new Set<number>();
                seen.add(index);
                matched.set(route, seen);
            });
            return found;
        },
        assertNoStale(route) {
            const seen = matched.get(route) ?? new Set<number>();
            matched.delete(route);
            const stale = list.filter((entry, index) => routeMatches(entry, route) && !seen.has(index));
            if (stale.length > 0) {
                throw new Error(
                    `The phone exceptions list holds ${stale.length} entr${stale.length === 1 ? 'y' : 'ies'} for ${route} that matched nothing ` +
                        `(the defect is gone: delete ${stale.length === 1 ? 'it' : 'them'} from e2e/support/phone-exceptions.ts):\n` +
                        stale.map((entry) => `  ${describeEntry(entry)} — ${entry.reason}`).join('\n')
                );
            }
        }
    };
}

/** The registry of the shipped list; `measurePhoneScreen` uses it unless a test hands it another. Loading this module validates the list. */
export const phoneExceptions: IPhoneExceptionRegistry = createExceptionRegistry();

/** Called at the end of a route's test: fails it when an entry of the shipped list for `route` matched nothing on the page. */
export function assertNoStaleExceptions(route: string): void {
    phoneExceptions.assertNoStale(route);
}

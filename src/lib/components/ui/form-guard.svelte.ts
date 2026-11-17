import type { SubmitFunction } from '@sveltejs/kit';
import { untrack } from 'svelte';
import { SvelteMap } from 'svelte/reactivity';
import { pageChange } from './page-change.svelte';
import type { IFormGuard, IGuardSubmitOptions } from './types';

/** How many guarded forms have been sent and have no answer yet, on every page of this tab. */
const awaiting = new (class {
    count = $state(0);
})();

/**
 * One submission per form at a time (constitution, "Press feedback and
 * single submission"). A person on a phone who sees no reaction taps again; without this the second
 * tap sends the form a second time.
 *
 * How it works, for every kind of form in the application:
 *
 * - A `submit` listener in the CAPTURE phase on the form runs before SvelteKit's `enhance`, before
 *   a superForm and before SvelteKit's router (all of which listen in the bubble phase). While the
 *   form is submitting — or while the page is already being replaced — it cancels the new
 *   submission and stops it reaching them.
 * - It never sets `disabled` on the button that submitted: the browser builds the form data AFTER
 *   the submit event, and a disabled submitter is left out of it, so a `name`/`value` on the button
 *   would silently not reach the server. `Button` shows the working state from `aria-disabled`,
 *   `aria-busy` and a spinner, and swallows further clicks itself.
 * - The submission ends when its answer is in:
 *   - plain POST form: the browser loads the answer as a new page, so it never "ends" here; the
 *     page is marked as changing (`pageChange.nativeSubmission`) and everything waits for the new
 *     page. Coming BACK to this page from the back/forward cache (`pageshow` with `persisted`)
 *     releases it;
 *   - SvelteKit `enhance`: when the callback returned by the SubmitFunction has finished, or the
 *     submission was cancelled or aborted (`guardEnhance` below);
 *   - superForm: when its `submitting` store returns to `false`;
 *   - GET form, or any submission that ended in a SvelteKit navigation: when `navigating.to`
 *     returns to `null` after having been set.
 * - A form whose answer opens in another window (`target`, `formtarget`) is not guarded: this page
 *   stays as it is, and nothing would ever end the submission.
 */
export class FormGuard implements IFormGuard {
    submitting = $state(false);
    submitter = $state<HTMLElement | null>(null);

    start(submitter: HTMLElement | null): void {
        if (!this.submitting) awaiting.count += 1;
        this.submitting = true;
        this.submitter = submitter;
    }

    release(): void {
        if (this.submitting) awaiting.count -= 1;
        this.submitting = false;
        this.submitter = null;
    }
}

/**
 * How many guarded forms are waiting for their answer right now. A live re-read of the page
 * (a live refresh) holds back while this is above zero, so a redraw never lands under a form the
 * person has just pressed. Reactive: an effect that reads it runs again when it changes.
 */
export function formsAwaitingAnswer(): number {
    return awaiting.count;
}

/** Every guarded form on the page, by element, so a `Button` finds its form's state through `button.form`. */
const guards = new SvelteMap<HTMLFormElement, IFormGuard>();

/**
 * The guard of the form a control belongs to — by nesting or through the `form` attribute, the same
 * rule the browser uses (`HTMLButtonElement.form`).
 */
export function formGuardOf(form: HTMLFormElement | null | undefined): IFormGuard | undefined {
    return form ? guards.get(form) : undefined;
}

/** The answer will open in another window or frame: this page is not replaced. */
function opensElsewhere(form: HTMLFormElement, submitter: HTMLElement | null): boolean {
    const own =
        submitter instanceof HTMLButtonElement || submitter instanceof HTMLInputElement
            ? submitter.getAttribute('formtarget')
            : null;
    const target = own ?? form.getAttribute('target');
    return Boolean(target) && target !== '_self';
}

/**
 * The guard as an action, for a `<form>` that cannot be a `GuardedForm` (an allowlisted exception
 * in `press-feedback-policy.test.ts`). `GuardedForm` is built on it.
 */
export function guardSubmit(form: HTMLFormElement, initial: IGuardSubmitOptions = {}) {
    let options = initial;
    const guard = options.guard ?? new FormGuard();
    guards.set(form, guard);

    function onSubmit(event: SubmitEvent): void {
        if (guard.submitting || pageChange.nativeSubmission) {
            event.preventDefault();
            event.stopImmediatePropagation();
            return;
        }
        if (opensElsewhere(form, event.submitter)) return;

        guard.start(event.submitter);
        // Who took the submission over is known only once every listener has run.
        setTimeout(() => settle(event), 0);
    }

    function settle(event: SubmitEvent): void {
        if (!guard.submitting) return;
        // Nobody cancelled it: the browser is loading the answer as a new page.
        if (!event.defaultPrevented) {
            pageChange.nativeSubmission = true;
            return;
        }
        // SvelteKit's enhance or a superForm says when the answer is in; SvelteKit's router
        // (a GET form) navigates, and the watcher below ends it with the navigation.
        if (options.enhanced || pageChange.destination !== null) return;
        // Somebody else cancelled it and started nothing this guard could wait for.
        guard.release();
    }

    /** Back from the back/forward cache: the page is as it was before the submission. */
    function onPageShow(event: PageTransitionEvent): void {
        if (event.persisted) guard.release();
    }

    let unsubscribe = (): void => {};
    function watchSuperForm(): void {
        unsubscribe();
        unsubscribe =
            options.submitting?.subscribe((submitting) => {
                if (!submitting) guard.release();
            }) ?? ((): void => {});
    }
    watchSuperForm();

    // A submission that became a SvelteKit navigation ends when the navigation does — whichever way.
    const stopWatchingNavigation = $effect.root(() => {
        let sawNavigation = false;
        $effect(() => {
            const destination = pageChange.destination;
            untrack(() => {
                if (!guard.submitting) {
                    sawNavigation = false;
                } else if (destination) {
                    sawNavigation = true;
                } else if (sawNavigation) {
                    sawNavigation = false;
                    guard.release();
                }
            });
        });
    });

    form.addEventListener('submit', onSubmit, { capture: true });
    window.addEventListener('pageshow', onPageShow);

    return {
        update(next: IGuardSubmitOptions = {}): void {
            const storeChanged = next.submitting !== options.submitting;
            options = next;
            if (storeChanged) watchSuperForm();
        },
        destroy(): void {
            form.removeEventListener('submit', onSubmit, { capture: true });
            window.removeEventListener('pageshow', onPageShow);
            unsubscribe();
            stopWatchingNavigation();
            // A form that goes away while sending (its answer sent the page elsewhere) can no longer
            // be found here to be released, and the count of forms awaiting an answer must not keep it.
            guard.release();
            if (guards.get(form) === guard) guards.delete(form);
        }
    };
}

/**
 * Wraps a SubmitFunction for SvelteKit's `enhance` so the form's guard ends exactly when the
 * submission does: after the callback it returned has finished (the default `update()` when it
 * returned none), or at once when it cancelled, threw, or the request was aborted.
 *
 * `GuardedForm` applies it for you. Call it yourself only with `use:guardSubmit` on a raw form:
 * `use:guardSubmit={{ enhanced: true }} use:enhance={guardEnhance(mySubmit)}`.
 */
export function guardEnhance(submit?: SubmitFunction): SubmitFunction {
    return async (input) => {
        const release = (): void => formGuardOf(input.formElement)?.release();
        input.controller.signal.addEventListener('abort', release, { once: true });

        let cancelled = false;
        const cancel = (): void => {
            cancelled = true;
            input.cancel();
        };

        let callback: Awaited<ReturnType<SubmitFunction>>;
        try {
            callback = await submit?.({ ...input, cancel });
        } catch (error) {
            release();
            throw error;
        }
        if (cancelled) {
            release();
            return;
        }

        return async (opts) => {
            try {
                if (typeof callback === 'function') await callback(opts);
                else await opts.update();
            } finally {
                release();
            }
        };
    };
}

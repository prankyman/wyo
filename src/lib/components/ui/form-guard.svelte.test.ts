import type { SubmitFunction } from '@sveltejs/kit';
import { writable } from 'svelte/store';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { resetPage } from '$test/support/page-state.svelte';
import { formGuardOf, formsAwaitingAnswer, guardEnhance, guardSubmit } from './form-guard.svelte';
import { pageChange } from './page-change.svelte';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

/**
 * The guard as an action, on a form that is NOT in the document. That is on purpose: a submit event
 * dispatched on it reaches no document listener, so nothing cancels it — exactly what a plain form
 * looks like in a browser, and what `src/test/setup.ts`'s document-wide safety net hides in every
 * component test.
 */
function plainForm() {
    const form = document.createElement('form');
    const button = document.createElement('button');
    button.type = 'submit';
    form.append(button);
    return { form, button };
}

function submit(form: HTMLFormElement, submitter: HTMLElement | null): SubmitEvent {
    const event = new SubmitEvent('submit', { cancelable: true, submitter });
    form.dispatchEvent(event);
    return event;
}

const afterListeners = () => new Promise((resolve) => setTimeout(resolve, 0));

function pageshow(persisted: boolean): void {
    const event = new Event('pageshow');
    Object.defineProperty(event, 'persisted', { value: persisted });
    window.dispatchEvent(event);
}

afterEach(() => {
    pageChange.nativeSubmission = false;
    resetPage();
});

describe('guardSubmit', () => {
    it('lets a plain form go once, marks the page as being replaced, and refuses every further submission', async () => {
        const { form, button } = plainForm();
        const action = guardSubmit(form);

        const first = submit(form, button);
        await afterListeners();
        const second = submit(form, button);

        expect(first.defaultPrevented).toBe(false);
        expect(formGuardOf(form)?.submitter).toBe(button);
        expect(pageChange.nativeSubmission).toBe(true);
        expect(second.defaultPrevented).toBe(true);

        action.destroy();
        expect(formGuardOf(form)).toBeUndefined();
    });

    it('refuses a submission of any form while another form is loading its answer as a page', async () => {
        const one = plainForm();
        const other = plainForm();
        const actions = [guardSubmit(one.form), guardSubmit(other.form)];

        submit(one.form, one.button);
        await afterListeners();

        expect(submit(other.form, other.button).defaultPrevented).toBe(true);
        actions.forEach((action) => action.destroy());
    });

    it('releases the form when the page comes back from the back/forward cache, and only then', async () => {
        const { form, button } = plainForm();
        const action = guardSubmit(form);
        submit(form, button);
        await afterListeners();

        pageshow(false);
        expect(formGuardOf(form)?.submitting).toBe(true);
        pageshow(true);
        expect(formGuardOf(form)?.submitting).toBe(false);
        action.destroy();
    });

    it('follows a new superForm store after an update', async () => {
        const { form, button } = plainForm();
        const first = writable(false);
        const second = writable(false);
        const action = guardSubmit(form, { enhanced: true, submitting: first });
        action.update({ enhanced: true, submitting: second });

        form.addEventListener('submit', (event) => event.preventDefault());
        submit(form, button);
        second.set(true);
        await afterListeners();
        expect(formGuardOf(form)?.submitting).toBe(true);

        second.set(false);
        expect(formGuardOf(form)?.submitting).toBe(false);
        action.destroy();
    });
});

describe('formsAwaitingAnswer', () => {
    it('counts the forms that were sent and have no answer yet, once each, and lets go of a form that goes away while sending', () => {
        const one = plainForm();
        const other = plainForm();
        const actions = [guardSubmit(one.form), guardSubmit(other.form)];
        expect(formsAwaitingAnswer()).toBe(0);

        formGuardOf(one.form)?.start(null);
        formGuardOf(one.form)?.start(null);
        formGuardOf(other.form)?.start(null);
        expect(formsAwaitingAnswer()).toBe(2);

        formGuardOf(one.form)?.release();
        formGuardOf(one.form)?.release();
        expect(formsAwaitingAnswer()).toBe(1);

        // The page leaves while this form is still waiting (its answer redirected): nothing will release it.
        actions[1].destroy();
        expect(formsAwaitingAnswer()).toBe(0);
        actions[0].destroy();
    });
});

describe('guardEnhance', () => {
    function enhanceInput(form: HTMLFormElement) {
        return {
            action: new URL('http://localhost/?/save'),
            formData: new FormData(form),
            formElement: form,
            controller: new AbortController(),
            submitter: null,
            cancel: vi.fn()
        };
    }

    it('ends the submission and passes the error on when the SubmitFunction throws', async () => {
        const { form } = plainForm();
        const action = guardSubmit(form, { enhanced: true });
        formGuardOf(form)?.start(null);
        const broken: SubmitFunction = () => {
            throw new Error('no form data');
        };

        await expect(guardEnhance(broken)(enhanceInput(form))).rejects.toThrow('no form data');
        expect(formGuardOf(form)?.submitting).toBe(false);
        action.destroy();
    });

    it('ends the submission when its request is aborted', async () => {
        const { form } = plainForm();
        const action = guardSubmit(form, { enhanced: true });
        formGuardOf(form)?.start(null);
        const input = enhanceInput(form);

        await guardEnhance()(input);
        expect(formGuardOf(form)?.submitting).toBe(true);
        input.controller.abort();

        expect(formGuardOf(form)?.submitting).toBe(false);
        action.destroy();
    });
});

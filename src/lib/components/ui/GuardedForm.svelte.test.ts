import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { writable } from 'svelte/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Harness from '$test/harnesses/ui/GuardedFormTestHarness.svelte';
import { resetPage, setNavigating } from '$test/support/page-state.svelte';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

/** Lets the guard's `setTimeout(0)` — "who took this submission over?" — run. */
const afterListeners = () => new Promise((resolve) => setTimeout(resolve, 0));

const button = (testId: string) => screen.getByTestId(testId);
const form = () => screen.getByTestId('harness-form') as HTMLFormElement;

/**
 * Every submission that got past the guard, as the server would receive it.
 *
 * What the browser does next with a plain form — load its answer as a new page — cannot be seen
 * here: `src/test/setup.ts` cancels every submission at the document, because jsdom cannot load a
 * page. That half is proven on a form outside the document in `form-guard.svelte.test.ts`.
 */
let sent: FormData[] = [];
const record = (event: Event) => {
    const submit = event as SubmitEvent;
    sent.push(new FormData(submit.target as HTMLFormElement, submit.submitter));
};

beforeEach(() => {
    sent = [];
    document.addEventListener('submit', record);
});

afterEach(() => {
    document.removeEventListener('submit', record);
    resetPage();
});

describe('GuardedForm — a plain form', () => {
    it('is sent once: the button that sent it works and keeps its name and value, every further submission is refused', async () => {
        render(Harness);

        await fireEvent.click(button('save'));

        expect(sent).toHaveLength(1);
        expect(sent[0].get('intent')).toBe('save');
        expect(sent[0].get('note')).toBe('hello');
        expect(button('save')).toHaveAttribute('aria-busy', 'true');
        expect(button('save')).not.toBeDisabled();
        expect(screen.getAllByTestId('button-spinner')).toHaveLength(1);
        // The form's other submit buttons, inside and outside it, are blocked without a spinner.
        expect(button('other')).toHaveAttribute('aria-disabled', 'true');
        expect(button('outside')).toHaveAttribute('aria-disabled', 'true');
        expect(form()).toHaveAttribute('aria-busy', 'true');

        await fireEvent.click(button('save'));
        await fireEvent.click(button('outside'));
        form().requestSubmit();
        await afterListeners();

        expect(sent).toHaveLength(1);
    });

    it('is not guarded when its answer opens in another window, because this page stays as it is', async () => {
        // `target` is also a Svelte mount option, so the props go under `props`.
        render(Harness, { props: { target: '_blank' } });

        await fireEvent.click(button('save'));
        await afterListeners();

        expect(sent).toHaveLength(1);
        expect(button('save')).not.toHaveAttribute('aria-busy');
        expect(button('other')).not.toHaveAttribute('aria-disabled');
    });

    it('sent from a button outside it (form="…") shows that button working', async () => {
        render(Harness);

        await fireEvent.click(button('outside'));

        expect(sent).toHaveLength(1);
        expect(button('outside')).toHaveAttribute('aria-busy', 'true');
        expect(button('save')).not.toHaveAttribute('aria-busy');
    });
});

describe('GuardedForm — a superForm', () => {
    it("is enhanced by the superForm and ends when the superForm's `submitting` returns to false", async () => {
        const submitting = writable(false);
        // What a superForm's `enhance` does on submit: it takes the submission over and says so.
        const enhance = vi.fn((element: HTMLFormElement) => {
            const onSubmit = (event: Event) => {
                event.preventDefault();
                submitting.set(true);
            };
            element.addEventListener('submit', onSubmit);
            return { destroy: () => element.removeEventListener('submit', onSubmit) };
        });
        render(Harness, { superform: { enhance, submitting } });

        expect(enhance).toHaveBeenCalledWith(form());
        await fireEvent.click(button('save'));
        await fireEvent.click(button('save'));
        await afterListeners();

        expect(sent).toHaveLength(1);
        expect(button('save')).toHaveAttribute('aria-busy', 'true');

        submitting.set(false);
        await waitFor(() => expect(button('save')).not.toHaveAttribute('aria-busy'));
    });

    // A form that keeps `method="GET"` in its markup is handed to superforms all the same.
    it('takes a superForm on a GET form and guards its press until `submitting` returns to false', async () => {
        const submitting = writable(false);
        const enhance = vi.fn((element: HTMLFormElement) => {
            const onSubmit = (event: Event) => {
                event.preventDefault();
                submitting.set(true);
            };
            element.addEventListener('submit', onSubmit);
            return { destroy: () => element.removeEventListener('submit', onSubmit) };
        });
        render(Harness, { method: 'GET', superform: { enhance, submitting } });

        expect(enhance).toHaveBeenCalledWith(form());
        expect(form()).toHaveAttribute('method', 'GET');
        await fireEvent.click(button('save'));
        await afterListeners();
        expect(button('save')).toHaveAttribute('aria-busy', 'true');

        submitting.set(false);
        await waitFor(() => expect(button('save')).not.toHaveAttribute('aria-busy'));
    });
});

describe('GuardedForm — handed to the router', () => {
    it('a GET form ends when the navigation it started ends', async () => {
        // What SvelteKit's router does with a GET form: it cancels the page load and navigates.
        const route = (event: Event) => {
            event.preventDefault();
            setNavigating('/list?note=hello');
        };
        document.addEventListener('submit', route);
        render(Harness, { method: 'GET' });

        await fireEvent.click(button('save'));
        await afterListeners();
        expect(button('save')).toHaveAttribute('aria-busy', 'true');

        setNavigating(null);
        await waitFor(() => expect(button('save')).not.toHaveAttribute('aria-busy'));
        document.removeEventListener('submit', route);
    });

    it('a submission cancelled by other code that starts nothing is released at once', async () => {
        const swallow = (event: Event) => event.preventDefault();
        document.addEventListener('submit', swallow);
        render(Harness, { method: 'GET' });

        await fireEvent.click(button('save'));
        await afterListeners();

        expect(button('save')).not.toHaveAttribute('aria-busy');
        document.removeEventListener('submit', swallow);
    });
});

describe('GuardedForm — a confirmation window', () => {
    const confirm = { title: 'Delete the thing?', message: 'It disappears.', confirmText: 'Delete' };

    it('opens the window on the press and sends nothing; «Отмена» closes it and still sends nothing', async () => {
        render(Harness, { confirm });

        await fireEvent.click(button('save'));

        expect(screen.getByRole('dialog')).toBeInTheDocument();
        expect(screen.getByText('Delete the thing?')).toBeInTheDocument();
        expect(sent).toHaveLength(0);
        expect(form()).not.toHaveAttribute('aria-busy');

        await fireEvent.click(button('harness-form-modal-cancel'));

        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(sent).toHaveLength(0);
    });

    it('sends the form once when the window is confirmed, with the pressed fields', async () => {
        render(Harness, { confirm });

        await fireEvent.click(button('save'));
        await fireEvent.click(button('harness-form-modal-confirm'));

        expect(sent).toHaveLength(1);
        expect(sent[0].get('note')).toBe('hello');
    });

    it('asks again on the next press after the window was cancelled', async () => {
        render(Harness, { confirm });

        await fireEvent.click(button('save'));
        await fireEvent.click(button('harness-form-modal-cancel'));
        await fireEvent.click(button('save'));

        expect(screen.getByRole('dialog')).toBeInTheDocument();
        expect(sent).toHaveLength(0);
    });

    it('closes the window when the answer is in (the superform stops sending)', async () => {
        const submitting = writable(false);
        render(Harness, { confirm, superform: { enhance: () => ({ destroy() {} }), submitting } });

        await fireEvent.click(button('save'));
        submitting.set(true);
        await afterListeners();
        submitting.set(false);

        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('sends at once, with no window, when it has no confirm', async () => {
        render(Harness);

        await fireEvent.click(button('save'));

        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(sent).toHaveLength(1);
    });
});

import { afterEach } from 'vitest';
import { resetPageState, setPageState } from './stores';

/**
 * Minimal shape of SvelteKit's ActionResult, as much as the form library's single-page mode produces and reads: it builds
 * its own result after the browser-side check and hands it to `applyAction`.
 */
export interface IMockActionResult {
    type: 'success' | 'failure' | 'redirect' | 'error';
    status?: number;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    data?: any;
    location?: string;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    error?: any;
}

afterEach(() => {
    resetPageState();
});

interface ISubmitFunctionArgs {
    action: URL;
    cancel: () => void;
    controller: AbortController;
    formData: FormData;
    formElement: HTMLFormElement;
    submitter: HTMLElement | null;
}

type TSubmitFunction = (args: ISubmitFunctionArgs) => unknown;

/**
 * Test mock of SvelteKit's `enhance` action from `$app/forms`, which the form library uses for every form.
 *
 * Matches the real `enhance(form_element, submit)` signature. When the form is submitted (e.g. via
 * `fireEvent.submit(formElement)`), it prevents the browser's own submission and calls the form library's submit function. In
 * the single-page mode the library cancels the request itself after its browser-side check and answers through
 * `applyAction`, so nothing here ever sends or answers anything: there is no server.
 */
export function enhance(formElement: HTMLFormElement, submit: TSubmitFunction = () => undefined): { destroy(): void } {
    async function handleSubmit(event: SubmitEvent) {
        event.preventDefault();

        const submitter = (event.submitter as HTMLElement | null) ?? null;
        const formData = new FormData(formElement, submitter ?? undefined);
        const action = new URL(formElement.action || window.location.href);
        const controller = new AbortController();

        await submit({ action, cancel: () => undefined, controller, formData, formElement, submitter });
    }

    formElement.addEventListener('submit', handleSubmit);

    return {
        destroy() {
            formElement.removeEventListener('submit', handleSubmit);
        }
    };
}

/**
 * Test mock of SvelteKit's `deserialize` from `$app/forms`: the form library imports it. The wyo interface has no form action
 * to deserialize, so a bare `JSON.parse` stands in.
 */
export function deserialize(text: string): IMockActionResult {
    return JSON.parse(text) as IMockActionResult;
}

/**
 * Mock of SvelteKit's `applyAction` from `$app/forms`.
 *
 * For a `success` or `failure` result SvelteKit updates the `$page` store so that the form library's `page.subscribe` can
 * detect the returned form state and refresh its stores (`$message`, `$errors`, `$form`, etc.). In tests we mirror that by
 * updating the mocked `$page` state through the same mechanism, without exposing a writable `page` store to consumers.
 */
export async function applyAction(result?: IMockActionResult): Promise<void> {
    if (!result) return;

    const status = result.status ?? (result.type === 'success' ? 200 : result.type === 'failure' ? 400 : 500);

    if (result.type === 'success' || result.type === 'failure') {
        setPageState({ status, form: result.data });
    } else if (result.type === 'error') {
        setPageState({
            status,
            error: result.error instanceof Error ? result.error : new Error(String(result.error))
        });
    }
}

import { fireEvent } from '@testing-library/svelte';
import { tick } from 'svelte';

/** Lets the answer of a sent form run through the form library's callbacks. */
export const settle = (): Promise<unknown> => new Promise((resolve) => setTimeout(resolve, 0));

/** Lets Svelte run its pending updates `times` times over: a form whose answer re-renders twice before it shows. */
export async function flushTicks(times = 4): Promise<void> {
    for (let i = 0; i < times; i += 1) await tick();
}

/**
 * Sends a real superform in a component's test. The form is in the single-page mode: its own check in the browser runs first
 * (a test fills the boxes the schema requires before it sends), then the form's `send` stub answers - there is no server, so
 * the "answer" of a test is whatever its `send` stub returns. Resolves when the answer has run through the library.
 */
export async function sendForm(form: Element): Promise<void> {
    await fireEvent.submit(form);
    await settle();
}

import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SuperForm } from 'sveltekit-superforms';
import { messages } from '$lib/i18n/messages';
import type { TApiResult } from '$lib/api/api';
import SpaFormTestHarness from '$test/harnesses/forms/SpaFormTestHarness.svelte';
import { toast } from '$lib/stores/toast.svelte';

const answering = vi.hoisted(() => ({ answered: vi.fn() }));
vi.mock('$lib/components/ui/disclosure-form', () => ({ disclosureFormOf: () => answering }));

import { applyRefusal, defaults, setMessage, zod4 } from './super-form';
import { z } from 'zod';
import { get } from 'svelte/store';

type TMade = SuperForm<{ name: string }>;

const text = (key: string): string => messages.ru[key];
const invalid = text('errors.invalid');

/** Draws the form with a name typed in, and sends it. */
async function typeAndSend(name: string, id = 'spa-form'): Promise<void> {
    await fireEvent.input(screen.getByTestId(`${id}-name`), { target: { value: name } });
    await fireEvent.submit(screen.getByTestId(id));
}

const answer = (id = 'spa-form') => screen.getByTestId(`${id}-answer`);

beforeEach(() => {
    toast.clear();
    answering.answered.mockClear();
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('superForm in the single-page mode', () => {
    it('sends the values of a form that passed the browser check and answers that it saved', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} }));
        render(SpaFormTestHarness, { props: { options: { send } } });

        await typeAndSend('ab');

        await vi.waitFor(() => expect(answer()).toHaveTextContent('saved'));
        expect(send).toHaveBeenCalledExactlyOnceWith({ name: 'ab' });
    });

    it('answers with the success code the form names', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} }));
        render(SpaFormTestHarness, { props: { options: { send, saved: 'renamed' } } });

        await typeAndSend('ab');

        await vi.waitFor(() => expect(answer()).toHaveTextContent('renamed'));
    });

    it('does not send a form the browser check refuses, and shows the schema’s message under the box', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} }));
        render(SpaFormTestHarness, { props: { options: { send } } });

        await typeAndSend('a');

        await vi.waitFor(() => expect(screen.getByTestId('spa-form-name-error')).toHaveTextContent(invalid));
        expect(send).not.toHaveBeenCalled();
    });

    it('keeps a value typed right after a press its own check refused, instead of putting the pressed value back', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} }));
        let made: TMade | undefined;
        render(SpaFormTestHarness, { props: { options: { send }, onMade: (form: TMade) => (made = form) } });

        // The next value goes in before the refused press has finished (a browser test on a fast machine did it within 2 ms).
        await typeAndSend('a');
        await fireEvent.input(screen.getByTestId('spa-form-name'), { target: { value: 'abc' } });

        await vi.waitFor(() => expect(made && get(made.submitting)).toBe(false));
        await new Promise((resolve) => setTimeout(resolve, 50));
        expect(screen.getByTestId('spa-form-name')).toHaveValue('abc');
        expect(made && get(made.form).name).toBe('abc');
        expect(send).not.toHaveBeenCalled();
    });

    it('lays a refusal of the wyo server onto the box it names and onto the form’s answer', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({
            ok: false,
            status: 422,
            refusal: { code: 'not_found', issues: [{ path: 'name', code: 'invalid' }] }
        }));
        render(SpaFormTestHarness, { props: { options: { send } } });

        await typeAndSend('ab');

        await vi.waitFor(() => expect(answer()).toHaveTextContent('errors.not_found'));
        expect(screen.getByTestId('spa-form-name-error')).toHaveTextContent(invalid);
    });

    it('answers a call that never arrived with the refusal for it', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({
            ok: false,
            status: 0,
            refusal: { code: 'network' }
        }));
        render(SpaFormTestHarness, { props: { options: { send } } });

        await typeAndSend('ab');

        await vi.waitFor(() => expect(answer()).toHaveTextContent('errors.network'));
        expect(toast.toasts).toHaveLength(0);
    });

    it('cancels every press of a learning sample before anything is checked or sent', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} }));
        render(SpaFormTestHarness, { props: { options: { send, sample: true } } });

        await typeAndSend('ab');
        await new Promise((resolve) => setTimeout(resolve, 20));

        expect(send).not.toHaveBeenCalled();
        expect(answer()).toHaveTextContent('');
    });

    it('tells the person when the submission itself fails, and keeps a form’s own handler for that', async () => {
        const failing = vi.fn(async (): Promise<TApiResult<unknown>> => {
            throw new Error('boom');
        });
        render(SpaFormTestHarness, { props: { options: { send: failing } } });

        await typeAndSend('ab');

        await vi.waitFor(() => expect(toast.toasts).toHaveLength(1));
        expect(toast.toasts[0].title).toBe(text('common.requestFailed'));

        toast.clear();
        const own = vi.fn();
        render(SpaFormTestHarness, { props: { id: 'own', options: { send: failing, onError: own } } });
        await typeAndSend('ab', 'own');

        await vi.waitFor(() => expect(own).toHaveBeenCalled());
        expect(toast.toasts).toHaveLength(0);
    });

    it('lets the form’s own onUpdate refuse before anything is sent', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} }));
        const refuse = vi.fn(({ form }: { form: { valid: boolean } }) => {
            form.valid = false;
        });
        render(SpaFormTestHarness, { props: { options: { send, onUpdate: refuse as never } } });

        await typeAndSend('ab');

        await vi.waitFor(() => expect(refuse).toHaveBeenCalled());
        expect(send).not.toHaveBeenCalled();
    });

    it('checks the whole form when a box is left, and shows the message of the box just left', async () => {
        render(SpaFormTestHarness);

        const box = screen.getByTestId('spa-form-name');
        await fireEvent.input(box, { target: { value: 'a' } });
        await fireEvent.focusOut(box);

        await vi.waitFor(() => expect(screen.getByTestId('spa-form-name-error')).toHaveTextContent(invalid));

        await fireEvent.input(box, { target: { value: 'abc' } });
        await fireEvent.focusOut(box);

        await vi.waitFor(() => expect(screen.getByTestId('spa-form-name-error')).toHaveTextContent(''));
    });

    it('clears the answers of the other forms of its group when one is sent', async () => {
        const send = vi.fn(async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} }));
        const made: TMade[] = [];
        const onMade = (form: unknown) => made.push(form as TMade);
        render(SpaFormTestHarness, { props: { id: 'first', options: { send, answerGroup: 'g' }, onMade } });
        render(SpaFormTestHarness, { props: { id: 'second', options: { send, answerGroup: 'g' }, onMade } });
        made[1].message.set({ saved: 'older' } as never);
        await vi.waitFor(() => expect(answer('second')).toHaveTextContent('older'));

        await typeAndSend('ab', 'first');

        await vi.waitFor(() => expect(answer('first')).toHaveTextContent('saved'));
        expect(answer('second')).toHaveTextContent('');
    });

    it('tells the DisclosureForm around it every answer, unless told not to', async () => {
        const ok = vi.fn(async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} }));
        const refused = vi.fn(async (): Promise<TApiResult<unknown>> => ({
            ok: false,
            status: 409,
            refusal: { code: 'conflict' }
        }));
        render(SpaFormTestHarness, { props: { id: 'good', options: { send: ok } } });
        await typeAndSend('ab', 'good');
        await vi.waitFor(() => expect(answering.answered).toHaveBeenCalledWith(true));

        render(SpaFormTestHarness, { props: { id: 'bad', options: { send: refused } } });
        await typeAndSend('ab', 'bad');
        await vi.waitFor(() => expect(answering.answered).toHaveBeenCalledWith(false));

        answering.answered.mockClear();
        render(SpaFormTestHarness, { props: { id: 'quiet', options: { send: ok, disclosure: false } } });
        await typeAndSend('ab', 'quiet');
        await vi.waitFor(() => expect(answer('quiet')).toHaveTextContent('saved'));
        expect(answering.answered).not.toHaveBeenCalled();
    });
});

describe('applyRefusal', () => {
    const schema = z.object({ name: z.string(), lines: z.array(z.object({ qty: z.number() })) });
    const fresh = () => defaults({ name: 'a', lines: [{ qty: 1 }] }, zod4(schema));

    it('puts each issue under its field, also a field inside a list, and the code into the form’s answer', () => {
        const form = fresh();

        applyRefusal(form, {
            code: 'invalid',
            params: { max: 3, ignored: { nested: true } },
            issues: [
                { path: 'name', code: 'invalid' },
                { path: 'lines[0].qty', code: 'never_heard_of_it' }
            ]
        });

        expect(form.valid).toBe(false);
        expect(form.errors.name).toEqual(['errors.invalid']);
        expect(form.errors.lines?.[0]?.qty).toEqual(['errors.invalid']);
        expect(form.message).toEqual({ error: 'errors.invalid', params: { max: 3 } });
    });

    it('words the codes from the family of texts it is given, and the rest from the general texts', () => {
        const form = fresh();

        applyRefusal(form, { code: 'key_refused', issues: [{ path: 'name', code: 'required' }] }, 'accounts.refusal');

        expect(form.errors.name).toEqual(['errors.required']);
        expect(form.message).toEqual({ error: 'accounts.refusal.key_refused', params: undefined });
    });

    it('makes a refusal with no issues an answer of the form and nothing else', () => {
        const form = fresh();

        applyRefusal(form, { code: 'conflict' });

        expect(form.valid).toBe(false);
        expect(form.errors).toEqual({});
        expect(form.message).toEqual({ error: 'errors.unknown', params: undefined });
    });

    it('can be followed by a success answer of the same form for the next try', () => {
        const form = fresh();
        applyRefusal(form, { code: 'conflict' });
        form.valid = true;

        setMessage(form, { saved: 'saved' });

        expect(form.message).toEqual({ saved: 'saved' });
    });
});

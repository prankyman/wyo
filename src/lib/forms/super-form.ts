import { defaults, setError, setMessage, superForm as baseSuperForm } from 'sveltekit-superforms';
import type { FormOptions, Infer, InferIn, SuperForm, SuperValidated } from 'sveltekit-superforms';
import { zod4, zod4Client, type ZodValidationSchema } from 'sveltekit-superforms/adapters';
import { onDestroy } from 'svelte';
import { get } from 'svelte/store';
import { refusalParams, type TApiResult, type TRefusal } from '$lib/api/api';
import { disclosureFormOf } from '$lib/components/ui/disclosure-form';
import { fieldRefusalKey, refusalKey } from '$lib/api/refusal-key';
import { getLocale, t } from '$lib/i18n/i18n';
import { toast } from '$lib/stores/toast.svelte';
import { answerOf } from './answer';
import { collectErrors, pathKey, refreshedErrors, serverOnlyErrors } from './form-error-refresh';

// The ONE file that imports the form library at run time (an ESLint rule fails every other file that does). What the
// pages need of the library comes from here: the schema adapters, the starting values and the two setters.
export { defaults, setError, setMessage, superValidate } from 'sveltekit-superforms';
export { zod4, zod4Client } from 'sveltekit-superforms/adapters';

/**
 * The library's options, and the project's own: `send`, `saved`, `sample`, `answerGroup`, `disclosure`, `refusals`. The form is always in
 * the library's single-page mode (`SPA`), so the options that belong to a server-side form (`SPA`, `invalidateAll`) are left
 * out: the wyo interface has no server side to post to.
 */
export type TSuperFormOptions<
    T extends Record<string, unknown>,
    M = unknown,
    In extends Record<string, unknown> = T
> = Omit<FormOptions<T, M, In>, 'SPA' | 'invalidateAll'> & {
    /**
     * Sends a form that passed the browser's check to the wyo server, through `src/lib/api/` (the one place `fetch` is
     * called). The form's values go as JSON. A success sets the form's answer to `{ saved }`; a refusal is laid onto the
     * fields and the form's answer by `applyRefusal`. The wyo server checks the values again and is the authority.
     */
    send?: (data: T) => Promise<TApiResult<unknown>>;
    /** The short code of the success answer (`{ saved }`, drawn by `FormAnswer`); `'saved'` by default. */
    saved?: string;
    /**
     * A learning sample: the form is drawn as on the real screen but every press is cancelled before anything is checked or
     * sent, so a learner can press it without changing anything.
     */
    sample?: boolean;
    /**
     * The forms of one screen that share one answer at a time: sending any of them clears the others' messages, so «Сохранено»
     * or a refusal is only ever under the button just pressed.
     */
    answerGroup?: string;
    /**
     * `false`: the form does not tell the `DisclosureForm` around it about its answers - a check inside an edit form whose
     * success must not close the form it sits in.
     */
    disclosure?: false;
    /**
     * The family of texts that words this form's refusals before the general `errors.<code>` (`accounts.refusal`: the
     * refusals of a marketplace key have texts of their own). See `refusalKey`.
     */
    refusals?: string;
};

/** The live forms of each answer group (`answerGroup`). */
const answerGroups = new Map<string, Set<{ message: { set(value: undefined): void } }>>();

/**
 * Lays the wyo server's refusal `{ error: { code, params?, issues?: [{ path, code }] } }` onto a form: every issue becomes the
 * message of its field (a key of `messages/`, drawn by `tError`), and the refusal itself becomes the form's answer
 * `{ error: <key>, params }`, which `FormAnswer` draws under the form's button. The ONE function that does this: no page
 * reads a refusal by itself. The form is made invalid.
 */
export function applyRefusal<T extends Record<string, unknown>>(
    form: SuperValidated<T>,
    refusal: TRefusal,
    family?: string
): void {
    for (const issue of refusal.issues ?? []) {
        setError(form, issue.path as never, fieldRefusalKey(issue.code, family));
    }
    setMessage(form, { error: refusalKey(refusal.code, family), params: refusalParams(refusal) }, { status: 400 });
}

/** The success answer of a form sent by `send`: `{ saved }`, drawn by `FormAnswer`. */
function setSaved<T extends Record<string, unknown>>(form: SuperValidated<T>, saved: string): void {
    setMessage(form, { saved });
}

/**
 * The project's `superForm`. Import this everywhere instead of the one from `sveltekit-superforms`; an ESLint rule enforces it.
 *
 * The form is in the library's single-page mode: it is checked in the browser by its `validators` (a zod schema through
 * `zod4Client`) and never posted to a page. What is sent is decided by `send` (see `TSuperFormOptions`).
 *
 * It adds these things.
 *
 * **1. An answer to "the submission never came back".** Without an `onError` of its own, the library rethrows the caught
 * exception, which becomes an unhandled promise rejection: the console gets a stack trace and **the person is shown nothing
 * at all** - the button just stops doing anything. A form with no `onError` of its own gets one that says the request did not
 * go through and asks the person to retry (a form that CAN say something more specific passes its own and keeps it). A call
 * made with `send` does not throw: its failure is a refusal laid onto the form.
 *
 * **2. Leaving a field re-checks the whole form.** Every field calls `validate('field', { update: 'errors' })` when it loses
 * focus. On its own that updates one message and leaves every other message on screen untouched, however untrue it has
 * become - see `form-error-refresh.ts` for the ways that happens and for the rules that decide what a re-check may change.
 * This wrapper replaces `validate` so every such call refreshes the form's messages, at no extra cost, since the library's own
 * `validate` already ran the whole schema to answer about one field. A call that asks for something else - `update: false`,
 * a `value` to test, a message to force, another `schema` - is handed to the library's own `validate` unchanged.
 *
 * The same refresh also runs on every `focusout` inside the `<form>` the form's `enhance` is on, the way the library's own
 * `'onblur'` mode listened. Per-field handlers cannot be relied on alone: a field rendered by a component that does not
 * forward `onfocusout` is silently never checked.
 *
 * **What a save does to the boxes is not decided here.** The library's `resetForm` defaults to `true`: after a successful save
 * it puts the boxes back to what they had when the form was made (https://superforms.rocks/concepts/enhance#resetform), so an
 * edit form that stays on the page showed the old values again. This wrapper leaves the option alone - a form that adds a
 * record and wants empty boxes for the next one is right with `true` - but every call must state it, `resetForm: true`,
 * `false` or a function. `super-form-reset-policy.test.ts` fails the build on a call that does not.
 *
 * **3. One answer at a time** (`answerGroup`), **4. learning samples** (`sample: true`) and **5.** A form made inside a
 * `DisclosureForm` tells it every answer - see `TSuperFormOptions`.
 *
 * The form's message (its «Сохранено» or refusal, drawn by `FormAnswer`) is cleared the moment the form is sent again - on every
 * press, also one the browser's own check stops - besides the library's own `clearOnSubmit: 'message'`.
 */
export function superForm<
    T extends Record<string, unknown> = Record<string, unknown>,
    M = unknown,
    In extends Record<string, unknown> = T
>(form: SuperValidated<T, M, In> | T, options?: TSuperFormOptions<T, M, In>): SuperForm<T, M> {
    const {
        send,
        saved = 'saved',
        answerGroup,
        sample,
        disclosure,
        refusals,
        ...formOptions
    }: TSuperFormOptions<T, M, In> = options ?? {};
    const around = disclosure === false ? undefined : disclosureFormOf();
    const group = answerGroup ? (answerGroups.get(answerGroup) ?? new Set()) : null;
    const instance = baseSuperForm<T, M, In>(form, {
        ...formOptions,
        SPA: true,
        ...((group || sample) && {
            onSubmit: (input) => {
                if (sample) {
                    input.cancel();
                    return;
                }
                if (group) for (const other of group) if (other !== instance) other.message.set(undefined);
                return formOptions.onSubmit?.(input);
            }
        }),
        // The form's own check ran in the browser before this: `onUpdate` is where the values are sent. The form's own
        // handler comes first and may refuse (a rule only it knows); a refused form is not sent.
        ...(send && {
            onUpdate: async (event) => {
                await formOptions.onUpdate?.(event);
                if (!event.form.valid) return;
                const result = await send(event.form.data);
                if (result.ok) setSaved(event.form, saved);
                else applyRefusal(event.form, result.refusal, refusals);
            }
        }),
        // `?? ` and not a merge: a form that handles its own errors has said something more useful than this could, and two
        // toasts for one failure is worse than one.
        onError: formOptions.onError ?? reportTransportFailure,
        // Composed, never replaced: the form's own handler still runs. Errors arriving from a submission are the moment to work
        // out which of them the browser cannot re-check.
        onUpdated: (event) => {
            // A saved form starts again empty (the library's `resetForm`): a box left before the press, still waiting in
            // `justLeft` (the press drew nothing), must not be checked against the emptied form later.
            if (event.form.valid && answerOf(event.form.message)?.error === undefined) {
                forgetLeftBoxes(asRefreshable(instance));
            }
            void recordServerErrors(asRefreshable(instance));
            formOptions.onUpdated?.(event);
            // A form opened by a press closes after a success or readies itself for the next row.
            around?.answered(event.form.valid && answerOf(event.form.message)?.error === undefined);
        }
    });

    if (answerGroup && group) {
        group.add(instance);
        answerGroups.set(answerGroup, group);
        onDestroy(() => {
            group.delete(instance);
            if (group.size === 0) answerGroups.delete(answerGroup);
        });
    }

    const refreshable = asRefreshable(instance);
    refreshStates.set(refreshable, {
        justLeft: new Set(),
        serverOnly: new Map(),
        recording: null,
        run: 0,
        pressing: false
    });

    const enhance = instance.enhance;
    instance.enhance = ((formElement: HTMLFormElement, events?: unknown) => {
        const onFocusOut = (event: FocusEvent) => {
            // Not while submitting: the submission is about to write its own errors, and the library skips its own blur
            // handling then for the same reason.
            if (get(instance.submitting)) return;
            const target = event.target;
            const name = target instanceof Element ? target.getAttribute('name') : null;
            void refreshErrors(refreshable, name ?? undefined);
        };
        // A press on a button of the form starts before the box loses the cursor (pointer down, then focus out) and ends with
        // the release anywhere on the page, or with the browser taking the pointer away.
        const pressState = stateOf(refreshable);
        const onPressEnd = () => {
            if (pressState) pressState.pressing = false;
        };
        const onPointerDown = (event: PointerEvent) => {
            if (!pressState || !(event.target instanceof Element) || !event.target.closest('button')) return;
            pressState.pressing = true;
            document.addEventListener('pointerup', onPressEnd, { once: true, capture: true });
            document.addEventListener('pointercancel', onPressEnd, { once: true, capture: true });
        };
        formElement.addEventListener('pointerdown', onPointerDown, true);
        // Every press clears the form's answer, also one the browser stops before anything is sent - the library's own check,
        // or the box's own limits (`max`, `required`), which fire `invalid` instead of `submit`: an earlier «Сохранено» must
        // not stand beside a value just refused.
        const onSubmit = () => instance.message.set(undefined as never);
        formElement.addEventListener('focusout', onFocusOut);
        formElement.addEventListener('submit', onSubmit);
        formElement.addEventListener('invalid', onSubmit, true);
        const action = enhance(formElement, events as never);
        return {
            destroy: () => {
                formElement.removeEventListener('focusout', onFocusOut);
                formElement.removeEventListener('pointerdown', onPointerDown, true);
                formElement.removeEventListener('submit', onSubmit);
                formElement.removeEventListener('invalid', onSubmit, true);
                action?.destroy?.();
            }
        };
    }) as typeof instance.enhance;

    const reset = instance.reset;
    instance.reset = ((resetOptions?: unknown) => {
        // A reset clears the messages on purpose. Anything still being checked belongs to the form as it was before, so it
        // must not be allowed to write its answer afterwards - that is how an error used to reappear on a form just closed.
        const state = stateOf(refreshable);
        if (state) {
            state.run++;
            state.justLeft.clear();
            state.serverOnly.clear();
        }
        return reset(resetOptions as never);
    }) as typeof instance.reset;

    const validate = instance.validate;
    instance.validate = ((path: string, opts: IValidateOptions = {}) => {
        if (!canRefresh(refreshable, opts)) return validate(path as never, opts as never);
        return refreshErrors(refreshable, path);
    }) as typeof instance.validate;

    // A form can be built with errors already on it. Those are server errors too.
    if (collectErrors(get(instance.errors)).size > 0) void recordServerErrors(refreshable);

    return instance;
}

/**
 * A form made from a zod schema in one call, for a page that has no data to load first: the starting values (`initial`, not
 * checked until the person leaves a box or sends), the browser-side check and the single-page mode. The schema's messages are
 * keys of `messages/`.
 */
export function spaForm<S extends ZodValidationSchema>(
    schema: S,
    initial?: Partial<Infer<S, 'zod4'>>,
    options?: TSuperFormOptions<Infer<S, 'zod4'>, unknown, InferIn<S, 'zod4'>>
): SuperForm<Infer<S, 'zod4'>> {
    const start = initial ? defaults(initial as never, zod4(schema)) : defaults(zod4(schema));
    return superForm(start as never, { validators: zod4Client(schema), ...options } as never) as never;
}

/** One internal shape for every form, whatever its data type; only the stores below are read. */
type TRefreshableForm = SuperForm<Record<string, unknown>>;

function asRefreshable<T extends Record<string, unknown>, M>(form: SuperForm<T, M>): TRefreshableForm {
    return form as unknown as TRefreshableForm;
}

function reportTransportFailure(): void {
    // `getLocale()` resolves the language the person is actually looking at, from the page's address.
    toast.error(t('common.requestFailed', getLocale()));
}

/** The options of the library's `validate`, as much of them as the decision below needs. */
interface IValidateOptions {
    value?: unknown;
    update?: boolean | 'errors' | 'value';
    errors?: string | string[];
    schema?: unknown;
}

interface IRefreshState {
    /** Fields left since the last check finished. Held until a result is applied, so a visitor
     *  tabbing faster than the schema runs still gets a message under every field they left. */
    justLeft: Set<string>;
    /** Errors the browser cannot reproduce, with the value each was raised on. */
    serverOnly: Map<string, unknown>;
    /** A classification in progress; a refresh waits for it rather than racing it. */
    recording: Promise<void> | null;
    /** Counts checks, so only the newest result is applied. */
    run: number;
    /**
     * A button of the form is held down. The check of a box the press just left writes nothing then: a message drawn under
     * the box moves the buttons below it, and the press is released beside the button and is lost. The box stays in
     * `justLeft`; a send checks the whole form anyway.
     */
    pressing: boolean;
}

const refreshStates = new WeakMap<TRefreshableForm, IRefreshState>();

function stateOf(form: TRefreshableForm): IRefreshState | undefined {
    return refreshStates.get(form);
}

/**
 * Forgets the boxes left before a save and drops any check still running: the form was saved and emptied, so a box left
 * before the press is no longer "just left" — checked against the empty form it would draw a refusal on a box the person has
 * not reached yet, and the section would move the cursor there while they type the next row (on 2026-10-25 a browser test
 * lost the values of two boxes this way).
 */
export function forgetLeftBoxes(form: TRefreshableForm): void {
    const state = stateOf(form);
    if (!state) return;
    state.run++;
    state.justLeft.clear();
}

/** Whether this `validate` call is the "the visitor left this field" call the refresh is for. */
function canRefresh(form: TRefreshableForm, opts: IValidateOptions): boolean {
    if (!form.options.validators) return false;
    if (opts.value !== undefined || opts.errors !== undefined || opts.schema !== undefined) return false;
    return opts.update === undefined || opts.update === true || opts.update === 'errors';
}

async function refreshErrors(form: TRefreshableForm, path: string | undefined): Promise<string[] | undefined> {
    const state = stateOf(form);
    if (!state || !form.options.validators) return undefined;
    if (path !== undefined) state.justLeft.add(path);

    // Only when one is actually in flight: an unconditional `await` on `null` would still
    // cost a turn of the event loop, and a field's message would land later than it used to.
    if (state.recording) await state.recording;

    const run = ++state.run;
    const result = await form.validateForm();
    const fresh = result.errors as Record<string, unknown>;

    // A newer check started while this one ran. It carries every field this one was asked
    // about — they are still in `justLeft` — so its result is this one's, only newer.
    if (run !== state.run) return messagesAt(fresh, path);
    // A button of the form is being pressed: nothing is drawn now, so nothing moves under the press (`pressing`).
    if (state.pressing) return messagesAt(fresh, path);

    const justLeft = [...state.justLeft];
    state.justLeft.clear();

    const refreshed = refreshedErrors({
        current: get(form.errors),
        fresh,
        justLeft,
        tainted: get(form.tainted),
        data: get(form.form),
        serverOnly: state.serverOnly
    });

    state.serverOnly = refreshed.serverOnly;
    form.errors.set(refreshed.errors as never);

    return messagesAt(fresh, path);
}

/**
 * Works out which errors on screen the browser's own check does not produce, and remembers the
 * value each was raised on, so `form-error-refresh.ts` can keep them until that value changes.
 */
async function recordServerErrors(form: TRefreshableForm): Promise<void> {
    const state = stateOf(form);
    if (!state || !form.options.validators) return;

    const recording = (async () => {
        const current = get(form.errors);
        if (collectErrors(current).size === 0) {
            state.serverOnly = new Map();
            return;
        }
        const result = await form.validateForm();
        state.serverOnly = serverOnlyErrors(current, result.errors, get(form.form));
    })();

    state.recording = recording;
    try {
        await recording;
    } finally {
        if (state.recording === recording) state.recording = null;
    }
}

/** The library's `validate` answers with the field's messages; this keeps that answer. */
function messagesAt(errors: Record<string, unknown>, path: string | undefined): string[] | undefined {
    if (path === undefined) return undefined;
    return collectErrors(errors).get(pathKey(path));
}

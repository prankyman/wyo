import { z } from 'zod';

/**
 * The forms of «Внешний API». Each folded section sends only its own fields to its own call of the wyo server; the wyo server
 * checks everything again and is the authority. The messages are keys of `messages/`.
 */

const REQUIRED = z.string().trim().min(1, 'errors.required');

/**
 * «Подключение»: the address of the fulfilment centre's API (only `https://`, so the key travels encrypted) and the key. A wyo
 * server built for development and tests takes plain `http://` too, for the fake fulfilment centre; `allowHttp` says which one the
 * page talks to, asked each time the form is checked because the page learns it after it is drawn.
 */
export function connectSchemaFor(allowHttp: () => boolean) {
    return z.object({
        address: z
            .string()
            .trim()
            .superRefine((address, context) => {
                if (address === '') context.addIssue({ code: 'custom', message: 'errors.required' });
                else if (!(allowHttp() ? /^https?:\/\/\S+$/i : /^https:\/\/\S+$/i).test(address))
                    context.addIssue({ code: 'custom', message: 'errors.invalid_address' });
            }),
        key: REQUIRED
    });
}

/** The released wyo server's form: `https://` only. */
export const connectSchema = connectSchemaFor(() => false);

/** The signing secret a fulfilment centre's system shows: `whsec_` and base64 (the Standard Webhooks scheme). */
const SECRET = /^whsec_[A-Za-z0-9+/]+={0,2}$/;

/**
 * «События»: how the wyo server learns what the fulfilment centre did. Notifications need the signing secret of the
 * fulfilment centre's system: required until one is saved (`secretSaved`, asked each time because a save changes it), after which an empty box keeps the saved one.
 */
export function eventsSchema(secretSaved: () => boolean) {
    return z
        .object({
            mode: z.enum(['poll', 'notify']),
            secret: z.string().trim().default('')
        })
        .superRefine((values, context) => {
            if (values.secret !== '' && !SECRET.test(values.secret)) {
                context.addIssue({ code: 'custom', path: ['secret'], message: 'errors.invalid_secret' });
            } else if (values.mode === 'notify' && values.secret === '' && !secretSaved()) {
                context.addIssue({ code: 'custom', path: ['secret'], message: 'errors.required' });
            }
        });
}

/** The call that saves the events' setting: the secret goes only when one was typed. */
export function eventsPayload(values: { mode: 'poll' | 'notify'; secret: string }): {
    mode: 'poll' | 'notify';
    secret?: string;
} {
    return values.secret === '' ? { mode: values.mode } : { mode: values.mode, secret: values.secret };
}

/** «Склады»: one row per warehouse of the seller; an empty `fcWarehouse` is no match. */
export const warehousesSchema = z.object({
    rows: z.array(z.object({ accountId: z.number(), warehouseNumber: z.string(), fcWarehouse: z.string().default('') }))
});

/** The call that saves the warehouse matches: a row with no match is sent without a warehouse, which removes its match. */
export function warehousesPayload(values: z.infer<typeof warehousesSchema>) {
    return {
        rows: values.rows.map(({ accountId, warehouseNumber, fcWarehouse }) =>
            fcWarehouse === '' ? { accountId, warehouseNumber } : { accountId, warehouseNumber, fcWarehouse }
        )
    };
}

/** «Маркетплейсы»: one row per account; an empty `fcMarketplace` is no match. */
export const marketplacesSchema = z.object({
    rows: z.array(z.object({ accountId: z.number(), fcMarketplace: z.string().default('') }))
});

/** The call that saves the marketplace codes: an empty code is sent as it is, which removes the match; the settings stay. */
export function marketplacesPayload(values: z.infer<typeof marketplacesSchema>) {
    return { rows: values.rows.map(({ accountId, fcMarketplace }) => ({ accountId, fcMarketplace })) };
}

/** «Передача заказов маркетплейсу»: the adapter's hand-over settings of each account, by name. */
export const handoverSettingsSchema = z.object({
    rows: z.array(z.object({ accountId: z.number(), handoverSettings: z.record(z.string(), z.string()) }))
});

/** The call that saves the hand-over settings: the marketplace code of each account stays as it is. */
export function handoverSettingsPayload(values: z.infer<typeof handoverSettingsSchema>) {
    return { rows: values.rows.map(({ accountId, handoverSettings }) => ({ accountId, handoverSettings })) };
}

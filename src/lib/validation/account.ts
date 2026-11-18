import { z } from 'zod';
import type { TKeyField } from '$lib/components/marketplaces/types';
import { numberField, readNumberText } from './number-input';

/**
 * The forms of a marketplace account: adding one (its name and the fields of its adapter's key form), replacing its key, and
 * changing its name and its hours to hand an order over. The key's fields are nested under `key`, as the wyo server names them
 * in its calls and refusals (`key.token`), so a refusal lands on its box by the one function every form uses. The wyo server
 * checks everything again and is the authority; the messages are keys of `messages/`.
 */

/**
 * The hours to hand an order over that the form accepts: whole hours from 1 to a week. The wyo server holds the same bounds and
 * refuses another value with `out_of_range`; the field's «?» names them.
 */
export const DEADLINE_HOURS = { decimals: 0, min: 1, max: 168 } as const;

const REQUIRED = z.string().trim().min(1, 'errors.required');

/** The boxes of an adapter's key form, each required. */
function keySchema(fields: readonly TKeyField[]) {
    return z.object(Object.fromEntries(fields.map((field) => [field.name, REQUIRED])));
}

/** The empty boxes of an adapter's key form. */
export function emptyKey(fields: readonly TKeyField[]): Record<string, string> {
    return Object.fromEntries(fields.map((field) => [field.name, '']));
}

/** The one shape of the key form: the account's name, checked by `name`, and the key's boxes. */
function keyFormSchema(fields: readonly TKeyField[], name: z.ZodType<string>) {
    return z.object({ name, key: keySchema(fields) });
}

/** «Добавить аккаунт»: the account's own name and its key. */
export function accountCreateSchema(fields: readonly TKeyField[]) {
    return keyFormSchema(fields, REQUIRED);
}

/** «Заменить ключ»: the key only. The name is not asked (it stays as it is), so the form has the same shape as the one that adds. */
export function keyReplaceSchema(fields: readonly TKeyField[]) {
    return keyFormSchema(fields, z.string().default(''));
}

/** «Изменить»: the name and the hours to hand an order over, the hours as typed. */
export const accountChangeSchema = z.object({
    name: REQUIRED,
    deadlineHours: numberField(DEADLINE_HOURS, {
        default: 'errors.wrong_kind',
        empty: 'errors.required',
        belowMin: 'errors.out_of_range',
        aboveMax: 'errors.out_of_range'
    })
});

/** The call that changes an account (`PATCH /api/v1/accounts/{id}`): the hours as a number. */
export function accountChangePayload(values: z.infer<typeof accountChangeSchema>): {
    name: string;
    deadlineHours: number;
} {
    const reading = readNumberText(values.deadlineHours, DEADLINE_HOURS);
    return { name: values.name, deadlineHours: reading.ok ? Number(reading.value) : Number.NaN };
}

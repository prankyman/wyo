import { z } from 'zod';
import type { TRefusal } from '$lib/api/api';
import type { components } from '$lib/api/wyo-api';
import { numberField, readNumberText } from './number-input';

/**
 * One group of the settings registry as the wyo server lists it (`GET /api/v1/settings`) and the form that draws and saves it.
 * The registry decides which settings exist, their kinds and their bounds, so the interface draws a setting a later plan adds
 * with no change here: a switch for a `boolean`, a number box for an `integer`.
 */
export type TSettingsGroup = components['schemas']['SettingsGroup'];
export type TSetting = components['schemas']['Setting'];

/** The values of a group's form: a switch is `true` or `false`, a number is the text in its box. */
export type TSettingsFormValues = Record<string, boolean | string>;

/** The text of each refusal of a typed number: the same keys the wyo server's own field refusals use. */
const NUMBER_MESSAGES = {
    default: 'errors.wrong_kind',
    empty: 'errors.required',
    belowMin: 'errors.out_of_range',
    aboveMax: 'errors.out_of_range'
} as const;

/**
 * The name of a setting's field in the form: the key without its group (`updates.auto` is `auto`). The form library reads a dot
 * as a step into a nested field, so a key cannot be a field name as it is.
 */
export function fieldNameOf(group: TSettingsGroup, key: string): string {
    return key.startsWith(`${group.id}.`) ? key.slice(group.id.length + 1) : key;
}

/** The key of a field: the group put back. */
export function settingKeyOf(group: TSettingsGroup, fieldName: string): string {
    return `${group.id}.${fieldName}`;
}

/** The inclusive bounds the wyo server sent for a number; null for a setting that has none (a switch). */
export function settingBounds(setting: TSetting): { min?: number; max?: number } | null {
    if (setting.kind !== 'integer') return null;
    const bounds: { min?: number; max?: number } = {};
    if (setting.min !== undefined) bounds.min = setting.min;
    if (setting.max !== undefined) bounds.max = setting.max;
    return bounds;
}

/** What the form holds when it opens: the stored value, a number as its text. */
export function settingsFormValues(group: TSettingsGroup): TSettingsFormValues {
    return Object.fromEntries(
        group.settings.map((setting) => [
            fieldNameOf(group, setting.key),
            setting.kind === 'boolean' ? setting.value === true : String(setting.value ?? '')
        ])
    );
}

/** The check of a group's form in the browser: the wyo server checks the same bounds again and is the authority. */
export function settingsGroupSchema(group: TSettingsGroup) {
    const shape: Record<string, z.ZodType> = {};
    for (const setting of group.settings) {
        shape[fieldNameOf(group, setting.key)] =
            setting.kind === 'boolean'
                ? z.boolean()
                : numberField({ decimals: 0, ...settingBounds(setting) }, NUMBER_MESSAGES);
    }
    return z.object(shape);
}

/** The call that saves a group: every key of the group with its value, a number as a number. */
export function settingsPayload(
    group: TSettingsGroup,
    values: TSettingsFormValues
): { values: Record<string, unknown> } {
    const payload: Record<string, unknown> = {};
    for (const setting of group.settings) {
        const typed = values[fieldNameOf(group, setting.key)];
        if (setting.kind === 'boolean') payload[setting.key] = typed === true;
        else {
            const reading = readNumberText(String(typed), { decimals: 0 });
            payload[setting.key] = reading.ok ? Number(reading.value) : typed;
        }
    }
    return { values: payload };
}

/**
 * A refusal of the wyo server with its issues named by the form's fields. The wyo server may name a setting by its key
 * (`security.idleMinutes`), or under the call's `values`; the form's field is the key without the group.
 */
export function refusalOfGroup(group: TSettingsGroup, refusal: TRefusal): TRefusal {
    if (!refusal.issues) return refusal;
    return {
        ...refusal,
        issues: refusal.issues.map((issue) => ({
            ...issue,
            path: fieldNameOf(group, issue.path.startsWith('values.') ? issue.path.slice('values.'.length) : issue.path)
        }))
    };
}

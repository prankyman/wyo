import { z } from 'zod';
import type { Readable } from 'svelte/store';

/**
 * One form per folded section of an edit screen (constitution, "Forms"): what a screen says about each of its sections, and
 * the helpers the browser and the wyo server's tests share. Each folded section has its own «Сохранить» and «Закрыть» buttons,
 * and fields that are checked together lie in one section, however large.
 *
 * A section sends only its own fields, to its own call of the wyo server. The wyo server puts them over the values stored for
 * the rest of the record and checks the whole record with the same full rules the create screen uses, so a section can never
 * store a combination the whole form would refuse. The section also sends `drawn`: a fingerprint of its fields as the page drew
 * them, which the wyo server compares with what is stored now - a section someone else saved in the meantime is refused
 * instead of being written over.
 */

/** One section of a record's edit screen. */
export interface IFormSection<K extends string = string> {
    /** The section's short name: its action is `save` + the name with a capital (`legal` → `saveLegal`). */
    id: string;
    /** The section's title, an i18n key; a refusal caused by this section names it. */
    titleKey: string;
    /** The record's fields this section shows and posts. Fields that are checked together are in one section. */
    fields: readonly K[];
}

/** The hidden field every section posts: the fingerprint of the values it was drawn with. */
export const DRAWN_FIELD = 'drawn';

/** The superform id of a section on a screen: two sections of one page must never share an id. */
export function sectionFormId(screen: string, sectionId: string): string {
    return `${screen}-${sectionId}`;
}

/** The section that holds a field, or nothing for a field no section shows (a bookkeeping column). */
export function sectionOfField<K extends string>(
    sections: readonly IFormSection<K>[],
    field: string
): IFormSection<K> | undefined {
    return sections.find((section) => (section.fields as readonly string[]).includes(field));
}

/** A section's own values out of a whole record's values. */
export function sectionValues<K extends string>(
    values: Record<string, unknown>,
    fields: readonly K[]
): Record<K, unknown> {
    return Object.fromEntries(fields.map((field) => [field, values[field]])) as Record<K, unknown>;
}

/** A value as the fingerprint counts it: an empty box, `null` and a missing value are the same «nothing». */
function normalized(value: unknown): unknown {
    if (value === undefined || value === null || value === '') return null;
    if (value instanceof Date) return value.toISOString();
    if (Array.isArray(value)) return value.map(normalized);
    if (typeof value === 'object') {
        const entries = Object.entries(value as Record<string, unknown>).sort(([a], [b]) => a.localeCompare(b));
        return Object.fromEntries(entries.map(([key, inner]) => [key, normalized(inner)]));
    }
    return value;
}

/**
 * The fingerprint of a section's values: the same text for the same values in any key order, an empty box and a missing
 * value counted alike. The server computes it from the stored record both when it draws the page and when the section is
 * saved, so the two match exactly while nobody else changed the section.
 */
export function sectionFingerprint(values: Record<string, unknown>, fields: readonly string[]): string {
    return JSON.stringify(fields.map((field) => normalized(values[field])));
}

/**
 * The schema of one section's form: the full schema's rules for the section's fields, the section's own cross-field check
 * when it has one (the legal form with ИНН and КПП), and the `drawn` fingerprint. The browser checks a section with it
 * before sending; the wyo server checks what it receives and then the whole record with the full schema.
 */
export function sectionSchema<Shape extends z.ZodRawShape, K extends keyof Shape & string>(
    shape: Shape,
    fields: readonly K[],
    refine?: (data: z.infer<z.ZodObject<Pick<Shape, K>>>, ctx: z.RefinementCtx) => void
) {
    const picked = Object.fromEntries(fields.map((field) => [field, shape[field]])) as Pick<Shape, K>;
    const object = z.object({ ...picked, [DRAWN_FIELD]: z.string().default('') }) as z.ZodObject<
        Pick<Shape, K> & { drawn: z.ZodDefault<z.ZodString> }
    >;
    return refine ? object.superRefine((data, ctx) => refine(data as never, ctx)) : object;
}

/** What a section reads of its superform (`superForm`, `src/lib/forms/super-form.ts`); `SectionForm.svelte` takes it. */
export interface ISectionSuperForm {
    enhance(formElement: HTMLFormElement): { destroy(): void };
    submitting: Readable<boolean>;
    message: Readable<unknown>;
    tainted: Readable<unknown>;
    isTainted(tainted?: unknown): boolean;
    form: Readable<Record<string, unknown>>;
    reset(options?: { newState?: Record<string, unknown> }): void;
}

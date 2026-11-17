import { describe, expect, it } from 'vitest';
import { z } from 'zod';
import { sectionFingerprint, sectionFormId, sectionOfField, sectionSchema, sectionValues } from './section-form';

/** One form per section of an edit screen: the helpers both sides use. */
const SECTIONS = [
    { id: 'main', titleKey: 'record.section.main', fields: ['code', 'name'] },
    { id: 'details', titleKey: 'record.section.details', fields: ['kind', 'taxCode'] }
] as const;

describe('section-form', () => {
    it('names the form id of a section on a screen', () => {
        expect(sectionFormId('record', 'details')).toBe('record-details');
    });

    it('finds the section of a field and picks a section’s values out of the record', () => {
        expect(sectionOfField(SECTIONS, 'taxCode')?.id).toBe('details');
        expect(sectionOfField(SECTIONS, 'isActive')).toBeUndefined();
        expect(sectionValues({ code: 'SAMPLE', name: 'Sample', taxCode: '1' }, ['code', 'name'])).toEqual({
            code: 'SAMPLE',
            name: 'Sample'
        });
    });

    it('fingerprints the same values alike whatever their key order, and an empty box as nothing', () => {
        const fields = ['phone', 'contractDate', 'barcodes'];
        const drawn = sectionFingerprint(
            { phone: '', contractDate: new Date('2026-01-15T00:00:00.000Z'), barcodes: [{ barcode: '46', id: 1 }] },
            fields
        );
        expect(
            sectionFingerprint(
                { barcodes: [{ id: 1, barcode: '46' }], contractDate: '2026-01-15T00:00:00.000Z', phone: undefined },
                fields
            )
        ).toBe(drawn);
        expect(sectionFingerprint({ phone: null }, ['phone'])).toBe(sectionFingerprint({}, ['phone']));
        expect(sectionFingerprint({ phone: '+7 900' }, ['phone'])).not.toBe(sectionFingerprint({}, ['phone']));
    });

    it('builds a section’s schema from the full shape, with the drawn fingerprint and the section’s own rule', () => {
        const shape = { code: z.string().min(2), taxCode: z.string(), kind: z.enum(['company', 'entrepreneur']) };
        const main = sectionSchema(shape, ['code']);
        expect(main.parse({ code: 'SAMPLE', taxCode: 'dropped' })).toEqual({ code: 'SAMPLE', drawn: '' });
        expect(main.safeParse({ code: 'D' }).success).toBe(false);

        const details = sectionSchema(shape, ['kind', 'taxCode'], (data, ctx) => {
            if (data.kind === 'company' && data.taxCode.length !== 10)
                ctx.addIssue({ code: 'custom', path: ['taxCode'], message: 'taxCodeLength' });
        });
        const refused = details.safeParse({ kind: 'company', taxCode: '123', drawn: 'x' });
        expect(refused.success ? [] : refused.error.issues.map((issue) => issue.message)).toEqual(['taxCodeLength']);
    });
});

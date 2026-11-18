import { describe, expect, it } from 'vitest';
import { shippingPointSchema } from './handover';

describe('shippingPointSchema', () => {
    it('asks for a point and takes the point as it is, without surrounding spaces', () => {
        expect(shippingPointSchema.safeParse({ point: ' p1 ' })).toMatchObject({
            success: true,
            data: { point: 'p1' }
        });
        expect(shippingPointSchema.safeParse({ point: '' }).error?.issues.map((issue) => issue.message)).toEqual([
            'errors.required'
        ]);
    });
});

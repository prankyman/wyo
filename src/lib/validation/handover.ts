import { z } from 'zod';

/** «Запомнить и продолжить» on a hand-over: the shipping point the owner picked from the marketplace's list. */
export const shippingPointSchema = z.object({
    point: z.string().trim().min(1, 'errors.required')
});

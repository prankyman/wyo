import { z } from 'zod';

/**
 * The sign-in form: a login and a password, both asked for. Nothing more is checked in the browser — a wrong login and a wrong
 * password get the same answer from the wyo server, so the browser never tells the two apart.
 */
export const signInSchema = z.object({
    login: z.string().trim().min(1, 'errors.required'),
    password: z.string().min(1, 'errors.required')
});

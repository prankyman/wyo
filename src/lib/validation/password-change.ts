import { z } from 'zod';

/** The fewest characters of a new password; the wyo server holds the same limit and is the authority. */
export const PASSWORD_MIN_LENGTH = 10;

/**
 * The form of «Мой профиль» that changes the password: the current one, the new one, and the new one again so a slip of the
 * finger is caught before it locks the owner out. The messages are keys of `messages/`. The wyo server checks the same things
 * again (`wrong`, `too_short`, `same_as_current`) and its refusal lands on the same fields.
 */
export const passwordChangeSchema = z
    .object({
        currentPassword: z.string().min(1, 'errors.required'),
        newPassword: z.string().min(PASSWORD_MIN_LENGTH, 'errors.too_short'),
        confirmNewPassword: z.string().min(1, 'errors.required')
    })
    .superRefine((values, context) => {
        if (values.newPassword.length >= PASSWORD_MIN_LENGTH && values.newPassword === values.currentPassword) {
            context.addIssue({ code: 'custom', path: ['newPassword'], message: 'errors.same_as_current' });
        }
        if (values.confirmNewPassword !== '' && values.confirmNewPassword !== values.newPassword) {
            context.addIssue({ code: 'custom', path: ['confirmNewPassword'], message: 'errors.passwords_differ' });
        }
    });

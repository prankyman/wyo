import { describe, expect, it } from 'vitest';
import { passwordChangeSchema, PASSWORD_MIN_LENGTH } from './password-change';

const TYPED = {
    currentPassword: 'old-password-1',
    newPassword: 'new-password-2',
    confirmNewPassword: 'new-password-2'
};

/** Which field each refusal lands on and the text key it carries. */
function refusals(values: Record<string, string>): [string, string][] {
    const result = passwordChangeSchema.safeParse(values);
    return result.success ? [] : result.error.issues.map((issue) => [String(issue.path[0]), issue.message]);
}

describe('passwordChangeSchema', () => {
    it('accepts the current password and a new one typed twice, at least ten characters long', () => {
        expect(PASSWORD_MIN_LENGTH).toBe(10);
        expect(passwordChangeSchema.safeParse(TYPED).success).toBe(true);
    });

    it.each([
        ['an empty current password', { currentPassword: '' }, [['currentPassword', 'errors.required']]],
        [
            'a new password of nine characters',
            { newPassword: '123456789', confirmNewPassword: '123456789' },
            [['newPassword', 'errors.too_short']]
        ],
        [
            'a new password equal to the current one',
            { newPassword: 'old-password-1', confirmNewPassword: 'old-password-1' },
            [['newPassword', 'errors.same_as_current']]
        ],
        [
            'a second typing that differs',
            { confirmNewPassword: 'new-password-3' },
            [['confirmNewPassword', 'errors.passwords_differ']]
        ],
        ['an empty second typing', { confirmNewPassword: '' }, [['confirmNewPassword', 'errors.required']]]
    ])('refuses %s on its own field', (_case, change, expected) => {
        expect(refusals({ ...TYPED, ...change })).toEqual(expected);
    });
});

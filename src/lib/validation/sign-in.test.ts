import { describe, expect, it } from 'vitest';
import { signInSchema } from './sign-in';

describe('signInSchema', () => {
    it('wants a login and a password and says nothing more: the wyo server alone decides whether they fit', () => {
        expect(signInSchema.safeParse({ login: 'owner', password: 'x' }).success).toBe(true);
        const refused = signInSchema.safeParse({ login: ' ', password: '' });
        expect(
            refused.success ? [] : refused.error.issues.map((issue) => [String(issue.path[0]), issue.message])
        ).toEqual([
            ['login', 'errors.required'],
            ['password', 'errors.required']
        ]);
    });
});

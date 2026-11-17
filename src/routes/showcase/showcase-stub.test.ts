import { describe, expect, it } from 'vitest';
import { REFUSED_NAME, stubSend } from './showcase-stub';

describe('stubSend (the showcase form has no server)', () => {
    it('answers a success for ordinary values and a refusal that names the name box for the refused word', async () => {
        await expect(stubSend({ name: 'Вася' })).resolves.toEqual({ ok: true, data: {} });

        const refused = await stubSend({ name: `  ${REFUSED_NAME.toUpperCase()} ` });
        expect(refused).toEqual({
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'name', code: 'invalid' }] }
        });
    });
});

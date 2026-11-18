import { afterEach, describe, expect, it, vi } from 'vitest';
import { handleError } from './hooks.client';

/**
 * An error while a page loads or draws. An address with no page (404) is an expected event, worded by the error page; every other
 * error is a fault and goes to the console, where a person or a browser test finds it.
 */
const event = {} as Parameters<typeof handleError>[0]['event'];

afterEach(() => {
    vi.restoreAllMocks();
});

describe('handleError', () => {
    it.each([
        { status: 404, logged: false },
        { status: 500, logged: true }
    ])('logs an error of status $status: $logged, and keeps the message', ({ status, logged }) => {
        const log = vi.spyOn(console, 'error').mockImplementation(() => {});
        const error = new Error('cause');

        expect(handleError({ error, event, status, message: 'Not Found' })).toEqual({ message: 'Not Found' });
        expect(log.mock.calls).toEqual(logged ? [[error]] : []);
    });
});

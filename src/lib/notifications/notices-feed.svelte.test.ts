import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn() }
}));

import { api } from '$lib/api/api';
import { NoticesFeed, NOTICES_EVERY_MS } from './notices-feed.svelte';

const get = vi.mocked(api.get);
const NOTICE = { id: 'backups:backup_none', source: 'backups', code: 'backup_none', tone: 'warn' } as const;

beforeEach(() => vi.useFakeTimers());
afterEach(() => {
    vi.useRealTimers();
    get.mockReset();
});

describe('NoticesFeed', () => {
    it('reads the notices on arrival and then every minute, always as a background call, and words them as lines', async () => {
        get.mockResolvedValue({ ok: true, data: { notices: [NOTICE] } });
        const feed = new NoticesFeed();

        feed.start('ru');
        await vi.advanceTimersByTimeAsync(0);
        expect(get).toHaveBeenCalledExactlyOnceWith('/api/v1/notices', { background: true });
        expect(feed.lines.map((line) => [line.key, line.urgency])).toEqual([['backups:backup_none', 'attention']]);

        get.mockResolvedValue({ ok: true, data: { notices: [] } });
        await vi.advanceTimersByTimeAsync(NOTICES_EVERY_MS);
        expect(get).toHaveBeenCalledTimes(2);
        expect(feed.lines).toEqual([]);
        feed.stop();
    });

    it('keeps the lines it has when a read fails, and stops asking when it is stopped', async () => {
        get.mockResolvedValueOnce({ ok: true, data: { notices: [NOTICE] } });
        const feed = new NoticesFeed();
        feed.start('ru');
        await vi.advanceTimersByTimeAsync(0);

        get.mockResolvedValue({ ok: false, status: 0, refusal: { code: 'network' } });
        await vi.advanceTimersByTimeAsync(NOTICES_EVERY_MS);
        expect(feed.lines).toHaveLength(1);

        feed.stop();
        await vi.advanceTimersByTimeAsync(NOTICES_EVERY_MS * 3);
        expect(get).toHaveBeenCalledTimes(2);
    });
});

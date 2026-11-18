import { api } from '$lib/api/api';
import type { TLocale } from '$lib/i18n/types';
import { noticeLines, type TNotice } from './notice-lines';
import type { INotificationLine } from './types';

/** How often the bell asks the wyo server for its notices. */
export const NOTICES_EVERY_MS = 60_000;

/**
 * The lines behind the bell: read from `GET /api/v1/notices` on arrival and then every minute. Every read is a BACKGROUND call
 * (`{ background: true }`): the person did not cause it, so it does not prolong the session, and a screen left open still signs
 * out after the idle minutes. A read that fails keeps the lines the bell already has.
 */
export class NoticesFeed {
    lines = $state<INotificationLine[]>([]);
    #timer: ReturnType<typeof setInterval> | undefined;

    start(lang: TLocale): void {
        this.stop();
        void this.#read(lang);
        this.#timer = setInterval(() => void this.#read(lang), NOTICES_EVERY_MS);
    }

    stop(): void {
        clearInterval(this.#timer);
        this.#timer = undefined;
    }

    async #read(lang: TLocale): Promise<void> {
        const result = await api.get<{ notices: TNotice[] }>('/api/v1/notices', { background: true });
        if (result.ok) this.lines = noticeLines(result.data.notices, lang);
    }
}

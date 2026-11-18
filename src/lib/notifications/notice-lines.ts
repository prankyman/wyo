import { messageParams } from '$lib/api/api';
import type { components } from '$lib/api/wyo-api';
import { hasMessage, localizePath, t } from '$lib/i18n/i18n';
import { tPlural } from '$lib/i18n/plural';
import { certificateReasonText } from '$lib/overview/certificate-reason';
import type { TLocale } from '$lib/i18n/types';
import type { INotificationLine, TUrgency } from './types';

/** One notice as the wyo server lists it (`GET /api/v1/notices`): a code with parameters, never a sentence. */
export type TNotice = components['schemas']['Notice'];

/** How much a notice asks, by its tone: a stop is urgent, a warning needs attention, everything else is for information. */
const URGENCY_OF_TONE: Record<TNotice['tone'], TUrgency> = {
    stop: 'urgent',
    warn: 'attention',
    wait: 'info',
    info: 'info',
    ok: 'info'
};

/** The page where each source's notices are dealt with: the certificate's card is on «Обзор», the rest are in «Настройки». */
const PAGE_OF_SOURCE: Record<TNotice['source'], string> = {
    updates: '/settings',
    backups: '/settings',
    certificate: '/'
};

/** The codes whose text is counted («3 дня назад»): the parameter that holds the count. */
const COUNTED: Record<string, string> = { backup_old: 'days' };

/** A notice's sentence, worded from its code and parameters in the language of the page. */
function textOf(notice: TNotice, lang: TLocale): string {
    const params = { ...messageParams(notice.params) };
    if (notice.code === 'certificate_failed') {
        params.reason = certificateReasonText(String(params.reason), String(params.domain ?? ''), lang);
    }
    const counted = COUNTED[notice.code];
    if (counted !== undefined && typeof params[counted] === 'number') {
        return tPlural(`notices.${notice.code}`, params[counted], lang, params);
    }
    const key = `notices.${notice.code}`;
    return t(hasMessage(key, lang) ? key : 'notices.unknown', lang, params);
}

/**
 * The lines of the bell's panel for the notices the wyo server listed, in the order it gave them. The wyo server writes no
 * sentence: each line is worded here from the notice's code (`notices.<code>`) and its parameters.
 */
export function noticeLines(notices: readonly TNotice[], lang: TLocale): INotificationLine[] {
    return notices.map((notice) => ({
        key: notice.id,
        kind: notice.code,
        urgency: URGENCY_OF_TONE[notice.tone],
        text: textOf(notice, lang),
        href: localizePath(PAGE_OF_SOURCE[notice.source], lang)
    }));
}

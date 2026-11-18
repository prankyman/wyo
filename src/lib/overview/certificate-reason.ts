import { t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';

/**
 * The reasons the wyo server names for a certificate it could not get (`certificate.lastError` of the installation, the `reason`
 * of the notice `certificate_failed`), and the end of the key of each one's text (`certificate.reason.<end>`). Any other word,
 * and no word, is read as `other`.
 */
const REASONS: Record<string, string> = {
    dns: 'dns',
    ports: 'ports',
    authority: 'authority',
    other: 'unspecified'
};

/** What to check, in the language of the page, for the reason `reason`: the one place this is worded. */
export function certificateReasonText(reason: string | undefined, domain: string, lang: TLocale): string {
    return t(`certificate.reason.${REASONS[reason ?? ''] ?? REASONS.other}`, lang, { domain });
}

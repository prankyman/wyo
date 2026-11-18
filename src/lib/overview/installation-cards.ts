import type { components } from '$lib/api/wyo-api';
import type { TTone } from '$lib/components/ui/status-tones';
import { formatDate } from '$lib/i18n/format-date';
import { formatBytes } from '$lib/i18n/format-bytes';
import { localizePath, t } from '$lib/i18n/i18n';
import { certificateReasonText } from './certificate-reason';
import type { TLocale } from '$lib/i18n/types';

/** The state of the installation as the wyo server answers it (`GET /api/v1/installation`). */
export type TInstallation = components['schemas']['Installation'];

/** One card of «Обзор», every text already worded in the language of the page; `InstallationStateCard` draws it. */
export interface IInstallationCard {
    /** The card's name: `version`, `certificate`, `backups`, and the marketplace cards' `accounts`, `orders`, `reading`. */
    key: string;
    title: string;
    /** What the card is, why it is there and what the person does about it: the text behind its «?». */
    help: string;
    state: { label: string; tone: TTone };
    sentence: string;
    action: { label: string; href: string };
    /** Further links of the card, drawn after its action (the card of «Внешний API» links to the orders that need the owner). */
    moreActions?: { label: string; href: string }[];
}

const SETTINGS = '/settings';

function versionCard(installation: TInstallation, lang: TLocale): Pick<IInstallationCard, 'state' | 'sentence'> {
    const { version, updates } = installation;
    const params = { version };
    if (updates.requested) {
        return {
            state: { label: t('overview.version.requested', lang), tone: 'wait' },
            sentence: t('overview.version.requestedSentence', lang)
        };
    }
    if (updates.failedVersion) {
        return {
            state: { label: t('overview.version.failed', lang), tone: 'stop' },
            sentence: t('overview.version.failedSentence', lang, { ...params, failed: updates.failedVersion })
        };
    }
    if (updates.waitingVersion) {
        return {
            state: { label: t('overview.version.waiting', lang), tone: 'info' },
            sentence: t(updates.auto ? 'overview.version.waitingAuto' : 'overview.version.waitingManual', lang, {
                ...params,
                waiting: updates.waitingVersion
            })
        };
    }
    return {
        state: { label: t('overview.version.ok', lang), tone: 'ok' },
        sentence: updates.lastCheckAt
            ? t('overview.version.okChecked', lang, { ...params, date: formatDate(updates.lastCheckAt, lang) })
            : t('overview.version.okNever', lang, params)
    };
}

function certificateCard(installation: TInstallation, lang: TLocale): Pick<IInstallationCard, 'state' | 'sentence'> {
    const { mode, state, domain = '', notAfter, lastError } = installation.certificate;
    if (state === 'not_applicable') {
        return mode === 'proxy'
            ? {
                  state: { label: t('overview.certificate.proxy', lang), tone: 'off' },
                  sentence: t('overview.certificate.proxySentence', lang)
              }
            : {
                  state: { label: t('overview.certificate.none', lang), tone: 'off' },
                  sentence: t('overview.certificate.noneSentence', lang)
              };
    }
    if (state === 'pending') {
        return {
            state: { label: t('overview.certificate.pending', lang), tone: 'wait' },
            sentence: t('overview.certificate.pendingSentence', lang, { domain })
        };
    }
    if (state === 'failed') {
        return {
            state: { label: t('overview.certificate.failed', lang), tone: 'stop' },
            sentence: t('overview.certificate.failedSentence', lang, {
                domain,
                reason: certificateReasonText(lastError, domain, lang)
            })
        };
    }
    return {
        state: { label: t('overview.certificate.ok', lang), tone: 'ok' },
        sentence: t('overview.certificate.okSentence', lang, {
            domain,
            date: notAfter ? formatDate(notAfter, lang) : ''
        })
    };
}

function backupsCard(installation: TInstallation, lang: TLocale): Pick<IInstallationCard, 'state' | 'sentence'> {
    const { state, last } = installation.backups;
    const params: Record<string, string> = last
        ? { date: formatDate(last.createdAt, lang), size: formatBytes(last.size, lang) }
        : {};
    switch (state) {
        case 'ok':
            return {
                state: { label: t('overview.backups.ok', lang), tone: 'ok' },
                sentence: t('overview.backups.okSentence', lang, params)
            };
        case 'old':
            return {
                state: { label: t('overview.backups.old', lang), tone: 'warn' },
                sentence: t('overview.backups.oldSentence', lang, params)
            };
        case 'failed':
            return {
                state: { label: t('overview.backups.failed', lang), tone: 'stop' },
                sentence: t('overview.backups.failedSentence', lang)
            };
        case 'none':
            return {
                state: { label: t('overview.backups.none', lang), tone: 'warn' },
                sentence: t('overview.backups.noneSentence', lang)
            };
    }
}

/**
 * The three cards of «Обзор» for an installation: the version and whether a newer one waits, the certificate and the day it ends,
 * the last copy of the database. Each answers what the card says, in sentences by the help-text rules, and leads to «Настройки»,
 * where the person acts. A later plan adds its cards beside these.
 */
export function installationCards(installation: TInstallation, lang: TLocale): IInstallationCard[] {
    const action = { label: t('overview.card.openSettings', lang), href: localizePath(SETTINGS, lang) };
    return [
        {
            key: 'version',
            title: t('overview.version.title', lang),
            help: t('overview.version.help', lang),
            action,
            ...versionCard(installation, lang)
        },
        {
            key: 'certificate',
            title: t('overview.certificate.title', lang),
            help: t('overview.certificate.help', lang),
            action,
            ...certificateCard(installation, lang)
        },
        {
            key: 'backups',
            title: t('overview.backups.title', lang),
            help: t('overview.backups.help', lang),
            action,
            ...backupsCard(installation, lang)
        }
    ];
}

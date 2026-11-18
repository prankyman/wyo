import type { components } from '$lib/api/wyo-api';
import { formatDateTime } from '$lib/i18n/format-date';
import { localizePath, t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';
import type { IInstallationCard } from './installation-cards';

/** The counts of «Обзор» about the marketplace accounts and their orders (`GET /api/v1/overview/marketplaces`). */
export type TMarketplacesOverview = components['schemas']['MarketplacesOverview'];

function accountsCard(overview: TMarketplacesOverview, lang: TLocale): Pick<IInstallationCard, 'state' | 'sentence'> {
    const { accounts, attention } = overview;
    if (accounts === 0)
        return {
            state: { label: t('overview.accounts.none', lang), tone: 'warn' },
            sentence: t('overview.accounts.noneSentence', lang)
        };
    if (attention > 0)
        return {
            state: { label: t('overview.accounts.attention', lang), tone: 'stop' },
            sentence: t('overview.accounts.attentionSentence', lang, { attention, accounts })
        };
    return {
        state: { label: t('overview.accounts.ok', lang), tone: 'ok' },
        sentence: t('overview.accounts.okSentence', lang, { accounts })
    };
}

function ordersCard(overview: TMarketplacesOverview, lang: TLocale): Pick<IInstallationCard, 'state' | 'sentence'> {
    const { newOrders, assembling, overdue, unknownStates } = overview;
    const counts = { newOrders, assembling, overdue };
    const state: IInstallationCard['state'] =
        overdue > 0
            ? { label: t('overview.orders.overdue', lang), tone: 'stop' }
            : { label: t('overview.orders.ok', lang), tone: 'ok' };
    const sentence = t(overdue > 0 ? 'overview.orders.overdueSentence' : 'overview.orders.okSentence', lang, counts);
    const unknown = unknownStates > 0 ? ` ${t('overview.orders.unknownSentence', lang, { unknownStates })}` : '';
    return { state, sentence: `${sentence}${unknown}` };
}

function readingCard(overview: TMarketplacesOverview, lang: TLocale): Pick<IInstallationCard, 'state' | 'sentence'> {
    if (overview.lastSuccessAt === undefined)
        return {
            state: { label: t('overview.reading.none', lang), tone: 'wait' },
            sentence: t('overview.reading.noneSentence', lang)
        };
    return {
        state: { label: t('overview.reading.ok', lang), tone: 'ok' },
        sentence: t('overview.reading.okSentence', lang, { date: formatDateTime(overview.lastSuccessAt, lang) })
    };
}

/**
 * The three cards «Обзор» gains with the marketplaces: the accounts and whether every one works, the orders waiting and those past
 * their deadline, and the last successful reading. Each leads to the page where the person acts; `InstallationStateCard` draws
 * them, as it draws the installation's cards.
 */
export function marketplaceCards(overview: TMarketplacesOverview, lang: TLocale): IInstallationCard[] {
    const card = (key: string, page: string, part: Pick<IInstallationCard, 'state' | 'sentence'>) => ({
        key,
        title: t(`overview.${key}.title`, lang),
        help: t(`overview.${key}.help`, lang),
        action: { label: t(`overview.${key}.action`, lang), href: localizePath(page, lang) },
        ...part
    });
    return [
        card('accounts', '/marketplaces', accountsCard(overview, lang)),
        card('orders', '/orders', ordersCard(overview, lang)),
        card('reading', '/journal', readingCard(overview, lang))
    ];
}

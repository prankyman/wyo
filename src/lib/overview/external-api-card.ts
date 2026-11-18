import type { components } from '$lib/api/wyo-api';
import { EXTERNAL_API_TONE } from '$lib/components/external-api/state';
import { formatDateTime } from '$lib/i18n/format-date';
import { localizePath, t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';
import type { IInstallationCard } from './installation-cards';

/** The card «Внешний API» of «Обзор» (`GET /api/v1/overview/external-api`). */
export type TExternalApiOverview = components['schemas']['ExternalApiOverview'];

/**
 * The card «Внешний API» of «Обзор»: the state of the setting, when the fulfilment centre last reported an event, how many
 * orders need the owner (with a link to the order list filtered for them) and how many hand-overs wait or have a problem (with a
 * link to «Передачи»). A card with nothing to say about the fulfilment centre says that nothing is sent anywhere. `InstallationStateCard`
 * draws it, as it draws the other cards.
 */
export function externalApiCard(overview: TExternalApiOverview, lang: TLocale): IInstallationCard {
    const { state, lastEventAt, attention, handoversWaiting } = overview;
    const sentence =
        state === 'empty'
            ? t('overview.externalApi.emptySentence', lang)
            : [
                  lastEventAt
                      ? t('overview.externalApi.lastEvent', lang, { date: formatDateTime(lastEventAt, lang) })
                      : t('overview.externalApi.noEvent', lang),
                  attention > 0 ? t('overview.externalApi.attention', lang, { attention }) : '',
                  handoversWaiting > 0 ? t('overview.externalApi.handovers', lang, { handovers: handoversWaiting }) : ''
              ]
                  .filter((part) => part !== '')
                  .join(' ');
    const moreActions: IInstallationCard['moreActions'] = [];
    if (attention > 0) {
        moreActions.push({
            label: t('overview.externalApi.attentionAction', lang, { attention }),
            href: localizePath('/orders?attention=true', lang)
        });
    }
    if (handoversWaiting > 0) {
        moreActions.push({
            label: t('overview.externalApi.handoversAction', lang),
            href: localizePath('/handovers', lang)
        });
    }
    return {
        key: 'externalApi',
        title: t('overview.externalApi.title', lang),
        help: t('overview.externalApi.help', lang),
        state: { label: t(`externalApi.state.${state}`, lang), tone: EXTERNAL_API_TONE[state] },
        sentence,
        action: { label: t('overview.externalApi.action', lang), href: localizePath('/external-api', lang) },
        moreActions
    };
}

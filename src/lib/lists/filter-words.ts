import type { IFilterChip } from '$lib/components/ui/types';
import { t } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';

/** The filters in use as words for a sentence («По фильтру ничего не найдено: …»): each chip's words in quotes, listed. */
export function filtersInWords(chips: readonly Pick<IFilterChip, 'label'>[], lang: TLocale): string {
    return chips.map((chip) => t('app.list.chip', lang, { label: chip.label })).join(', ');
}

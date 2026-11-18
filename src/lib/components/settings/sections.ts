import type { TSettingsGroup } from '$lib/validation/settings-group';

/** One folded section of «Настройки»: a group of the registry, or the copies of the database, which hold no setting. */
export type TSettingsSection = { kind: 'group'; group: TSettingsGroup } | { kind: 'backups' };

/** The order the sections are drawn in: the groups this program knows by name, with the copies after the updates. */
const KNOWN_ORDER = ['updates', 'backups', 'security'];

/**
 * The sections of «Настройки» in the order they are drawn: «Обновления», «Резервные копии», «Вход и безопасность», then every
 * other group of the registry in the order the wyo server listed it, so a group a later plan adds appears with no change here.
 */
export function settingsSections(groups: readonly TSettingsGroup[]): TSettingsSection[] {
    const sections: TSettingsSection[] = [
        ...groups.map((group): TSettingsSection => ({ kind: 'group', group })),
        { kind: 'backups' }
    ];
    const rank = (section: TSettingsSection): number => {
        const known = KNOWN_ORDER.indexOf(section.kind === 'group' ? section.group.id : 'backups');
        return known === -1 ? KNOWN_ORDER.length : known;
    };
    // `sort` is stable: groups of equal rank keep the registry's order.
    return sections.sort((a, b) => rank(a) - rank(b));
}

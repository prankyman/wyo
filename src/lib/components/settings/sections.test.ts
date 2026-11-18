import { describe, expect, it } from 'vitest';
import type { TSettingsGroup } from '$lib/validation/settings-group';
import { settingsSections } from './sections';

const group = (id: string): TSettingsGroup => ({ id, settings: [] });

describe('settingsSections', () => {
    it('puts the copies between the updates and the security, which the registry knows nothing about', () => {
        const sections = settingsSections([group('security'), group('updates')]);

        expect(sections.map((section) => (section.kind === 'group' ? section.group.id : section.kind))).toEqual([
            'updates',
            'backups',
            'security'
        ]);
    });

    it('draws a group a later plan adds after the known ones, in the order the registry gave, and the copies even with no group', () => {
        const sections = settingsSections([group('marketplaces'), group('updates'), group('later'), group('security')]);

        expect(sections.map((section) => (section.kind === 'group' ? section.group.id : section.kind))).toEqual([
            'updates',
            'backups',
            'security',
            'marketplaces',
            'later'
        ]);
        expect(settingsSections([])).toEqual([{ kind: 'backups' }]);
    });
});

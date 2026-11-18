import { describe, expect, it } from 'vitest';
import type { TSettingsGroup } from './settings-group';
import {
    fieldNameOf,
    refusalOfGroup,
    settingKeyOf,
    settingsFormValues,
    settingsGroupSchema,
    settingsPayload,
    settingBounds
} from './settings-group';

/** The registry's two groups as the wyo server answers them (`GET /api/v1/settings`). */
const UPDATES: TSettingsGroup = {
    id: 'updates',
    settings: [{ key: 'updates.auto', kind: 'boolean', value: false, defaultValue: true }]
};
const SECURITY: TSettingsGroup = {
    id: 'security',
    settings: [{ key: 'security.idleMinutes', kind: 'integer', value: 60, defaultValue: 60, min: 5, max: 720 }]
};

describe('field names', () => {
    it('drops the group from the key, because the form library reads a dot as a nested field, and puts it back', () => {
        expect(fieldNameOf(UPDATES, 'updates.auto')).toBe('auto');
        expect(fieldNameOf(SECURITY, 'security.idleMinutes')).toBe('idleMinutes');
        expect(settingKeyOf(SECURITY, 'idleMinutes')).toBe('security.idleMinutes');
    });
});

describe('refusalOfGroup', () => {
    it.each([
        ['the key of a setting', 'security.idleMinutes'],
        ['the key under `values`', 'values.security.idleMinutes'],
        ['the field name already', 'idleMinutes']
    ])('lays a refusal that names %s onto the setting’s field', (_case, path) => {
        const refusal = { code: 'invalid', issues: [{ path, code: 'out_of_range' }] };

        expect(refusalOfGroup(SECURITY, refusal).issues).toEqual([{ path: 'idleMinutes', code: 'out_of_range' }]);
    });

    it('leaves a refusal with no issues as it is', () => {
        expect(refusalOfGroup(SECURITY, { code: 'internal' })).toEqual({ code: 'internal' });
    });
});

describe('settingsFormValues', () => {
    it('starts a form with a switch as it is and a number as the text a person would type', () => {
        expect(settingsFormValues(UPDATES)).toEqual({ auto: false });
        expect(settingsFormValues(SECURITY)).toEqual({ idleMinutes: '60' });
    });
});

describe('settingsGroupSchema', () => {
    it('accepts a switch and a number inside the bounds the wyo server sent', () => {
        expect(settingsGroupSchema(UPDATES).safeParse({ auto: true }).success).toBe(true);
        expect(settingsGroupSchema(SECURITY).safeParse({ idleMinutes: '5' }).success).toBe(true);
        expect(settingsGroupSchema(SECURITY).safeParse({ idleMinutes: '720' }).success).toBe(true);
    });

    it.each([
        ['below the lowest value', '4', 'errors.out_of_range'],
        ['above the highest value', '721', 'errors.out_of_range'],
        ['a fraction', '5,5', 'errors.wrong_kind'],
        ['not a number', 'abc', 'errors.wrong_kind'],
        ['nothing', '', 'errors.required']
    ])('refuses %s with the text of its reason', (_case, typed, key) => {
        const result = settingsGroupSchema(SECURITY).safeParse({ idleMinutes: typed });

        expect(result.success).toBe(false);
        expect(result.error?.issues.map((issue) => issue.message)).toEqual([key]);
    });
});

describe('settingsPayload', () => {
    it('turns the form back into the call: the group’s keys, a switch as true or false, a number as a number', () => {
        expect(settingsPayload(UPDATES, { auto: true })).toEqual({ values: { 'updates.auto': true } });
        expect(settingsPayload(SECURITY, { idleMinutes: ' 90 ' })).toEqual({ values: { 'security.idleMinutes': 90 } });
    });
});

describe('settingBounds', () => {
    it('says the bounds of a number and none for a switch', () => {
        expect(settingBounds(SECURITY.settings[0])).toEqual({ min: 5, max: 720 });
        expect(settingBounds(UPDATES.settings[0])).toBeNull();
    });
});

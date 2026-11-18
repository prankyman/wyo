import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushTicks, sendForm } from '$test/support/superform-answer';
import type { TSettingsGroup } from '$lib/validation/settings-group';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { put: vi.fn() }
}));

import { api } from '$lib/api/api';
import SettingsSection from './SettingsSection.svelte';

const put = vi.mocked(api.put);

const UPDATES: TSettingsGroup = {
    id: 'updates',
    settings: [{ key: 'updates.auto', kind: 'boolean', value: true, defaultValue: true }]
};
const SECURITY: TSettingsGroup = {
    id: 'security',
    settings: [{ key: 'security.idleMinutes', kind: 'integer', value: 60, defaultValue: 60, min: 5, max: 720 }]
};
const extra = createRawSnippet(() => ({ render: () => '<p data-test="extra-part">Версия</p>' }));

async function openSection(id: string): Promise<HTMLDetailsElement> {
    const section = screen.getByTestId(id) as HTMLDetailsElement;
    section.open = true;
    await fireEvent(section, new Event('toggle'));
    await flushTicks();
    return section;
}

afterEach(() => {
    put.mockReset();
});

describe('SettingsSection', () => {
    it('draws one registry group as a folded section with its title, its help and a switch for a boolean setting', async () => {
        render(SettingsSection, { props: { lang: 'ru', group: UPDATES, extra } });

        const section = screen.getByTestId('settings-updates') as HTMLDetailsElement;
        expect(section.open).toBe(false);
        expect(screen.getByTestId('settings-updates-heading')).toHaveTextContent('Обновления');
        await openSection('settings-updates');
        const toggle = screen.getByTestId('setting-auto');
        expect(toggle).toHaveAttribute('role', 'switch');
        expect(toggle).toBeChecked();
        expect(screen.getByTestId('setting-auto-field')).toHaveTextContent('Обновлять автоматически');
        expect(screen.getByTestId('extra-part')).toBeInTheDocument();
        expect(screen.queryByTestId('settings-updates-save')).toBeNull();
    });

    it('saves a changed switch through the group’s call, tells the page the answer, says «Сохранено.» and folds', async () => {
        const saved: TSettingsGroup = { ...UPDATES, settings: [{ ...UPDATES.settings[0], value: false }] };
        put.mockResolvedValue({ ok: true, data: saved });
        const onSaved = vi.fn();
        render(SettingsSection, { props: { lang: 'ru', group: UPDATES, onSaved } });
        const section = await openSection('settings-updates');

        await fireEvent.click(screen.getByTestId('setting-auto'));
        await flushTicks();
        await sendForm(screen.getByTestId('settings-updates-form'));

        await waitFor(() => expect(onSaved).toHaveBeenCalledExactlyOnceWith(saved));
        expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/settings/updates', { values: { 'updates.auto': false } });
        expect(section.open).toBe(false);
        expect(screen.getByTestId('settings-updates-saved')).toHaveTextContent('Сохранено.');
    });

    it('checks a number in the browser against the registry’s bounds and sends it as a number once it fits', async () => {
        put.mockResolvedValue({ ok: true, data: SECURITY });
        render(SettingsSection, { props: { lang: 'ru', group: SECURITY } });
        await openSection('settings-security');
        const box = screen.getByTestId('setting-idleMinutes') as HTMLInputElement;

        await fireEvent.input(box, { target: { value: '4' } });
        await sendForm(screen.getByTestId('settings-security-form'));
        await waitFor(() =>
            expect(screen.getByTestId('setting-idleMinutes-field')).toHaveTextContent('Число вне допустимых границ')
        );
        expect(put).not.toHaveBeenCalled();

        await fireEvent.input(box, { target: { value: '90' } });
        await sendForm(screen.getByTestId('settings-security-form'));
        await waitFor(() =>
            expect(put).toHaveBeenCalledExactlyOnceWith('/api/v1/settings/security', {
                values: { 'security.idleMinutes': 90 }
            })
        );
    });

    it('lays the wyo server’s refusal of one setting onto its field and says it under the buttons', async () => {
        put.mockResolvedValue({
            ok: false,
            status: 422,
            refusal: { code: 'invalid', issues: [{ path: 'idleMinutes', code: 'out_of_range' }] }
        });
        render(SettingsSection, { props: { lang: 'ru', group: SECURITY } });
        await openSection('settings-security');

        await fireEvent.input(screen.getByTestId('setting-idleMinutes'), { target: { value: '100' } });
        await sendForm(screen.getByTestId('settings-security-form'));

        await waitFor(() =>
            expect(screen.getByTestId('setting-idleMinutes-field')).toHaveTextContent('Число вне допустимых границ')
        );
        expect(screen.getByTestId('settings-security-error')).toHaveTextContent('Значение не подходит.');
    });
});

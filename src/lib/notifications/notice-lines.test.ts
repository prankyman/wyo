import { describe, expect, it } from 'vitest';
import { noticeLines } from './notice-lines';
import type { TNotice } from './notice-lines';

const notice = (change: Partial<TNotice> & Pick<TNotice, 'code' | 'tone' | 'source'>): TNotice => ({
    id: `${change.source}:${change.code}`,
    ...change
});

describe('noticeLines', () => {
    it.each([
        [
            'a newer version waits',
            notice({ source: 'updates', code: 'update_waiting', tone: 'info', params: { version: '1.3.0' } }),
            'info',
            '/settings',
            'Вышла новая версия wyo: 1.3.0. Установите её в разделе «Настройки» или дождитесь, пока wyo сделает это сама.'
        ],
        [
            'an update failed',
            notice({ source: 'updates', code: 'update_failed', tone: 'stop', params: { version: '1.3.0' } }),
            'urgent',
            '/settings',
            'Обновление до версии 1.3.0 не удалось, и wyo вернула прежнюю версию. Выполните на сервере команду «sudo wyo doctor», чтобы узнать причину.'
        ],
        [
            'the certificate is being obtained',
            notice({
                source: 'certificate',
                code: 'certificate_pending',
                tone: 'wait',
                params: { domain: 'shop.example.ru' }
            }),
            'info',
            '/',
            'wyo получает сертификат для адреса shop.example.ru. Это занимает несколько минут.'
        ],
        [
            'a marketplace key ends in 3 days',
            notice({
                source: 'accounts',
                code: 'account_key_expiring',
                tone: 'warn',
                params: { days: 3, account: 'Магазин одежды' }
            }),
            'attention',
            '/marketplaces',
            'Ключ аккаунта «Магазин одежды» истекает через 3 дня. Создайте новый ключ в кабинете маркетплейса и замените его в разделе «Маркетплейсы» кнопкой «Заменить ключ».'
        ],
        [
            'a marketplace key was not accepted',
            notice({ source: 'accounts', code: 'account_key_refused', tone: 'stop', params: { account: 'Магазин' } }),
            'urgent',
            '/marketplaces',
            'Маркетплейс не принял ключ аккаунта «Магазин», поэтому wyo не читает его заказы. Проверьте ключ в кабинете маркетплейса и замените его в разделе «Маркетплейсы» кнопкой «Заменить ключ».'
        ],
        [
            'the last copy is old',
            notice({ source: 'backups', code: 'backup_old', tone: 'warn', params: { days: 3 } }),
            'attention',
            '/settings',
            'Последняя копия базы данных сделана 3 дня назад. Сделайте новую копию в разделе «Настройки».'
        ]
    ] as const)(
        'words %s from its code and parameters, with the colour of its tone and a link to where it is dealt with',
        (_case, source, urgency, href, text) => {
            const [line] = noticeLines([source], 'ru');

            expect(line).toEqual({ key: source.id, kind: source.code, urgency, href, text });
        }
    );

    it('words the reason a certificate failed from the reason’s own text', () => {
        const [line] = noticeLines(
            [
                notice({
                    source: 'certificate',
                    code: 'certificate_failed',
                    tone: 'stop',
                    params: { domain: 'shop.example.ru', reason: 'dns' }
                })
            ],
            'ru'
        );

        expect(line.urgency).toBe('urgent');
        expect(line.text).toBe(
            'Не удалось получить сертификат для адреса shop.example.ru. Скорее всего, адрес shop.example.ru указывает не на этот сервер. Проверьте у регистратора домена запись типа A: она должна содержать адрес этого сервера.'
        );
    });

    it('says a general sentence for a code this interface has no text for, and keeps the order the wyo server gave', () => {
        const lines = noticeLines(
            [
                notice({ source: 'backups', code: 'backup_none', tone: 'warn' }),
                notice({ source: 'updates', code: 'from_a_later_release', tone: 'ok' })
            ],
            'ru'
        );

        expect(lines.map((line) => line.kind)).toEqual(['backup_none', 'from_a_later_release']);
        expect(lines[1]).toMatchObject({
            urgency: 'info',
            text: 'Программа сообщает о событии, для которого в этом окне нет описания. Обновите страницу или откройте раздел «Настройки».'
        });
    });
});

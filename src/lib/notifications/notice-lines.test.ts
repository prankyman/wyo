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
            'a marketplace’s notifications stopped arriving',
            notice({
                source: 'accounts',
                code: 'account_notifications_lost',
                tone: 'warn',
                params: { account: 'Кабинет' }
            }),
            'attention',
            '/marketplaces',
            'Уведомления маркетплейса по аккаунту «Кабинет» перестали приходить, поэтому заказы доходят до wyo позже, чем могли бы. Пока следующее уведомление не придёт, wyo читает заказы сама каждые 5 минут, а если раздел «Внешний API» не заполнен, каждые 30 минут. Что проверить в кабинете маркетплейса, написано на карточке аккаунта в разделе «Маркетплейсы».'
        ],
        [
            'the fulfilment centre does not answer',
            notice({ source: 'external_api', code: 'fc_no_answer', tone: 'warn' }),
            'attention',
            '/external-api',
            'Система фулфилмент-центра не отвечает, поэтому wyo не может передавать заказы и узнавать, что с ними. wyo повторит попытку сама; состояние и причина — в разделе «Внешний API».'
        ],
        [
            'the fulfilment centre refuses a key that may only read',
            notice({
                source: 'external_api',
                code: 'fc_key_refused',
                tone: 'stop',
                params: { state: 'read_only_key' }
            }),
            'urgent',
            '/external-api',
            'Ключ фулфилмент-центра только читает, а для передачи заказов нужна ещё и запись. Поэтому wyo не передаёт ему заказы. Создайте в его системе ключ с правом записи и введите его в разделе «Внешний API».'
        ],
        [
            'orders have a problem at a step',
            notice({ source: 'external_api', code: 'fc_order_problems', tone: 'stop', params: { count: 2 } }),
            'urgent',
            '/orders?attention=true',
            'У 2 заказов шаг обмена не удался: маркетплейс или фулфилмент-центр отказал. Слова отказа и что делать — в подробностях заказа в разделе «Заказы», по фильтру «Требуют внимания».'
        ],
        [
            'the fulfilment centre does not accept the key',
            notice({ source: 'external_api', code: 'fc_key_refused', tone: 'stop' }),
            'urgent',
            '/external-api',
            'Фулфилмент-центр не принимает ключ, поэтому wyo не передаёт ему заказы. Создайте в его системе новый ключ и введите его в разделе «Внешний API».'
        ],
        [
            'two orders wait at the fulfilment centre',
            notice({ source: 'external_api', code: 'fc_orders_held', tone: 'warn', params: { count: 2 } }),
            'attention',
            '/orders?attention=true',
            '2 заказа ждут у фулфилмент-центра: он не может принять их в работу. Причины — в подробностях заказа в разделе «Заказы», по фильтру «Требуют внимания».'
        ],
        [
            'five orders have no reserve in time',
            notice({ source: 'external_api', code: 'fc_not_reserved', tone: 'warn', params: { count: 5 } }),
            'attention',
            '/orders?attention=true',
            'У 5 заказов нет резерва товара у фулфилмент-центра, хотя прошла четверть срока передачи. Решите, ждать, привезти товар или отменить заказы в кабинете маркетплейса: сама wyo заказы не отменяет.'
        ],
        [
            'one marking code was refused',
            notice({ source: 'external_api', code: 'fc_codes_refused', tone: 'stop', params: { count: 1 } }),
            'urgent',
            '/orders?attention=true',
            'Маркетплейс не принял код маркировки по 1 заказу. Исправьте код у фулфилмент-центра и упакуйте единицу товара заново; новые коды wyo отправит сама.'
        ],
        [
            'a hand-over waits',
            notice({ source: 'external_api', code: 'fc_handover_waits', tone: 'warn', params: { count: 1 } }),
            'attention',
            '/handovers',
            '1 передача ждёт вашего решения или остановилась из-за проблемы. Откройте раздел «Передачи» и прочитайте, что нужно сделать.'
        ],
        [
            'orders were cancelled by the warehouse',
            notice({ source: 'external_api', code: 'fc_cancelled_by_warehouse', tone: 'warn', params: { count: 3 } }),
            'attention',
            '/orders?attention=true',
            'Склад фулфилмент-центра отменил 3 заказа, а на маркетплейсе они остаются. Решите, что делать с каждым: собрать иначе или отменить в кабинете маркетплейса. wyo сама заказы не отменяет.'
        ],
        [
            'changes wait for a person at the warehouse',
            notice({ source: 'external_api', code: 'fc_change_waits', tone: 'warn', params: { count: 4 } }),
            'attention',
            '/orders?attention=true',
            'Фулфилмент-центр ждёт решения человека на своём складе по изменению 4 заказов. Свяжитесь с фулфилмент-центром: wyo сама ничего в заказах не меняет.'
        ],
        [
            'an account key may only read',
            notice({
                source: 'external_api',
                code: 'fc_account_read_only',
                tone: 'warn',
                params: { account: 'Магазин' }
            }),
            'attention',
            '/marketplaces',
            'Ключ аккаунта «Магазин» только читает, поэтому wyo не может передавать его заказы фулфилмент-центру. Создайте в кабинете маркетплейса ключ с правом записи и замените им ключ в разделе «Маркетплейсы».'
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

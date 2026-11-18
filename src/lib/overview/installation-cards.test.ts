import { describe, expect, it } from 'vitest';
import { installationCards } from './installation-cards';
import type { TInstallation } from './installation-cards';

const DAY = '2026-11-17T12:00:00';

/** An installation as the wyo server answers it, with everything in order; each case changes only what it is about. */
function installation(change: {
    version?: string;
    updates?: Partial<TInstallation['updates']>;
    certificate?: Partial<TInstallation['certificate']>;
    backups?: Partial<TInstallation['backups']>;
}): TInstallation {
    return {
        version: change.version ?? '1.2.3',
        updates: { auto: true, requested: false, ...change.updates },
        certificate: {
            mode: 'automatic',
            state: 'ok',
            domain: 'shop.example.ru',
            notAfter: DAY,
            ...change.certificate
        },
        backups: { state: 'ok', last: { name: 'wyo-2026-11-17.db', size: 1536, createdAt: DAY }, ...change.backups }
    };
}

const cardOf = (source: TInstallation, key: string) =>
    installationCards(source, 'ru').find((card) => card.key === key)!;

describe('installationCards', () => {
    it('draws three cards in a fixed order, each with a title, a help, an action to the settings and a tone', () => {
        const cards = installationCards(installation({}), 'ru');

        expect(cards.map((card) => card.key)).toEqual(['version', 'certificate', 'backups']);
        expect(cards.map((card) => card.title)).toEqual(['Версия программы', 'Сертификат https', 'Копия базы данных']);
        for (const card of cards) {
            expect(card.help.length).toBeGreaterThan(40);
            expect(card.action).toEqual({ label: 'Открыть «Настройки»', href: '/settings' });
        }
    });

    it.each([
        [
            'nothing waits and a look was made',
            { lastCheckAt: DAY },
            'ok',
            'Актуальная',
            'Установлена версия 1.2.3. Последняя проверка новых версий была 17 нояб. 2026 г.; новее ничего не нашлось.'
        ],
        [
            'nothing waits and no look was made',
            {},
            'ok',
            'Актуальная',
            'Установлена версия 1.2.3. wyo ещё ни разу не проверяла, вышла ли новая версия. Она проверит сама в течение суток, или нажмите «Проверить сейчас» в разделе «Настройки».'
        ],
        [
            'a newer version waits and updates are automatic',
            { waitingVersion: '1.3.0' },
            'info',
            'Есть новая версия',
            'Установлена версия 1.2.3, а вышла версия 1.3.0. wyo установит её сама в ближайшие сутки.'
        ],
        [
            'a newer version waits and updates are manual',
            { auto: false, waitingVersion: '1.3.0' },
            'info',
            'Есть новая версия',
            'Установлена версия 1.2.3, а вышла версия 1.3.0. Автоматическое обновление выключено, поэтому нажмите «Обновить сейчас» в разделе «Настройки».'
        ],
        [
            'an update failed',
            { failedVersion: '1.3.0' },
            'stop',
            'Обновление не удалось',
            'Обновление до версии 1.3.0 не удалось: новая версия не запустилась, и wyo вернула прежнюю, 1.2.3. Чтобы понять причину, выполните на сервере команду «sudo wyo doctor».'
        ],
        [
            'the owner asked and the updater has not answered yet',
            { requested: true },
            'wait',
            'Запрос принят',
            'Вы попросили проверить или установить обновление. Это делает отдельная программа, обновлятор; она выполнит запрос в течение минуты, и карточка обновится.'
        ]
    ] as const)('version card: %s', (_case, updates, tone, label, sentence) => {
        const card = cardOf(installation({ updates }), 'version');

        expect([card.state.tone, card.state.label, card.sentence]).toEqual([tone, label, sentence]);
    });

    it.each([
        [
            'obtained',
            { state: 'ok' },
            'ok',
            'Действует',
            'wyo продлевает сертификат для адреса shop.example.ru сама, заранее. Сейчас он действует до 17 нояб. 2026 г.'
        ],
        [
            'being obtained',
            { state: 'pending' },
            'wait',
            'Получаем',
            'wyo получает сертификат для адреса shop.example.ru. Обычно это занимает несколько минут. Пока сертификата нет, браузер может предупреждать, что соединение небезопасно.'
        ],
        [
            'held by a web server in front',
            { mode: 'proxy', state: 'not_applicable' },
            'off',
            'Другая программа',
            'Сертификатом занимается веб-сервер, который стоит перед wyo. wyo сертификат не получает и не продлевает.'
        ],
        [
            'not used',
            { mode: 'none', state: 'not_applicable' },
            'off',
            'Без шифрования',
            'wyo работает без шифрования. Так бывает только при разработке и проверках, не на настоящем сервере.'
        ]
    ] as const)('certificate card: %s', (_case, certificate, tone, label, sentence) => {
        const card = cardOf(installation({ certificate }), 'certificate');

        expect([card.state.tone, card.state.label, card.sentence]).toEqual([tone, label, sentence]);
    });

    it.each([
        ['dns', 'указывает не на этот сервер'],
        ['ports', 'порты 80 и 443 закрыты или заняты'],
        ['authority', 'Центр сертификации, который выдаёт сертификаты, отказал'],
        ['other', 'Причина не определена'],
        [undefined, 'Причина не определена']
    ])('certificate card: a failure with the reason %s says what to check', (lastError, words) => {
        const card = cardOf(installation({ certificate: { state: 'failed', lastError } }), 'certificate');

        expect(card.state).toEqual({ tone: 'stop', label: 'Не получен' });
        expect(card.sentence).toContain('Не удалось получить сертификат для адреса shop.example.ru.');
        expect(card.sentence).toContain(words);
    });

    it.each([
        [
            'fresh',
            { state: 'ok' },
            'ok',
            'Копия свежая',
            'Последняя копия сделана 17 нояб. 2026 г., её размер — 1,5 КБ. wyo делает копию раз в сутки и хранит семь последних.'
        ],
        [
            'old',
            { state: 'old' },
            'warn',
            'Копия устарела',
            'Последняя копия сделана 17 нояб. 2026 г., то есть больше двух суток назад, хотя wyo делает её раз в сутки. Сделайте копию вручную в разделе «Настройки» и проверьте, что на диске сервера есть свободное место.'
        ],
        [
            'failed',
            { state: 'failed', last: undefined },
            'stop',
            'Копия не сделана',
            'Последняя попытка сделать копию не удалась. Чаще всего на диске сервера не хватает места. Освободите место и сделайте копию вручную в разделе «Настройки».'
        ],
        [
            'missing',
            { state: 'none', last: undefined },
            'warn',
            'Копий ещё нет',
            'wyo ещё не сделала ни одной копии: первая появится ночью. Можно сделать её прямо сейчас в разделе «Настройки».'
        ]
    ] as const)('backups card: %s', (_case, backups, tone, label, sentence) => {
        const card = cardOf(installation({ backups }), 'backups');

        expect([card.state.tone, card.state.label, card.sentence]).toEqual([tone, label, sentence]);
    });
});

import { describe, expect, it } from 'vitest';
import { certificateReasonText } from './certificate-reason';

describe('certificateReasonText', () => {
    it.each([
        ['dns', 'Скорее всего, адрес shop.example.ru указывает не на этот сервер.'],
        ['ports', 'Скорее всего, порты 80 и 443 закрыты или заняты другой программой.'],
        ['authority', 'Центр сертификации, который выдаёт сертификаты, отказал или не ответил.'],
        ['other', 'Причина не определена.'],
        ['a word of a later release', 'Причина не определена.'],
        [undefined, 'Причина не определена.']
    ])('words the reason «%s» for a person, and reads a reason it does not know as «other»', (reason, start) => {
        expect(certificateReasonText(reason, 'shop.example.ru', 'ru')).toContain(start);
    });
});

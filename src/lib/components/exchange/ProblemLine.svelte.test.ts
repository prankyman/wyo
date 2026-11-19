import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { exchangeProblemOf } from '$test/support/external-api';
import ProblemLine from './ProblemLine.svelte';
import { problemHelp } from './problem-text';
import type { TProblem } from './types';

function draw(problem: TProblem): void {
    render(ProblemLine, { props: { lang: 'ru', problem, testId: 'problem' } });
}

/** A problem in wyo's sentence and, beside it, the marketplace's or the fulfilment centre's own words and code as received. */
describe('ProblemLine', () => {
    it('does not say the code again when the marketplace’s words are the code itself', () => {
        draw(
            exchangeProblemOf({
                code: 'codes_refused',
                params: {},
                marketplaceCode: 'MARK_NOT_FOUND',
                words: 'MARK_NOT_FOUND'
            })
        );

        expect(screen.getByTestId('problem-words')).toHaveTextContent('Слова маркетплейса: «MARK_NOT_FOUND»');
        expect(screen.getByTestId('problem-words')).not.toHaveTextContent('код');
    });

    it('names the postings a marketplace took out of the documents of a shipment', () => {
        draw(
            exchangeProblemOf({
                code: 'marketplace_refused',
                params: { step: 'documents', postings: ['100-2', '100-3'] },
                marketplaceCode: 'postings_removed_from_act'
            })
        );

        expect(screen.getByTestId('problem')).toHaveTextContent(
            'Маркетплейс отказал на шаге «получение документов партии»: затронуты отправления 100-2, 100-3.'
        );
    });

    it('says what a shipment lacks and what it holds besides, and a list with nothing in it as a dash', () => {
        draw(
            exchangeProblemOf({
                code: 'marketplace_refused',
                params: { step: 'move', missing: [], extra: ['200-1'] },
                marketplaceCode: 'batch_differs'
            })
        );

        expect(screen.getByTestId('problem')).toHaveTextContent('Не хватает: —. Лишнее: 200-1.');
    });

    it('names the postings of a shipment the wyo server sends as one text, as it sends them', () => {
        draw(
            exchangeProblemOf({
                code: 'marketplace_refused',
                params: { step: 'close', missing: '', extra: '200-1, 200-2' },
                marketplaceCode: 'batch_differs'
            })
        );

        expect(screen.getByTestId('problem')).toHaveTextContent('Не хватает: —. Лишнее: 200-1, 200-2.');
    });

    it('words a problem of parts the marketplace made and wyo lost the numbers of', () => {
        draw(exchangeProblemOf({ code: 'pack_parts_unknown', params: {}, marketplaceCode: undefined }));

        expect(screen.getByTestId('problem')).toHaveTextContent('номера частей заказа неизвестны');
    });

    it('words a refusal of the marketplace with the step named, and shows its words and its code beside the sentence', () => {
        draw(exchangeProblemOf());

        expect(screen.getByTestId('problem')).toHaveTextContent(
            'Маркетплейс отказал на шаге «взятие заказа в сборку».'
        );
        expect(screen.getByTestId('problem-words')).toHaveTextContent(
            'Слова маркетплейса: «Не удалось добавить заказ в поставку» (код FailedToAddSupplyOrder)'
        );
        expect(screen.getByTestId('problem-help-button')).toBeInTheDocument();
    });

    it('attributes the words of a refusal of the fulfilment centre to the fulfilment centre', () => {
        draw(
            exchangeProblemOf({
                code: 'fc_refused',
                params: { step: 'labels' },
                marketplaceCode: undefined,
                words: 'Заказ закрыт'
            })
        );

        expect(screen.getByTestId('problem')).toHaveTextContent(
            'Фулфилмент-центр отказал на шаге «получение этикетки».'
        );
        expect(screen.getByTestId('problem-words')).toHaveTextContent('Слова фулфилмент-центра: «Заказ закрыт»');
        expect(screen.getByTestId('problem-words')).not.toHaveTextContent('код');
    });

    it.each([
        [
            { code: 'marking_code_missing' },
            'Маркетплейс требует код маркировки, а фулфилмент-центр не прислал ни одного.'
        ],
        [{ code: 'codes_refused' }, 'Маркетплейс не принял код маркировки.'],
        [
            { code: 'identifier_unsupported', params: { identifier: 'imei' } },
            'требует не только код маркировки, но и «Номер IMEI'
        ],
        [
            { code: 'identifier_unsupported', params: { identifier: 'weight' } },
            'требует не только код маркировки, но и «Вес единицы товара»'
        ],
        [
            { code: 'identifier_unsupported', params: { identifier: 'multibox' } },
            'Товар отправляется в нескольких коробках, и маркетплейс требует указать их количество.'
        ],
        [
            { code: 'pack_contents_differ' },
            'Состав посылок у фулфилмент-центра не совпадает с составом заказа на маркетплейсе.'
        ],
        [
            { code: 'pack_reopened_after_report' },
            'открыл заказ заново после того, как упаковка уже была передана маркетплейсу'
        ],
        [
            { code: 'pack_part_cancelled', params: { part: '100-3' } },
            'Маркетплейс отменил часть 100-3 заказа, который разделился при упаковке на несколько посылок.'
        ],
        [{ code: 'labels_not_made' }, 'Маркетплейс не смог сделать этикетку посылки.'],
        [{ code: 'codes_not_checked' }, 'Маркетплейс не успел проверить коды маркировки.'],
        [{ code: 'batch_waits' }, 'Маркетплейс пока не может создать партию заказов на этот день.'],
        [{ code: 'documents_wait' }, 'Маркетплейс ещё не сделал документы передачи.'],
        [
            { code: 'label_changed', params: { orders: ['4455667788', '4455667799'] } },
            'изменилась у заказов: 4455667788, 4455667799'
        ],
        [{ code: 'shipping_point_needed' }, 'Не выбран пункт сдачи'],
        [{ code: 'shipping_point_gone' }, 'больше не предлагает'],
        [{ code: 'no_orders' }, 'не осталось заказов'],
        [{ code: 'settings_missing' }, 'Не заполнены настройки передачи'],
        [{ code: 'something_new' }, 'кодом «something_new», которого wyo не знает']
    ] as const)('words the problem %j', (given, text) => {
        draw({ ...given, at: '2026-11-18T12:00:00' } as TProblem);

        expect(screen.getByTestId('problem')).toHaveTextContent(text);
        expect(screen.queryByTestId('problem-words')).toBeNull();
    });

    it.each([
        [{ code: 'identifier_unsupported', params: { identifier: 'multibox' } }, 'количество коробок для этого заказа'],
        [{ code: 'pack_contents_differ' }, 'Уберите отсутствующие товары из заказа в кабинете маркетплейса сами'],
        [{ code: 'pack_reopened_after_report' }, 'Изменить их у маркетплейса после этого из wyo нельзя'],
        [
            { code: 'pack_part_cancelled', params: { part: '100-3' } },
            'скажите фулфилмент-центру, что делать с посылкой части 100-3'
        ],
        [{ code: 'labels_not_made' }, 'wyo запросит этикетки у маркетплейса ещё раз'],
        [{ code: 'codes_not_checked' }, 'wyo отправит весь набор кодов ещё раз'],
        [{ code: 'batch_waits' }, 'wyo сама повторяет попытку каждые 5 минут'],
        [{ code: 'documents_wait' }, 'wyo сама спрашивает его снова']
    ] as const)('explains the problem %j behind its «?»', (given, text) => {
        expect(problemHelp({ ...given, at: '2026-11-18T12:00:00' } as TProblem, 'ru')).toContain(text);
    });
});

/**
 * The marketplaces' names, in Latin and Cyrillic letters, as whole words — the one list two policy tests read: the neutral
 * fulfilment-centre API names no marketplace (`reference-policy.test.ts`, rule R2), and the wyo interface names one only in the
 * texts of the family `marketplace.<code>.*` (`marketplace-name-policy.test.ts`, rule R6).
 */
export const MARKETPLACE_NAMES =
    /(?<![\p{L}\p{N}])(wildberries|вайлдберриз|ozon|озон|yandex|яндекс|megamarket|мегамаркет|sbermegamarket|avito|авито|lemana|лемана|леруа|aliexpress|kazanexpress|\bwb\b)(?![\p{L}\p{N}])/giu;

/** Every marketplace name in `text`, one line each: `line <n>: <name>`. */
export function marketplaceNamesIn(text: string): string[] {
    return text
        .split('\n')
        .flatMap((line, index) =>
            [...line.matchAll(MARKETPLACE_NAMES)].map((match) => `line ${index + 1}: ${match[0]}`)
        );
}

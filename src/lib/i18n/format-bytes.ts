import { t } from './i18n';
import type { TLocale } from './types';

/**
 * A file's size as a person reads it: «1,5 КБ», «5 МБ». The unit is the language's own word (`units.*`), the number the language's
 * own writing of a decimal (comma or point), with one place after the separator at most.
 */
export function formatBytes(bytes: number, lang: TLocale): string {
    const units = ['units.bytes', 'units.kilobytes', 'units.megabytes', 'units.gigabytes'];
    let value = Math.max(0, bytes);
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
        value /= 1024;
        unit += 1;
    }
    const number = new Intl.NumberFormat(lang, { maximumFractionDigits: unit === 0 ? 0 : 1 }).format(value);
    return `${number} ${t(units[unit], lang)}`;
}

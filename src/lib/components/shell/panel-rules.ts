import type { INotificationLine, TUrgency } from '$lib/notifications/types';

/**
 * The rules of the bell and its panel, pure so the components stay drawing only.
 */

/** The colours of a line by its urgency — never the only sign: an urgent line also starts with a warning icon (WCAG 1.4.1). */
export const URGENCY_STYLE: Record<TUrgency, { border: string; background: string }> = {
    urgent: { border: 'border-stop-600', background: 'bg-stop-50' },
    attention: { border: 'border-warn-500', background: 'bg-warn-50' },
    info: { border: 'border-wait-500', background: 'bg-wait-50' }
};

/** How the bell looks: grey with nothing, yellow for attention or information only, red with any urgent line. */
export type TBellTone = 'quiet' | 'attention' | 'urgent';

export function bellTone(lines: readonly INotificationLine[]): TBellTone {
    if (lines.some((line) => line.urgency === 'urgent')) return 'urgent';
    return lines.length > 0 ? 'attention' : 'quiet';
}

/** The number on the bell: the lines, «99+» above 99, nothing when there is nothing. */
export function bellNumber(count: number): string | null {
    if (count <= 0) return null;
    return count > 99 ? '99+' : String(count);
}

/** What the pulse compares: the keys of the urgent lines. */
export function attentionKeys(lines: readonly INotificationLine[]): string[] {
    return lines.filter((line) => line.urgency === 'urgent').map((line) => `line:${line.key}`);
}

/**
 * Whether the bell pulses: while the panel holds an urgent line that was not there when the panel was last opened. Opening the
 * panel stops it; a new urgent line starts it again.
 */
export function shouldPulse(current: readonly string[], seen: ReadonlySet<string>): boolean {
    return current.some((key) => !seen.has(key));
}

/**
 * The text of the first line that was not there before — what the polite live region says. Null on the first answer (`previous`
 * null) and when nothing is new.
 */
export function announcement(previous: ReadonlySet<string> | null, lines: readonly INotificationLine[]): string | null {
    if (previous === null) return null;
    const fresh = lines.find((item) => !previous.has(`line:${item.key}`));
    return fresh ? fresh.text : null;
}

/** Every key of the panel, for `announcement` to compare the next answer with. */
export function panelKeys(lines: readonly INotificationLine[]): Set<string> {
    return new Set(lines.map((line) => `line:${line.key}`));
}

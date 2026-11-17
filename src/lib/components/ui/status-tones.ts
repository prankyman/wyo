/**
 * The colours of a status badge, decided once, in this one file (the theme names the colours: `src/app.css`). Without it every
 * screen holds its own `Record<status, classes>` and the same state is drawn in up to four shades. A badge names the TONE of its status — what the person should make of it — and the
 * look of each tone is here:
 *
 * - `ok` — done, accepted, connected (green, the theme's `ok`);
 * - `wait` — in progress, waiting on someone else (sky, the theme's `wait`);
 * - `warn` — needs a look soon: the person's turn, a key about to expire (amber, the theme's `warn`);
 * - `stop` — refused, broken, past its term (red, the theme's `stop`);
 * - `info` — information, a step of its own that is neither (the brand colour, the theme's `brand`);
 * - `off` — closed, cancelled, switched off: nothing more happens (grey, the theme's `ink`).
 */
export type TTone = 'ok' | 'wait' | 'warn' | 'stop' | 'info' | 'off';

export const TONE_CLASS: Readonly<Record<TTone, string>> = {
    ok: 'bg-ok-50 text-ok-800',
    wait: 'bg-wait-50 text-wait-800',
    warn: 'bg-warn-50 text-warn-800',
    stop: 'bg-stop-50 text-stop-800',
    info: 'bg-brand-50 text-brand-800',
    off: 'bg-ink-100 text-ink-600'
};

/** The same tones as plain coloured words, for a status written in a sentence or a table cell without a badge. */
export const TONE_TEXT_CLASS: Readonly<Record<TTone, string>> = {
    ok: 'text-ok-800',
    wait: 'text-wait-800',
    warn: 'text-warn-800',
    stop: 'text-stop-800',
    info: 'text-brand-800',
    off: 'text-ink-600'
};

export function toneClass(tone: TTone): string {
    return TONE_CLASS[tone];
}

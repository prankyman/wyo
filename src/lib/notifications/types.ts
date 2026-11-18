/** How much a line asks: urgent (red), needs attention (yellow), for information (blue). */
export type TUrgency = 'urgent' | 'attention' | 'info';

/**
 * One line of the bell's panel: one thing that needs the owner's attention, with the page where it is dealt with. A line has no
 * «×»: it goes by itself when the thing is dealt with.
 */
export interface INotificationLine {
    /** Stable for the same thing: the pulse rule tells a new line from an old one by it. */
    key: string;
    /** The kind of line, e.g. `update_waiting`; also the `data-kind` of its row. */
    kind: string;
    urgency: TUrgency;
    /** The text, in the page's language. */
    text: string;
    /** Where the line leads, before the language prefix; null when the line only explains. */
    href: string | null;
}

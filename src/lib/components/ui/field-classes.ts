/**
 * The look of the interface's typed boxes, decided once (constitution, "Forms"). `TextBox`, `SelectBox`
 * and `Textarea` draw with these; a place that must keep a raw tag (listed with its reason in `form-control-policy.test.ts`)
 * imports them instead of declaring a look of its own. A caller changes only what differs — its width (`w-auto`, `sm:w-64`),
 * the bigger letters of a large-print screen (`text-base`) — through the component's `class`, which is merged over this look.
 */

/** A one-line text box: 44 px tall at least (a finger's tap area), the full width of its place. */
export const INPUT_CLASS =
    'min-h-11 w-full min-w-0 rounded border border-ink-300 bg-white px-3 text-sm text-ink-900 focus:border-brand-600 focus:outline-none disabled:opacity-50';

/** A drop-down: the box's look; the room of its arrow is kept by the `select` rule of `src/app.css`. */
export const SELECT_CLASS = INPUT_CLASS;

/** A text area: the box's look with room above and below its lines. */
export const TEXTAREA_CLASS =
    'w-full min-w-0 rounded border border-ink-300 bg-white px-3 py-2 text-sm text-ink-900 focus:border-brand-600 focus:outline-none disabled:opacity-50';

/** The smaller box of a dense row (a table's filter): less padding and smaller letters, the same height. */
export const SMALL_CLASS = 'px-2 text-xs';

/** A box holding a refused value: a red border (and `aria-invalid` on the element). */
export const INVALID_CLASS = 'border-stop-600 focus:border-stop-700';

/** A refusal written under a box that is not inside `Field` (which draws its own). */
export const ERROR_CLASS = 'text-xs text-stop-700';

/** The caption of a figure or a block: small grey capitals (a stat's name, a card's corner word). */
export const CAPTION_CLASS = 'text-xs tracking-widest text-ink-500 uppercase';

/** A box of text to copy: a token, an install command. Breaks anywhere, so it never widens the phone's page. */
export const COPY_BOX_CLASS =
    'block w-full rounded border border-ink-300 bg-ink-50 px-3 py-2 font-mono text-xs break-all text-ink-900 select-all';

/** The column's name written before a table cell's value on a phone, where the table's head is hidden. */
export const CELL_LABEL_CLASS = 'mr-1 text-xs text-ink-500 sm:hidden';

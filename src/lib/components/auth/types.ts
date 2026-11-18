/** One password box of `NewPasswordFields`: its `id` and the `data-test` of the box and of its refusal line. */
export interface IPasswordBoxIds {
    id: string;
    test: string;
    errorTest: string;
}

/** The three boxes' names inside the form; the first is asked only when the current password is. */
export type TPasswordBoxName = 'currentPassword' | 'newPassword' | 'confirmNewPassword';

/**
 * How the refusal under a box is drawn: `line` keeps a line free under the box so a message coming or going never
 * moves the button; `text` draws the message only when there is one; `field` hands it to `Field` (its own error line).
 */
export type TPasswordErrorLook = 'line' | 'text' | 'field';

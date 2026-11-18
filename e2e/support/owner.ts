/**
 * The owner every browser test's wyo server is started with: the one person who signs in. The login and the passwords are made up
 * for the tests and protect nothing: the data folders they belong to are made empty for every run (`e2e/support/wyo-server.ts`).
 * The passwords are never printed, never put in a failure message and never written to a file that outlives the run.
 */
export interface IOwner {
    login: string;
    password: string;
}

/** The owner of every wyo server the tests start. */
export const TEST_OWNER: IOwner = { login: 'owner', password: 'a-long-test-password-1' };

/** A password the owner changes to (at least 10 characters), different from `TEST_OWNER.password`. */
export const NEW_TEST_PASSWORD = 'another-long-test-password-2';

/** A password no owner has. */
export const WRONG_TEST_PASSWORD = 'not-the-password-at-all';

import { renameSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import type { IWyoServer } from '../support/wyo-server.js';

/** What the updater writes into `update-state.json` (`internal/updatestate`); every field is optional. */
export interface IUpdateState {
    lastCheckAt?: string;
    latestVersion?: string;
    waitingVersion?: string;
    failedVersion?: string;
    failedAt?: string;
    lastError?: string;
}

/**
 * Writes the file only the updater writes, `update-state.json`, into the data folder of `server`, as the updater does: a new
 * file renamed over the old one, so the wyo server never reads half of it. The wyo server reads it at every call, so the next
 * read of the notices or of the installation shows it.
 */
export function writeUpdateState(server: IWyoServer, state: IUpdateState): void {
    const file = path.join(server.dataDir, 'update-state.json');
    const temporary = `${file}.tmp`;
    writeFileSync(temporary, JSON.stringify(state));
    renameSync(temporary, file);
}

/** Removes the updater's file: the wyo server then knows of no look and no waiting version. */
export function clearUpdateState(server: IWyoServer): void {
    rmSync(path.join(server.dataDir, 'update-state.json'), { force: true });
}

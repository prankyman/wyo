import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * The seller's instructions name nothing that does not exist (constitution, «Help texts», «The seller's instructions»). A seller
 * follows `README.md` word by word on a server: a command the wyo server does not have, an installer flag the installer does not
 * read, a button that is not on the screen or an install address spelled differently from the installer's own leaves the
 * seller stuck. This test reads the README, the installer, the commands' table of the wyo server and the interface's texts.
 */

const README = readFileSync(path.resolve('README.md'), 'utf8');
const INSTALLER = readFileSync(path.resolve('scripts/install.sh'), 'utf8');
const COMMANDS_TABLE = readFileSync(path.resolve('internal/cli/commands.go'), 'utf8');
const MESSAGES: Record<string, string> = JSON.parse(readFileSync(path.resolve('messages/ru.json'), 'utf8'));

/** The commands of the wyo server: every `Name: "…"` of the table in `internal/cli/commands.go` (e.g. `owner reset-password`). */
function commandsOfTable(table: string): string[] {
    return [...table.matchAll(/Name:\s*"([^"]+)"/g)].map((match) => match[1]);
}

/**
 * The commands a text names: `wyo <command>` in a command line or in `code`, where `<command>` is one word, or two words when
 * the first is a group such as `owner`. Flags and file names after it are not part of the command.
 */
function commandsNamedIn(text: string, groups: string[]): string[] {
    const named = new Set<string>();
    for (const match of text.matchAll(/(?:^|[\s`])wyo ([a-z][a-z-]*)(?: ([a-z][a-z-]*))?/gm)) {
        const [, first, second] = match;
        named.add(groups.includes(first) && second ? `${first} ${second}` : first);
    }
    return [...named].sort();
}

/** The flags the installer reads: the `--name)` branches of its argument loop, alone or in a `a | b)` list. */
function flagsOfInstaller(script: string): string[] {
    const flags = new Set<string>();
    for (const match of script.matchAll(/^\s*((?:--[a-z-]+(?:\s*\|\s*)?)+(?:\s*\|\s*-h)?)\)/gm)) {
        for (const flag of match[1].matchAll(/--[a-z-]+/g)) flags.add(flag[0]);
    }
    return [...flags].sort();
}

/** The installer flags a text names: every `--flag` on a line that runs `install.sh`, and every flag written alone in `code`. */
function installerFlagsNamedIn(text: string): string[] {
    const flags = new Set<string>();
    for (const line of text.split('\n')) {
        if (line.includes('install.sh')) for (const flag of line.matchAll(/(?<![\w-])--[a-z-]+/g)) flags.add(flag[0]);
        for (const flag of line.matchAll(/`(--[a-z-]+)[^`]*`/g)) flags.add(flag[1]);
    }
    return [...flags].sort();
}

/** The names in «…» that a text calls a page, a section, a button, a switch or a setting. */
function screenNamesIn(text: string): string[] {
    const names = new Set<string>();
    const pattern = /(?:страниц|кнопк|раздел|переключател|настройк|пункт)\p{L}*\s+«([^»]+)»(?:\s+или\s+«([^»]+)»)?/gu;
    for (const match of text.matchAll(pattern)) {
        names.add(match[1]);
        if (match[2]) names.add(match[2]);
    }
    return [...names].sort();
}

/**
 * Names in «…» that are not on any screen of this release, each with its reason. The list starts empty and holds exceptions
 * only (constitution, principle IV).
 */
const NOT_ON_A_SCREEN_YET: Record<string, string> = {
    'External API': 'the setting is built by Plan 4; the README describes what the program will do and says so'
};

const COMMAND_GROUPS = ['owner'];

describe('the README names only what exists', () => {
    it('names only commands the wyo server has', () => {
        const commands = commandsOfTable(COMMANDS_TABLE);

        expect(commands.length).toBeGreaterThanOrEqual(9);
        expect(commandsNamedIn(README, COMMAND_GROUPS).filter((command) => !commands.includes(command))).toEqual([]);
    });

    it('names only flags the installer reads', () => {
        const flags = flagsOfInstaller(INSTALLER);

        expect(flags).toContain('--behind-proxy');
        expect(installerFlagsNamedIn(README).filter((flag) => !flags.includes(flag))).toEqual([]);
    });

    it('names only pages, sections, buttons and settings that the interface has', () => {
        const texts = new Set(Object.values(MESSAGES));
        const missing = screenNamesIn(README).filter((name) => !texts.has(name) && !(name in NOT_ON_A_SCREEN_YET));

        expect(missing).toEqual([]);
    });

    it('spells the install address exactly as the installer downloads from', () => {
        const base = /^RELEASE_URL='([^']+)'$/m.exec(INSTALLER)?.[1];

        expect(base).toBeDefined();
        expect(README).toContain(`curl -fsSL -o install.sh ${base}/install.sh`);
        expect(README.match(/https:\/\/github\.com\/prankyman\/wyo\/releases\/[^\s`]*install\.sh/g)).toEqual([
            `${base}/install.sh`
        ]);
    });

    it('lists only exceptions that still match a name of the README', () => {
        const named = screenNamesIn(README);

        expect(Object.keys(NOT_ON_A_SCREEN_YET).filter((name) => !named.includes(name))).toEqual([]);
    });
});

describe('the readers of this test catch what they must', () => {
    it('reads commands, flags and screen names out of planted texts', () => {
        const text = [
            'Введите `sudo wyo owner reset-password` или `wyo version`.',
            'sudo wyo doctor',
            'sudo bash install.sh --domain a.ru --behind-proxy',
            'Добавьте `--purge`, затем нажмите кнопку «Сохранить» или «Отмена» в разделе «Обновления».'
        ].join('\n');

        expect(commandsNamedIn(text, COMMAND_GROUPS)).toEqual(['doctor', 'owner reset-password', 'version']);
        expect(installerFlagsNamedIn(text)).toEqual(['--behind-proxy', '--domain', '--purge']);
        expect(screenNamesIn(text)).toEqual(['Обновления', 'Отмена', 'Сохранить']);
        expect(commandsOfTable('{Name: "serve"}, {Name: "owner create"}')).toEqual(['serve', 'owner create']);
        expect(
            flagsOfInstaller('        --domain | --login)\n        --purge) x ;;\n        --help | -h) y ;;')
        ).toEqual(['--domain', '--help', '--login', '--purge']);
    });
});

/** The letter the round avatar draws for a login: its first letter, in capitals; a login with no letter is drawn as «?». */
export function loginInitials(login: string): string {
    const first = login.trim().match(/[\p{L}\p{N}]/u);
    return first ? first[0].toLocaleUpperCase() : '?';
}

/**
 * The whole minutes until «Обновить» may be pressed again, rounded up — «Обновить можно через 1 мин.» stays until the moment
 * itself; 0 when the time is not given or has passed.
 */
export function minutesUntil(nextAllowedAt: string | undefined, now: Date): number {
    if (nextAllowedAt === undefined) return 0;
    const left = new Date(nextAllowedAt).getTime() - now.getTime();
    return left <= 0 ? 0 : Math.ceil(left / 60_000);
}

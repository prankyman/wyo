/* eslint-disable @typescript-eslint/no-unused-vars */
export function afterNavigate(callback: () => void): void {
    // Mock implementation - does nothing in tests
}

export function beforeNavigate(callback: () => void): void {
    // Mock implementation - does nothing in tests
}

export function goto(url: string): Promise<void> {
    return Promise.resolve();
}

export function invalidate(url: string): Promise<void> {
    return Promise.resolve();
}

export function invalidateAll(): Promise<void> {
    return Promise.resolve();
}

export function refreshAll(): Promise<void> {
    return Promise.resolve();
}

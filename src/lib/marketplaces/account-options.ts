import type { TAccount } from '$lib/components/marketplaces/types';
import type { ISelectOption } from '$lib/components/ui/types';

/** The accounts as the choices of a list's filter («Аккаунт»): each by its own name, its id as the value. */
export function accountOptions(accounts: readonly TAccount[]): ISelectOption[] {
    return accounts.map((account) => ({ value: String(account.id), label: account.name }));
}

/** The name of the account a filter names by its id; the id itself when no such account is listed any more. */
export function accountNameOf(accounts: readonly TAccount[], id: string): string {
    return accounts.find((account) => String(account.id) === id)?.name ?? id;
}

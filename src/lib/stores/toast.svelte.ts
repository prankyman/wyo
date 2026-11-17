export type ToastVariant = 'default' | 'destructive' | 'success' | 'warning';

export interface Toast {
    id: string;
    title: string;
    description?: string;
    variant?: ToastVariant;
    action?: {
        label: string;
        onClick: () => void;
    };
    timeout?: number; // timeout in seconds
}

class ToastStore {
    toasts: Toast[] = $state([]);

    success(title: string, description?: string, action?: Toast['action'], timeout?: number) {
        this.addToast(title, description, 'success', action, timeout);
    }

    error(title: string, description?: string, action?: Toast['action'], timeout?: number) {
        this.addToast(title, description, 'destructive', action, timeout);
    }

    warning(title: string, description?: string, action?: Toast['action'], timeout?: number) {
        this.addToast(title, description, 'warning', action, timeout);
    }

    info(title: string, description?: string, action?: Toast['action'], timeout?: number) {
        this.addToast(title, description, 'default', action, timeout);
    }

    private addToast(
        title: string,
        description?: string,
        variant: ToastVariant = 'default',
        action?: Toast['action'],
        timeout?: number
    ) {
        const id = `toast-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
        this.toasts.push({ id, title, description, variant, action, timeout: timeout ?? 5 });
    }

    remove(id: string) {
        this.toasts = this.toasts.filter((toast) => toast.id !== id);
    }

    clear() {
        this.toasts = [];
    }
}

export const toast = new ToastStore();

import { writable } from 'svelte/store';

export type ToastType = 'info' | 'success' | 'warning' | 'error';

export interface Toast {
    id: string;
    message: string;
    type: ToastType;
    duration?: number;
}

function createToastStore() {
    const { subscribe, update } = writable<Toast[]>([]);

    function add(message: string, type: ToastType = 'info', duration = 3000) {
        const id = Math.random().toString(36).slice(2);
        update(all => [{ id, message, type, duration }, ...all]);
        
        if (duration > 0) {
            setTimeout(() => remove(id), duration);
        }
    }

    function remove(id: string) {
        update(all => all.filter(t => t.id !== id));
    }

    return {
        subscribe,
        info: (msg: string, dur?: number) => add(msg, 'info', dur),
        success: (msg: string, dur?: number) => add(msg, 'success', dur),
        warning: (msg: string, dur?: number) => add(msg, 'warning', dur),
        error: (msg: string, dur?: number) => add(msg, 'error', dur),
        remove
    };
}

export const toasts = createToastStore();

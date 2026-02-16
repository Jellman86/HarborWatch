<script lang="ts">
    import { toasts } from '../stores/ToastStore';
    import { flip } from 'svelte/animate';
    import { fly } from 'svelte/transition';

    const typeClasses = {
        info: 'bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300',
        success: 'bg-emerald-50 dark:bg-emerald-900/20 border-emerald-200 dark:border-emerald-800 text-emerald-700 dark:text-emerald-400',
        warning: 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800 text-amber-700 dark:text-amber-400',
        error: 'bg-rose-50 dark:bg-rose-900/20 border-rose-200 dark:border-rose-800 text-rose-700 dark:text-rose-400'
    };

    const iconColors = {
        info: 'text-brand-500',
        success: 'text-emerald-500',
        warning: 'text-amber-500',
        error: 'text-rose-500'
    };
</script>

<div class="fixed bottom-6 right-6 z-[9999] flex flex-col gap-3 w-full max-w-sm pointer-events-none">
    {#each $toasts as toast (toast.id)}
        <div 
            animate:flip={{ duration: 300 }}
            transition:fly={{ x: 100, duration: 400 }}
            class="pointer-events-auto p-4 rounded-2xl border shadow-2xl flex items-center gap-4 {typeClasses[toast.type]} backdrop-blur-xl"
        >
            <div class="flex-shrink-0">
                {#if toast.type === 'success'}
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 {iconColors[toast.type]}" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                    </svg>
                {:else if toast.type === 'error'}
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 {iconColors[toast.type]}" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                    </svg>
                {:else}
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 {iconColors[toast.type]}" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" />
                    </svg>
                {/if}
            </div>
            <p class="text-sm font-bold leading-tight">{toast.message}</p>
            <button 
                onclick={() => toasts.remove(toast.id)}
                class="ml-auto p-1 hover:bg-black/5 dark:hover:bg-white/5 rounded-lg transition-colors"
                aria-label="Dismiss Notification"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 opacity-50" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
                </svg>
            </button>
        </div>
    {/each}
</div>

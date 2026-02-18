<script lang="ts">
    import { themeStore, type BrandTheme, type ThemeMode } from '../stores/theme.svelte';

    const modes: { value: ThemeMode, label: string, icon: string }[] = [
        { value: 'light', label: 'Light', icon: 'M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z' },
        { value: 'dark', label: 'Dark', icon: 'M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z' },
        { value: 'system', label: 'System', icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z' }
    ];

    const brands: { value: BrandTheme, label: string, subtitle: string, color: string }[] = [
        { value: 'tech', label: 'Tech Innovation', subtitle: 'Precision grid / signal glow', color: 'bg-cyan-500' },
        { value: 'forest', label: 'Forest Canopy', subtitle: 'Editorial earth / organic grain', color: 'bg-emerald-600' }
    ];
</script>

<div class="space-y-6">
    <div class="space-y-3">
        <span class="block text-[10px] font-black uppercase text-slate-400 ml-1">Appearance Mode</span>
        <div class="flex gap-2 p-1 bg-slate-100 dark:bg-slate-900/50 rounded-2xl border border-slate-200 dark:border-slate-800 w-fit">
            {#each modes as mode}
                <button 
                    onclick={() => themeStore.setMode(mode.value)}
                    class="px-4 py-2 rounded-xl flex items-center gap-2 text-[10px] font-black uppercase tracking-widest transition-all {themeStore.mode === mode.value ? 'bg-white dark:bg-slate-700 text-brand-600 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'}"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={mode.icon} />
                    </svg>
                    {mode.label}
                </button>
            {/each}
        </div>
    </div>

    <div class="space-y-3">
        <span class="block text-[10px] font-black uppercase text-slate-400 ml-1">Color Theme</span>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {#each brands as brand}
                <button 
                    onclick={() => themeStore.setBrand(brand.value)}
                    class="flex items-center gap-3 p-3 rounded-xl border transition-all text-left group relative overflow-hidden {themeStore.brand === brand.value ? 'bg-white dark:bg-slate-800 border-brand-500 ring-1 ring-brand-500 shadow-md' : 'bg-slate-50 dark:bg-slate-900/30 border-transparent hover:bg-white dark:hover:bg-slate-800 hover:border-slate-200 dark:hover:border-slate-700'}"
                >
                    <div class="w-8 h-8 rounded-lg {brand.color} shadow-sm flex items-center justify-center text-white">
                        {#if themeStore.brand === brand.value}
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                                <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                            </svg>
                        {/if}
                    </div>
                    <div class="min-w-0">
                        <p class="text-xs font-bold text-slate-700 dark:text-slate-200 uppercase tracking-tight">{brand.label}</p>
                        <p class="text-[10px] text-slate-500 dark:text-slate-400">{brand.subtitle}</p>
                    </div>
                </button>
            {/each}
        </div>
    </div>
</div>
